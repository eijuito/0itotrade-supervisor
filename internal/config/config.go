package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	InstanceName string         `yaml:"instance_name"`
	Server       ServerConfig   `yaml:"server"`
	MySQL        MySQLConfig    `yaml:"mysql"`
	Telegram     TelegramConfig `yaml:"telegram"`
}

type ServerConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	AuthToken string `yaml:"auth_token"` // Personal Access Token (PAT) estilo GitHub
}

type MySQLConfig struct {
	Enabled       bool          `yaml:"enabled"`
	Host          string        `yaml:"host"`
	Port          int           `yaml:"port"`
	User          string        `yaml:"user"`
	Password      string        `yaml:"password"`
	Database      string        `yaml:"database"`
	CheckInterval time.Duration `yaml:"check_interval"`
	Timeout       time.Duration `yaml:"timeout"`
	MaxLatencyMs  int64         `yaml:"max_latency_ms"`
}

type TelegramConfig struct {
	Enabled         bool   `yaml:"enabled"`
	BotToken        string `yaml:"bot_token"`
	ChatID          string `yaml:"chat_id"`
	DailyReportTime string `yaml:"daily_report_time"` // Format "HH:MM", e.g. "08:00"
	AlertOnFailure  bool   `yaml:"alert_on_failure"`
}

// LoadConfig loads configuration from a given path or standard search locations.
func LoadConfig(customPath string) (*Config, error) {
	cfg := &Config{
		InstanceName: "auto",
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 9100,
		},
		MySQL: MySQLConfig{
			Enabled:       true,
			Host:          "127.0.0.1",
			Port:          3306,
			User:          "0itotrade",
			Database:      "0itotrade",
			CheckInterval: 10 * time.Second,
			Timeout:       2000 * time.Millisecond,
			MaxLatencyMs:  2000,
		},
		Telegram: TelegramConfig{
			Enabled:         false,
			DailyReportTime: "08:00",
			AlertOnFailure:  true,
		},
	}

	searchPaths := []string{}
	if customPath != "" {
		searchPaths = append(searchPaths, customPath)
	}
	searchPaths = append(searchPaths,
		"/etc/0itotrade/supervisor.yaml",
		"./configs/supervisor.yaml",
		"supervisor.yaml",
	)

	var loadedPath string
	for _, p := range searchPaths {
		if _, err := os.Stat(p); err == nil {
			loadedPath = p
			break
		} else if os.IsPermission(err) {
			return nil, fmt.Errorf("permissão negada para ler o arquivo de configuração '%s'. Execute o comando com 'sudo' (ex: sudo 0itotrade-supervisor %s)", p, strings.Join(os.Args[1:], " "))
		}
	}

	if loadedPath != "" {
		data, err := os.ReadFile(loadedPath)
		if err != nil {
			return nil, fmt.Errorf("falha ao ler arquivo de config %s: %w", loadedPath, err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("falha ao parsear yaml de %s: %w", loadedPath, err)
		}
	}

	// Resolve hostname if InstanceName is "auto" or empty
	if strings.TrimSpace(cfg.InstanceName) == "" || strings.EqualFold(cfg.InstanceName, "auto") {
		host, err := os.Hostname()
		if err == nil && host != "" {
			cfg.InstanceName = host
		} else {
			cfg.InstanceName = "0itotrade-node"
		}
	}

	// Environment variable overrides
	if envInst := os.Getenv("SUPERVISOR_INSTANCE_NAME"); envInst != "" {
		cfg.InstanceName = envInst
	}
	if envHost := os.Getenv("SUPERVISOR_MYSQL_HOST"); envHost != "" {
		cfg.MySQL.Host = envHost
	}
	if envUser := os.Getenv("SUPERVISOR_MYSQL_USER"); envUser != "" {
		cfg.MySQL.User = envUser
	}
	if envPass := os.Getenv("SUPERVISOR_MYSQL_PASSWORD"); envPass != "" {
		cfg.MySQL.Password = envPass
	}
	if envDB := os.Getenv("SUPERVISOR_MYSQL_DATABASE"); envDB != "" {
		cfg.MySQL.Database = envDB
	}
	if envTgToken := os.Getenv("SUPERVISOR_TELEGRAM_BOT_TOKEN"); envTgToken != "" {
		cfg.Telegram.BotToken = envTgToken
		cfg.Telegram.Enabled = true
	}
	if envTgChat := os.Getenv("SUPERVISOR_TELEGRAM_CHAT_ID"); envTgChat != "" {
		cfg.Telegram.ChatID = envTgChat
	}

	// Sanity checks
	if cfg.MySQL.CheckInterval < 1*time.Second {
		cfg.MySQL.CheckInterval = 10 * time.Second
	}
	if cfg.MySQL.Timeout < 500*time.Millisecond {
		cfg.MySQL.Timeout = 2000 * time.Millisecond
	}
	if cfg.MySQL.MaxLatencyMs <= 0 {
		cfg.MySQL.MaxLatencyMs = 2000
	}
	if strings.TrimSpace(cfg.Telegram.DailyReportTime) == "" {
		cfg.Telegram.DailyReportTime = "08:00"
	}

	return cfg, nil
}
