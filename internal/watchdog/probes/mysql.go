package probes

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLProbeConfig struct {
	Host          string
	Port          int
	User          string
	Password      string
	Database      string
	CheckInterval time.Duration
	Timeout       time.Duration
	MaxLatencyMs  int64
}

type MySQLProbe struct {
	cfg          MySQLProbeConfig
	mu           sync.Mutex
	db           *sql.DB
	lastSuccess  time.Time
	failures     int
	lastErrorMsg string
}

func NewMySQLProbe(cfg MySQLProbeConfig) *MySQLProbe {
	if cfg.Port <= 0 {
		cfg.Port = 3306
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 10 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2000 * time.Millisecond
	}
	if cfg.MaxLatencyMs <= 0 {
		cfg.MaxLatencyMs = 2000
	}

	return &MySQLProbe{
		cfg: cfg,
	}
}

func (p *MySQLProbe) Name() string {
	return "mysql_subnet"
}

func (p *MySQLProbe) Interval() time.Duration {
	return p.cfg.CheckInterval
}

func (p *MySQLProbe) getDB() (*sql.DB, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.db != nil {
		return p.db, nil
	}

	target := fmt.Sprintf("%s:%d", p.cfg.Host, p.cfg.Port)
	// Build DSN with network timeout parameters
	timeoutSec := int(p.cfg.Timeout.Seconds())
	if timeoutSec < 1 {
		timeoutSec = 2
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?timeout=%ds&readTimeout=%ds&writeTimeout=%ds",
		p.cfg.User,
		p.cfg.Password,
		target,
		p.cfg.Database,
		timeoutSec,
		timeoutSec,
		timeoutSec,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("erro ao configurar pool de conexão mysql (%s): %w", target, err)
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(1 * time.Minute)

	p.db = db
	return p.db, nil
}

func (p *MySQLProbe) Check(parentCtx context.Context) ProbeResult {
	target := fmt.Sprintf("%s:%d", p.cfg.Host, p.cfg.Port)
	start := time.Now()

	ctx, cancel := context.WithTimeout(parentCtx, p.cfg.Timeout)
	defer cancel()

	result := ProbeResult{
		Name:    p.Name(),
		Target:  target,
		Details: make(map[string]interface{}),
	}

	// 1. TCP connection probe (fast failure check for subnet issues)
	tcpDialer := net.Dialer{Timeout: p.cfg.Timeout}
	tcpConn, tcpErr := tcpDialer.DialContext(ctx, "tcp", target)
	if tcpErr != nil {
		latency := time.Since(start).Milliseconds()
		p.mu.Lock()
		p.failures++
		p.lastErrorMsg = fmt.Sprintf("TCP dial failed: %v", tcpErr)
		failures := p.failures
		lastSuccess := p.lastSuccess
		p.mu.Unlock()

		result.Status = StatusCritical
		result.LatencyMs = latency
		result.Failures = failures
		result.LastSuccess = lastSuccess
		result.ErrorMessage = fmt.Sprintf("TCP connection refused on subnet (%s): %v", target, tcpErr)
		return result
	}
	_ = tcpConn.Close()

	// 2. MySQL Auth & SQL Ping Probe
	db, err := p.getDB()
	if err != nil {
		latency := time.Since(start).Milliseconds()
		p.mu.Lock()
		p.failures++
		p.lastErrorMsg = err.Error()
		failures := p.failures
		lastSuccess := p.lastSuccess
		p.mu.Unlock()

		result.Status = StatusCritical
		result.LatencyMs = latency
		result.Failures = failures
		result.LastSuccess = lastSuccess
		result.ErrorMessage = err.Error()
		return result
	}

	pingErr := db.PingContext(ctx)
	latency := time.Since(start).Milliseconds()
	result.LatencyMs = latency

	if pingErr != nil {
		p.mu.Lock()
		p.failures++
		p.lastErrorMsg = pingErr.Error()
		failures := p.failures
		lastSuccess := p.lastSuccess
		// Close DB to force new pool creation on next probe
		if p.db != nil {
			_ = p.db.Close()
			p.db = nil
		}
		p.mu.Unlock()

		result.Status = StatusCritical
		result.Failures = failures
		result.LastSuccess = lastSuccess
		result.ErrorMessage = fmt.Sprintf("MySQL ping error: %v", pingErr)
		return result
	}

	// Query light status metrics
	var threadsConnected string
	row := db.QueryRowContext(ctx, "SHOW STATUS LIKE 'Threads_connected'")
	var varName string
	_ = row.Scan(&varName, &threadsConnected)
	if threadsConnected != "" {
		result.Details["threads_connected"] = threadsConnected
	}

	p.mu.Lock()
	p.lastSuccess = time.Now()
	p.failures = 0
	p.lastErrorMsg = ""
	lastSuccess := p.lastSuccess
	p.mu.Unlock()

	result.LastSuccess = lastSuccess
	result.Failures = 0

	// Check against latency SLA (< 2000ms)
	if latency > p.cfg.MaxLatencyMs {
		result.Status = StatusDegraded
		result.ErrorMessage = fmt.Sprintf("Latência de ping (%d ms) excede SLA de %d ms", latency, p.cfg.MaxLatencyMs)
	} else {
		result.Status = StatusHealthy
	}

	return result
}
