package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eijuito/0itotrade-supervisor/internal/config"
	"github.com/eijuito/0itotrade-supervisor/internal/notifier"
	"github.com/eijuito/0itotrade-supervisor/internal/telemetry"
	"github.com/eijuito/0itotrade-supervisor/internal/watchdog"
	"github.com/eijuito/0itotrade-supervisor/internal/watchdog/probes"
)

var (
	Version   = "0.1.0"
	BuildDate = "dev"
)

func main() {
	configPath := flag.String("config", "", "Caminho customizado para o arquivo supervisor.yaml")
	flag.Parse()

	args := flag.Args()
	command := "run"
	if len(args) > 0 {
		command = args[0]
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("[ERRO] Falha ao carregar configurações: %v", err)
	}

	switch command {
	case "version":
		fmt.Printf("0itotrade-supervisor v%s (build: %s)\n", Version, BuildDate)
		return

	case "status":
		executeStatus(cfg)
		return

	case "test-telegram":
		executeTestTelegram(cfg)
		return

	case "run":
		executeRun(cfg)

	default:
		fmt.Printf("Comando desconhecido: %s\n", command)
		fmt.Println("Uso: 0itotrade-supervisor [run | status | test-telegram | version] [-config path]")
		os.Exit(1)
	}
}

func executeRun(cfg *config.Config) {
	log.Printf("=====================================================")
	log.Printf(" 0ITOTRADE-SUPERVISOR v%s", Version)
	log.Printf(" Instância: %s", cfg.InstanceName)
	log.Printf(" Telemetria: http://%s:%d/health", cfg.Server.Host, cfg.Server.Port)
	log.Printf(" Telegram Notifier Ativo: %t", cfg.Telegram.Enabled)
	if cfg.MySQL.Enabled {
		log.Printf(" Watchdog MySQL Subnet: %s:%d (Check: %s, Timeout: %s)",
			cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.CheckInterval, cfg.MySQL.Timeout)
	}
	log.Printf("=====================================================")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Inicia o Notificador do Telegram
	tgNotifier := notifier.NewTelegramNotifier(cfg.Telegram)

	// 2. Inicia o Watchdog
	wd := watchdog.NewWatchdog(cfg.InstanceName, tgNotifier)

	// Registra probe do MySQL se habilitada
	if cfg.MySQL.Enabled {
		mysqlProbe := probes.NewMySQLProbe(probes.MySQLProbeConfig{
			Host:          cfg.MySQL.Host,
			Port:          cfg.MySQL.Port,
			User:          cfg.MySQL.User,
			Password:      cfg.MySQL.Password,
			Database:      cfg.MySQL.Database,
			CheckInterval: cfg.MySQL.CheckInterval,
			Timeout:       cfg.MySQL.Timeout,
			MaxLatencyMs:  cfg.MySQL.MaxLatencyMs,
		})
		wd.RegisterProbe(mysqlProbe)
	}

	wd.Start(ctx)

	// 3. Inicia agendamento do relatório diário do Telegram
	if tgNotifier.IsEnabled() {
		tgNotifier.StartDailyScheduler(ctx, cfg.InstanceName, func() notifier.DailyReportData {
			return wd.GetDailyReportData()
		})
	}

	// 4. Inicia Servidor de Telemetria HTTP
	telemetryServer := telemetry.NewServer(cfg.Server, cfg.InstanceName, wd)
	go func() {
		if err := telemetryServer.Start(ctx); err != nil {
			log.Printf("[TELEMETRY] Erro no servidor HTTP: %v", err)
		}
	}()

	// 5. Captura sinais de encerramento do SO (SIGINT, SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	sig := <-sigChan
	log.Printf("[SUPERVISOR] Sinal de interrupção recebido (%v). Finalizando graciosamente...", sig)
	cancel()
	time.Sleep(1 * time.Second)
	log.Printf("[SUPERVISOR] Encerrado com sucesso.")
}

func executeStatus(cfg *config.Config) {
	url := fmt.Sprintf("http://%s:%d/health", cfg.Server.Host, cfg.Server.Port)
	client := http.Client{Timeout: 3 * time.Second}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Printf("Erro ao criar requisição: %v\n", err)
		os.Exit(1)
	}
	if cfg.Server.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Server.AuthToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Supervisor inacessível em %s: %v\n", url, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Erro ao ler resposta: %v\n", err)
		os.Exit(1)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		fmt.Println(string(body))
		return
	}

	prettyJSON, _ := json.MarshalIndent(data, "", "  ")
	fmt.Printf("Status do 0itotrade-supervisor (%s):\n", url)
	fmt.Println(string(prettyJSON))
}

func executeTestTelegram(cfg *config.Config) {
	if !cfg.Telegram.Enabled || cfg.Telegram.BotToken == "" || cfg.Telegram.ChatID == "" {
		fmt.Println("❌ Telegram não está configurado no arquivo supervisor.yaml ou nas variáveis de ambiente.")
		fmt.Println("Defina 'telegram.enabled: true', 'telegram.bot_token' e 'telegram.chat_id'.")
		os.Exit(1)
	}

	tg := notifier.NewTelegramNotifier(cfg.Telegram)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("Enviando mensagem de teste para o Telegram (Chat ID: %s)...\n", cfg.Telegram.ChatID)
	err := tg.SendAlert(ctx, cfg.InstanceName, "HEALTHY", "Teste de Conectividade do Telegram", "Esta é uma mensagem de teste enviada pelo comando `0itotrade-supervisor test-telegram`.")
	if err != nil {
		fmt.Printf("❌ Falha ao enviar mensagem: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Mensagem de teste enviada com sucesso ao Telegram!")
}
