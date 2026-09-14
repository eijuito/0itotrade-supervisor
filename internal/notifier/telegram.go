package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/eijuito/0itotrade-supervisor/internal/config"
)

type DailyReportData struct {
	Status             string
	Uptime             string
	MemoryRSS          string
	MySQLTarget        string
	MySQLStatus        string
	MySQLLatencyMs     int64
	MySQLFailures      int
	LastSuccessfulPing string
}

type TelegramNotifier struct {
	cfg        config.TelegramConfig
	httpClient *http.Client
}

func NewTelegramNotifier(cfg config.TelegramConfig) *TelegramNotifier {
	return &TelegramNotifier{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (t *TelegramNotifier) IsEnabled() bool {
	return t.cfg.Enabled && t.cfg.BotToken != "" && t.cfg.ChatID != ""
}

type telegramPayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// SendMessage sends a raw markdown-formatted message to Telegram.
func (t *TelegramNotifier) SendMessage(ctx context.Context, text string) error {
	if !t.IsEnabled() {
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.cfg.BotToken)
	payload := telegramPayload{
		ChatID:    t.cfg.ChatID,
		Text:      text,
		ParseMode: "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("erro ao serializar payload telegram: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("erro ao criar requisicao telegram: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("erro ao enviar notificacao telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API retornou status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// SendAlert sends an immediate status alert (e.g. MySQL down or recovered).
func (t *TelegramNotifier) SendAlert(ctx context.Context, instanceName, alertLevel, title, details string) error {
	if !t.IsEnabled() || !t.cfg.AlertOnFailure {
		return nil
	}

	icon := "⚠️"
	if alertLevel == "CRITICAL" {
		icon = "🚨"
	} else if alertLevel == "RECOVERED" || alertLevel == "HEALTHY" {
		icon = "✅"
	}

	msg := fmt.Sprintf("%s *[0ITOTRADE ALERT]* %s\n\n"+
		"🖥️ *Instância:* `%s`\n"+
		"📌 *Título:* %s\n"+
		"🕒 *Horário:* `%s`\n\n"+
		"ℹ️ *Detalhes:*\n%s",
		icon, alertLevel,
		instanceName,
		title,
		time.Now().Format("2006-01-02 15:04:05 MST"),
		details,
	)

	return t.SendMessage(ctx, msg)
}

// SendDailyReport formats and dispatches the 24h operational status report.
func (t *TelegramNotifier) SendDailyReport(ctx context.Context, instanceName string, data DailyReportData) error {
	if !t.IsEnabled() {
		return nil
	}

	statusIcon := "🟢"
	if data.Status == "DEGRADED" {
		statusIcon = "🟡"
	} else if data.Status == "CRITICAL" {
		statusIcon = "🔴"
	}

	msg := fmt.Sprintf("📊 *[0ITOTRADE SUPERVISOR]* Relatório Diário\n\n"+
		"🖥️ *Instância:* `%s`\n"+
		"%s *Status Geral:* `%s`\n"+
		"⏱️ *Uptime:* `%s`\n"+
		"🧠 *Memória (RSS):* `%s`\n\n"+
		"🗄️ *MySQL (Subnet):*\n"+
		"  • Alvo: `%s`\n"+
		"  • Status: `%s`\n"+
		"  • Latência: `%d ms`\n"+
		"  • Falhas Recentes: `%d`\n"+
		"  • Último Ping OK: `%s`\n\n"+
		"🕒 *Gerado em:* `%s`",
		instanceName,
		statusIcon, data.Status,
		data.Uptime,
		data.MemoryRSS,
		data.MySQLTarget,
		data.MySQLStatus,
		data.MySQLLatencyMs,
		data.MySQLFailures,
		data.LastSuccessfulPing,
		time.Now().Format("2006-01-02 15:04:05 MST"),
	)

	return t.SendMessage(ctx, msg)
}

// StartDailyScheduler runs a background worker firing the daily report at configured daily_report_time (HH:MM).
func (t *TelegramNotifier) StartDailyScheduler(ctx context.Context, instanceName string, reportProvider func() DailyReportData) {
	if !t.IsEnabled() {
		return
	}

	go func() {
		for {
			nextTrigger := t.calculateNextRun(t.cfg.DailyReportTime)
			waitDuration := time.Until(nextTrigger)
			log.Printf("[TELEGRAM] Próximo relatório diário agendado para: %s (em %s)", nextTrigger.Format("2006-01-02 15:04:05"), waitDuration.Round(time.Minute))

			select {
			case <-ctx.Done():
				return
			case <-time.After(waitDuration):
				data := reportProvider()
				sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				if err := t.SendDailyReport(sendCtx, instanceName, data); err != nil {
					log.Printf("[TELEGRAM] Erro ao enviar relatório diário: %v", err)
				} else {
					log.Printf("[TELEGRAM] Relatório diário enviado com sucesso para chat ID %s", t.cfg.ChatID)
				}
				cancel()
				// Small sleep to prevent re-triggering within the same minute
				time.Sleep(2 * time.Minute)
			}
		}
	}()
}

func (t *TelegramNotifier) calculateNextRun(hhmm string) time.Time {
	parts := strings.Split(hhmm, ":")
	hour := 8
	minute := 0
	if len(parts) == 2 {
		if h, err := strconv.Atoi(parts[0]); err == nil {
			hour = h
		}
		if m, err := strconv.Atoi(parts[1]); err == nil {
			minute = m
		}
	}

	now := time.Now()
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !target.After(now) {
		target = target.Add(24 * time.Hour)
	}
	return target
}
