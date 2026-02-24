package config

import (
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Server             ServerConfig             `mapstructure:"server"`
	Auth               AuthConfig               `mapstructure:"auth"`
	Email              EmailConfig              `mapstructure:"email"`
	CORS               CORSConfig               `mapstructure:"cors"`
	RateLimit          RateLimitConfig          `mapstructure:"rate_limit"`
	Redis              RedisConfig              `mapstructure:"redis"`
	Supabase           SupabaseConfig           `mapstructure:"supabase"`
	Queue              QueueConfig              `mapstructure:"queue"`
	RecipientRateLimit RecipientRateLimitConfig `mapstructure:"recipient_rate_limit"`
	Reaper             ReaperConfigYAML         `mapstructure:"reaper"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// AuthConfig holds API key authentication settings.
type AuthConfig struct {
	APIKeys []string `mapstructure:"api_keys"`
}

// EmailConfig holds email provider settings.
type EmailConfig struct {
	Provider    string `mapstructure:"provider"`
	APIKey      string `mapstructure:"api_key"`
	FromAddress string `mapstructure:"from_address"`
	FromName    string `mapstructure:"from_name"`
}

// CORSConfig holds CORS policy settings.
type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	AllowedMethods []string `mapstructure:"allowed_methods"`
	AllowedHeaders []string `mapstructure:"allowed_headers"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`
	Burst             int     `mapstructure:"burst"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// SupabaseConfig holds Supabase project settings.
type SupabaseConfig struct {
	URL        string `mapstructure:"url"`
	ServiceKey string `mapstructure:"service_key"`
}

// QueueConfig holds async queue settings.
type QueueConfig struct {
	Concurrency   int `mapstructure:"concurrency"`
	MaxRetry      int `mapstructure:"max_retry"`
	RetryDelaySec int `mapstructure:"retry_delay_sec"`
}

// RecipientRateLimitConfig holds per-recipient rate limiting settings.
type RecipientRateLimitConfig struct {
	MaxPerHour int `mapstructure:"max_per_hour"`
}

// ReaperConfigYAML holds stale task reaper settings (durations as seconds for YAML/env compat).
type ReaperConfigYAML struct {
	IntervalSec       int `mapstructure:"interval_sec"`
	StaleThresholdSec int `mapstructure:"stale_threshold_sec"`
	BatchSize         int `mapstructure:"batch_size"`
}

// Load reads configuration from config.yaml and environment variables.
// Environment variables use the NOTIFLY_ prefix and underscore separators.
// Example: NOTIFLY_SERVER_PORT overrides server.port in config.yaml.
func Load() (*Config, error) {
	v := viper.New()

	// Config file settings
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	// Load .env file if it exists
	_ = godotenv.Load()

	// Environment variable settings
	v.SetEnvPrefix("NOTIFLY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Defaults
	v.SetDefault("server.port", 8081)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("email.provider", "resend")
	v.SetDefault("rate_limit.requests_per_second", 10)
	v.SetDefault("rate_limit.burst", 20)
	v.SetDefault("redis.address", "localhost:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("queue.concurrency", 10)
	v.SetDefault("queue.max_retry", 5)
	v.SetDefault("queue.retry_delay_sec", 30)
	v.SetDefault("recipient_rate_limit.max_per_hour", 3)
	v.SetDefault("reaper.interval_sec", 300)         // 5 minutes
	v.SetDefault("reaper.stale_threshold_sec", 600)   // 10 minutes
	v.SetDefault("reaper.batch_size", 50)

	// Read config file (optional — env vars can provide everything)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Handle comma-separated API keys from env var
	if apiKeysStr := v.GetString("auth.api_keys"); apiKeysStr != "" && len(cfg.Auth.APIKeys) == 0 {
		keys := strings.Split(apiKeysStr, ",")
		for i := range keys {
			keys[i] = strings.TrimSpace(keys[i])
		}
		cfg.Auth.APIKeys = keys
	}

	return &cfg, nil
}
