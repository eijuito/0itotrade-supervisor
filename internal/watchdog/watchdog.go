package watchdog

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/eijuito/0itotrade-supervisor/internal/notifier"
	"github.com/eijuito/0itotrade-supervisor/internal/watchdog/probes"
)

type OverallStatus string

const (
	OverallHealthy  OverallStatus = "HEALTHY"
	OverallDegraded OverallStatus = "DEGRADED"
	OverallCritical OverallStatus = "CRITICAL"
)

type Watchdog struct {
	instanceName   string
	probes         []probes.Probe
	circuitBreaker *CircuitBreaker
	notifier       *notifier.TelegramNotifier

	mu           sync.RWMutex
	lastResults  map[string]probes.ProbeResult
	lastState    OverallStatus
	startTime    time.Time
}

func NewWatchdog(instanceName string, notifier *notifier.TelegramNotifier) *Watchdog {
	return &Watchdog{
		instanceName:   instanceName,
		circuitBreaker: NewCircuitBreaker(5, 2*time.Second, 60*time.Second),
		notifier:       notifier,
		lastResults:    make(map[string]probes.ProbeResult),
		lastState:      OverallHealthy,
		startTime:      time.Now(),
	}
}

func (w *Watchdog) RegisterProbe(p probes.Probe) {
	w.probes = append(w.probes, p)
}

func (w *Watchdog) Start(ctx context.Context) {
	log.Printf("[WATCHDOG] Iniciando motor de monitoramento para instância: %s (%d probes registradas)", w.instanceName, len(w.probes))

	for _, p := range w.probes {
		probe := p
		go w.runProbeLoop(ctx, probe)
	}
}

func (w *Watchdog) runProbeLoop(ctx context.Context, p probes.Probe) {
	ticker := time.NewTicker(p.Interval())
	defer ticker.Stop()

	// Execute initial check immediately
	w.executeCheck(ctx, p)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[WATCHDOG] Finalizando loop da probe: %s", p.Name())
			return
		case <-ticker.C:
			w.executeCheck(ctx, p)
		}
	}
}

func (w *Watchdog) executeCheck(ctx context.Context, p probes.Probe) {
	if !w.circuitBreaker.CanExecute() {
		log.Printf("[CIRCUIT_BREAKER] Execução da probe %s pausada pelo circuit breaker (estado: %s)", p.Name(), w.circuitBreaker.State())
		return
	}

	res := p.Check(ctx)

	w.mu.Lock()
	w.lastResults[p.Name()] = res
	currentOverall := w.calculateOverallStatusLocked()
	previousState := w.lastState
	w.lastState = currentOverall
	w.mu.Unlock()

	// Circuit breaker management & logging
	if res.Status == probes.StatusCritical {
		tripped := w.circuitBreaker.RecordFailure()
		log.Printf("[PROBE_ALERT] Falha crítica em %s (%s): %s (Falhas consecutivas: %d)",
			res.Name, res.Target, res.ErrorMessage, res.Failures)

		if tripped {
			log.Printf("[CIRCUIT_BREAKER] Circuit breaker desarmado para estado OPEN após %d falhas consecutivas!", w.circuitBreaker.Failures())
		}
	} else {
		w.circuitBreaker.RecordSuccess()
		if res.Status == probes.StatusDegraded {
			log.Printf("[PROBE_WARN] %s em estado DEGRADADO: %s (latência: %d ms)",
				res.Name, res.ErrorMessage, res.LatencyMs)
		}
	}

	// Trigger alert on state transitions
	if previousState != currentOverall {
		w.handleStateTransition(ctx, p.Name(), previousState, currentOverall, res)
	}
}

func (w *Watchdog) handleStateTransition(ctx context.Context, probeName string, from, to OverallStatus, res probes.ProbeResult) {
	log.Printf("[WATCHDOG] Transição de estado de saúde: %s -> %s (causado por %s)", from, to, probeName)

	if w.notifier == nil || !w.notifier.IsEnabled() {
		return
	}

	go func() {
		alertCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if to == OverallCritical {
			details := fmt.Sprintf("• *Probe:* `%s`\n• *Alvo:* `%s`\n• *Erro:* `%s`\n• *Circuit Breaker:* `%s`",
				res.Name, res.Target, res.ErrorMessage, w.circuitBreaker.State())
			_ = w.notifier.SendAlert(alertCtx, w.instanceName, "CRITICAL", "Alvo Inacessível na Subnet", details)
		} else if from == OverallCritical && (to == OverallHealthy || to == OverallDegraded) {
			details := fmt.Sprintf("• *Probe:* `%s`\n• *Alvo:* `%s`\n• *Latência:* `%d ms`\n• *Status:* `%s`",
				res.Name, res.Target, res.LatencyMs, to)
			_ = w.notifier.SendAlert(alertCtx, w.instanceName, "RECOVERED", "Serviço Recuperado com Sucesso", details)
		}
	}()
}

func (w *Watchdog) calculateOverallStatusLocked() OverallStatus {
	hasCritical := false
	hasDegraded := false

	for _, r := range w.lastResults {
		if r.Status == probes.StatusCritical {
			hasCritical = true
		} else if r.Status == probes.StatusDegraded {
			hasDegraded = true
		}
	}

	if hasCritical {
		return OverallCritical
	}
	if hasDegraded {
		return OverallDegraded
	}
	return OverallHealthy
}

func (w *Watchdog) GetSnapshot() (OverallStatus, map[string]probes.ProbeResult, time.Duration) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	resultsCopy := make(map[string]probes.ProbeResult, len(w.lastResults))
	for k, v := range w.lastResults {
		resultsCopy[k] = v
	}

	return w.lastState, resultsCopy, time.Since(w.startTime)
}

func (w *Watchdog) GetDailyReportData() notifier.DailyReportData {
	status, results, uptime := w.GetSnapshot()

	data := notifier.DailyReportData{
		Status:             string(status),
		Uptime:             uptime.Round(time.Second).String(),
		MemoryRSS:          "Normal (<30MB)",
		MySQLStatus:        "N/A",
		LastSuccessfulPing: "N/A",
	}

	if mysqlRes, ok := results["mysql_subnet"]; ok {
		data.MySQLTarget = mysqlRes.Target
		data.MySQLStatus = string(mysqlRes.Status)
		data.MySQLLatencyMs = mysqlRes.LatencyMs
		data.MySQLFailures = mysqlRes.Failures
		if !mysqlRes.LastSuccess.IsZero() {
			data.LastSuccessfulPing = mysqlRes.LastSuccess.Format("2006-01-02 15:04:05")
		}
	}

	return data
}
