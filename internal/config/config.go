package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Telegram  TelegramConfig  `yaml:"telegram"`
	LLM       LLMConfig       `yaml:"llm"`
	Admin     AdminConfig     `yaml:"admin"`
	Currency  CurrencyConfig  `yaml:"currency"`
	Timezone  string          `yaml:"timezone"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type DatabaseConfig struct {
	DSN string `yaml:"dsn"`
}

type TelegramConfig struct {
	Token      string `yaml:"token"`
	WebhookURL string `yaml:"webhook_url"`
	WebhookPath string `yaml:"webhook_path"`
}

type LLMConfig struct {
	BaseURL    string  `yaml:"base_url"`
	APIKey     string  `yaml:"api_key"`
	Model      string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens  int     `yaml:"max_tokens"`
	Mock       bool    `yaml:"mock"`
	TimeoutSec int     `yaml:"timeout_sec"`
	// ReasoningEffort is sent only for reasoning models (gpt-oss): low keeps tokens/TPM down.
	ReasoningEffort string `yaml:"reasoning_effort"`
	// HistoryWindow is how many stored history messages are sent to the LLM per request.
	HistoryWindow int `yaml:"history_window"`
}

type AdminConfig struct {
	User string `yaml:"user"`
	Pass string `yaml:"pass"`
}

// Rates: how many BYN kopecks for 1 unit of foreign currency (or 1 BYN).
// Example: USD = 32000 means 1 USD = 320.00 BYN.
type CurrencyConfig struct {
	BYNToKopecks int64 `yaml:"byn_to_kopecks"` // always 100
	USDToBYNKopecks int64 `yaml:"usd_to_byn_kopecks"`
	RUBToBYNKopecks int64 `yaml:"rub_to_byn_kopecks"`
}

func Load(path string) (*Config, error) {
	// .env from project root (and /app/.env in container). Does not override existing env.
	LoadDotEnv(".env", "/app/.env")

	cfg := defaults()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config: %w", err)
		}
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parse config: %w", err)
			}
		}
	}

	applyEnv(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Server: ServerConfig{Addr: ":8080"},
		Database: DatabaseConfig{
			DSN: "postgres://expenses:expenses@localhost:5432/expenses?sslmode=disable",
		},
		Telegram: TelegramConfig{
			WebhookPath: "/telegram/webhook",
		},
		LLM: LLMConfig{
			BaseURL:     "https://api.groq.com/openai/v1",
			Model:       "openai/gpt-oss-120b",
			Temperature: 0,
			MaxTokens:   1024,
			Mock:        false,
			TimeoutSec:  30,
			ReasoningEffort: "low",
			HistoryWindow:   12,
		},
		Admin: AdminConfig{
			User: "admin",
			Pass: "admin",
		},
		Currency: CurrencyConfig{
			BYNToKopecks:    100,
			USDToBYNKopecks: 300, // 1 USD = 3.00 BYN
			RUBToBYNKopecks: 4,   // 1 RUB ≈ 0.04 BYN
		},
		Timezone: "Europe/Moscow",
	}
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("SERVER_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("DATABASE_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("TELEGRAM_TOKEN"); v != "" {
		cfg.Telegram.Token = v
	}
	if v := os.Getenv("TELEGRAM_WEBHOOK_URL"); v != "" {
		cfg.Telegram.WebhookURL = v
	}
	if v := os.Getenv("TELEGRAM_WEBHOOK_PATH"); v != "" {
		cfg.Telegram.WebhookPath = v
	}
	if v := os.Getenv("LLM_BASE_URL"); v != "" {
		cfg.LLM.BaseURL = v
	}
	if v := os.Getenv("LLM_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		cfg.LLM.Model = v
	}
	if v := os.Getenv("LLM_MOCK"); v != "" {
		cfg.LLM.Mock = parseBool(v)
	}
	if v := os.Getenv("LLM_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.LLM.Temperature = f
		}
	}
	if v := os.Getenv("LLM_MAX_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.LLM.MaxTokens = n
		}
	}
	if v, ok := os.LookupEnv("LLM_REASONING_EFFORT"); ok {
		cfg.LLM.ReasoningEffort = v
	}
	if v := os.Getenv("LLM_HISTORY_WINDOW"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.LLM.HistoryWindow = n
		}
	}
	if v := os.Getenv("ADMIN_USER"); v != "" {
		cfg.Admin.User = v
	}
	if v := os.Getenv("ADMIN_PASS"); v != "" {
		cfg.Admin.Pass = v
	}
	if v := os.Getenv("TIMEZONE"); v != "" {
		cfg.Timezone = v
	}
	if v := os.Getenv("USD_TO_BYN_KOPECKS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Currency.USDToBYNKopecks = n
		}
	}
	if v := os.Getenv("RUB_TO_BYN_KOPECKS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Currency.RUBToBYNKopecks = n
		}
	}
}

func (c *Config) Validate() error {
	if c.Database.DSN == "" {
		return fmt.Errorf("database dsn is required")
	}
	if c.Timezone == "" {
		return fmt.Errorf("timezone is required")
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}
	if c.Admin.User == "" || c.Admin.Pass == "" {
		return fmt.Errorf("admin credentials are required")
	}
	return nil
}

func (c *Config) Location() *time.Location {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
