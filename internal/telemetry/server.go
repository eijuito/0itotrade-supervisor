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
	mux.HandleFunc("/", s.handleDashboard)

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

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>0itotrade-supervisor | Dashboard</title>
  <style>
    :root {
      --bg: #0b0f19;
      --card-bg: #111827;
      --card-border: #1f2937;
      --text: #f3f4f6;
      --text-muted: #9ca3af;
      --accent: #3b82f6;
      --green: #10b981;
      --yellow: #f59e0b;
      --red: #ef4444;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
    body { background-color: var(--bg); color: var(--text); padding: 2rem; min-height: 100vh; }
    .container { max-width: 900px; margin: 0 auto; }
    header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 2rem; border-bottom: 1px solid var(--card-border); padding-bottom: 1rem; }
    .brand { font-size: 1.4rem; font-weight: 700; color: #60a5fa; display: flex; align-items: center; gap: 0.5rem; }
    .badge { padding: 0.35rem 0.85rem; border-radius: 9999px; font-weight: 600; font-size: 0.85rem; text-transform: uppercase; letter-spacing: 0.05em; }
    .badge-HEALTHY { background: rgba(16, 185, 129, 0.15); color: var(--green); border: 1px solid var(--green); }
    .badge-DEGRADED { background: rgba(245, 158, 11, 0.15); color: var(--yellow); border: 1px solid var(--yellow); }
    .badge-CRITICAL { background: rgba(239, 68, 68, 0.15); color: var(--red); border: 1px solid var(--red); }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 1.25rem; margin-bottom: 2rem; }
    .card { background: var(--card-bg); border: 1px solid var(--card-border); border-radius: 0.75rem; padding: 1.25rem; }
    .card-title { font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); margin-bottom: 0.5rem; }
    .card-value { font-size: 1.5rem; font-weight: 700; }
    .probes-section { background: var(--card-bg); border: 1px solid var(--card-border); border-radius: 0.75rem; padding: 1.5rem; }
    .probes-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
    .probe-item { background: #0f172a; border: 1px solid #1e293b; border-radius: 0.5rem; padding: 1rem; margin-bottom: 0.75rem; display: flex; justify-content: space-between; align-items: center; }
    .probe-info h4 { font-size: 1.05rem; margin-bottom: 0.25rem; }
    .probe-info p { font-size: 0.85rem; color: var(--text-muted); }
    .probe-stats { text-align: right; }
    .latency { font-size: 1.2rem; font-weight: 700; color: #38bdf8; }
    .footer { text-align: center; font-size: 0.8rem; color: var(--text-muted); margin-top: 2rem; }
    .pulse { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: var(--green); margin-right: 6px; animation: blink 1.5s infinite; }
    @keyframes blink { 0%, 100% { opacity: 1; } 50% { opacity: 0.3; } }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <div class="brand">
        <span>🛡️ 0itotrade-supervisor</span>
      </div>
      <div id="status-badge" class="badge badge-HEALTHY"><span class="pulse"></span>Carregando...</div>
    </header>

    <div class="grid">
      <div class="card">
        <div class="card-title">🖥️ Nome da Instância</div>
        <div class="card-value" id="val-instance">-</div>
      </div>
      <div class="card">
        <div class="card-title">⏱️ Tempo Ativo (Uptime)</div>
        <div class="card-value" id="val-uptime">-</div>
      </div>
      <div class="card">
        <div class="card-title">🧠 Memória (Alocada / Sys)</div>
        <div class="card-value" id="val-memory">-</div>
      </div>
    </div>

    <div class="probes-section">
      <div class="probes-header">
        <h3>Sondas de Monitoramento (Watchdog)</h3>
        <span style="font-size: 0.8rem; color: var(--text-muted);">Atualização automática a cada 5s</span>
      </div>
      <div id="probes-list">Carregando sondas...</div>
    </div>

    <div class="footer">
      0itotrade Platform • Endpoint de API: <a href="/health" style="color: #60a5fa;">/health</a>
    </div>
  </div>

  <script>
    async function updateDashboard() {
      try {
        const res = await fetch('/health');
        const data = await res.json();

        // Status badge
        const badge = document.getElementById('status-badge');
        badge.className = 'badge badge-' + data.status;
        badge.innerHTML = '<span class="pulse"></span>' + data.status;

        // Cards
        document.getElementById('val-instance').innerText = data.instance_name;
        
        const uptimeMins = Math.floor(data.uptime_seconds / 60);
        const uptimeHours = Math.floor(uptimeMins / 60);
        document.getElementById('val-uptime').innerText = 
          uptimeHours > 0 ? (uptimeHours + 'h ' + (uptimeMins % 60) + 'm') : (uptimeMins + ' min ' + Math.floor(data.uptime_seconds % 60) + 's');
        
        document.getElementById('val-memory').innerText = 
          data.memory_alloc_mb.toFixed(1) + ' MB / ' + data.memory_sys_mb.toFixed(1) + ' MB';

        // Probes
        const probesDiv = document.getElementById('probes-list');
        probesDiv.innerHTML = '';
        for (const [key, probe] of Object.entries(data.probes || {})) {
          const item = document.createElement('div');
          item.className = 'probe-item';
          item.innerHTML = 
            '<div class="probe-info">' +
              '<h4>🗄️ ' + probe.name + '</h4>' +
              '<p>Alvo: <code>' + probe.target + '</code> | Falhas consecutivas: ' + probe.consecutive_failures + '</p>' +
            '</div>' +
            '<div class="probe-stats">' +
              '<div class="latency">' + probe.latency_ms + ' ms</div>' +
              '<span class="badge badge-' + probe.status + '">' + probe.status + '</span>' +
            '</div>';
          probesDiv.appendChild(item);
        }
      } catch (err) {
        console.error('Falha ao atualizar telemetria:', err);
      }
    }

    updateDashboard();
    setInterval(updateDashboard, 5000);
  </script>
</body>
</html>`
	_, _ = w.Write([]byte(html))
}
