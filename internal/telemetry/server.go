package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/eijuito/0itotrade-supervisor/internal/config"
	"github.com/eijuito/0itotrade-supervisor/internal/watchdog"
	"github.com/eijuito/0itotrade-supervisor/internal/watchdog/probes"
)

type Server struct {
	cfg          config.ServerConfig
	instanceName string
	wd           *watchdog.Watchdog
	httpServer   *http.Server
}

func NewServer(cfg config.ServerConfig, instanceName string, wd *watchdog.Watchdog) *Server {
	return &Server{
		cfg:          cfg,
		instanceName: instanceName,
		wd:           wd,
	}
}

type HealthResponse struct {
	Status        string                        `json:"status"`
	InstanceName  string                        `json:"instance_name"`
	UptimeSeconds float64                       `json:"uptime_seconds"`
	MemoryAllocMB float64                       `json:"memory_alloc_mb"`
	MemorySysMB   float64                       `json:"memory_sys_mb"`
	NumGoroutine  int                           `json:"num_goroutine"`
	Timestamp     string                        `json:"timestamp"`
	Probes        map[string]probes.ProbeResult `json:"probes"`
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong\n"))
	})

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("[TELEMETRY] Servidor HTTP de telemetria ativo em http://%s/health", addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("erro no servidor http de telemetria: %w", err)
	}

	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// If auth token is configured, enforce bearer token
	if s.cfg.AuthToken != "" {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + s.cfg.AuthToken
		if auth != expected {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
	}

	overallStatus, results, uptime := s.wd.GetSnapshot()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	resp := HealthResponse{
		Status:        string(overallStatus),
		InstanceName:  s.instanceName,
		UptimeSeconds: uptime.Seconds(),
		MemoryAllocMB: float64(m.Alloc) / 1024 / 1024,
		MemorySysMB:   float64(m.Sys) / 1024 / 1024,
		NumGoroutine:  runtime.NumGoroutine(),
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Probes:        results,
	}

	w.Header().Set("Content-Type", "application/json")
	if overallStatus == watchdog.OverallCritical {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	_ = json.NewEncoder(w).Encode(resp)
}
