package publisher

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config — параметры Publisher'а.
type Config struct {
	SSPAddr         string
	APIKey          string
	SlotName        string
	RPS             int
	Duration        time.Duration
	ImpressionDelay time.Duration
	ReportInterval  time.Duration
	Verbose         bool
}

// ParseFlags читает параметры из командной строки и переменных
// окружения. Если --api-key не задан, берётся из SSP_API_KEY,
// затем из DSP_API_KEY (в .env это один и тот же ключ).
func ParseFlags() Config {
	var cfg Config

	flag.StringVar(&cfg.SSPAddr, "ssp", "localhost:50051", "SSP gRPC address")
	flag.StringVar(&cfg.APIKey, "api-key", "", "publisher api-key (default from SSP_API_KEY env)")
	flag.StringVar(&cfg.SlotName, "slot", "home_banner", "slot name to bid on")
	flag.IntVar(&cfg.RPS, "rps", 1, "requests per second")
	flag.DurationVar(&cfg.Duration, "duration", 0, "run duration (0 = until Ctrl+C)")
	flag.DurationVar(&cfg.ImpressionDelay, "impression-delay", 100*time.Millisecond,
		"delay between RunAuction and Impression (simulates ad impression)")
	flag.DurationVar(&cfg.ReportInterval, "report-interval", 5*time.Second,
		"interval between progress reports")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "log every request")

	flag.Parse()

	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("SSP_API_KEY")
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("DSP_API_KEY")
	}

	return cfg
}

// Validate проверяет, что параметры консистентны.
func (c Config) Validate() error {
	if strings.TrimSpace(c.SSPAddr) == "" {
		return fmt.Errorf("ssp address is required")
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("api-key is required (set --api-key or SSP_API_KEY)")
	}
	if strings.TrimSpace(c.SlotName) == "" {
		return fmt.Errorf("slot name is required")
	}
	if c.RPS <= 0 {
		return fmt.Errorf("rps must be positive, got %d", c.RPS)
	}
	if c.Duration < 0 {
		return fmt.Errorf("duration cannot be negative")
	}
	if c.ImpressionDelay < 0 {
		return fmt.Errorf("impression-delay cannot be negative")
	}
	if c.ReportInterval <= 0 {
		return fmt.Errorf("report-interval must be positive")
	}
	return nil
}

// Interval возвращает интервал между запросами.
func (c Config) Interval() time.Duration {
	return time.Second / time.Duration(c.RPS)
}
