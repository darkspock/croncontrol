// Package config loads CronControl configuration from YAML + environment variables.
package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Database    DatabaseConfig    `mapstructure:"database"`
	Planner     PlannerConfig     `mapstructure:"planner"`
	Executor    ExecutorConfig    `mapstructure:"executor"`
	Monitor     MonitorConfig     `mapstructure:"monitor"`
	Concurrency ConcurrencyConfig `mapstructure:"concurrency"`
	Auth        AuthConfig        `mapstructure:"auth"`
	Retention   RetentionConfig   `mapstructure:"retention"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	SaaS        SaaSConfig        `mapstructure:"saas"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"sslmode"`
}

// URL builds a properly percent-encoded PostgreSQL connection string.
func (d DatabaseConfig) URL() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(d.User, d.Password),
		Host:   fmt.Sprintf("%s:%d", d.Host, d.Port),
		Path:   d.Name,
	}
	q := u.Query()
	q.Set("sslmode", d.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

type PlannerConfig struct {
	Interval string `mapstructure:"interval"`
	Horizon  string `mapstructure:"horizon"`
}

type ExecutorConfig struct {
	Interval string `mapstructure:"interval"`
}

type MonitorConfig struct {
	Interval string `mapstructure:"interval"`
}

type ConcurrencyConfig struct {
	MaxGlobal int `mapstructure:"max_global"`
	MaxHTTP   int `mapstructure:"max_http"`
	MaxSSH    int `mapstructure:"max_ssh"`
	MaxSSM    int `mapstructure:"max_ssm"`
	MaxK8s    int `mapstructure:"max_k8s"`
}

type AuthConfig struct {
	GoogleClientID     string `mapstructure:"google_client_id"`
	GoogleClientSecret string `mapstructure:"google_client_secret"`
	SessionSecret      string `mapstructure:"session_secret"`
	EncryptionKey      string `mapstructure:"encryption_key"`
}

// EncryptionKeyBytes returns the encryption key as a 32-byte slice for AES-256.
func (a AuthConfig) EncryptionKeyBytes() ([]byte, error) {
	key := []byte(a.EncryptionKey)
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be exactly 32 bytes, got %d", len(key))
	}
	return key, nil
}

type RetentionConfig struct {
	Slots            string `mapstructure:"slots"`
	Output           string `mapstructure:"output"`
	Heartbeats       string `mapstructure:"heartbeats"`
	Jobs             string `mapstructure:"jobs"`
	JobAttempts      string `mapstructure:"job_attempts"`
	Audit            string `mapstructure:"audit"`
	CleanupSchedule  string `mapstructure:"cleanup_schedule"`
	CleanupBatchSize int    `mapstructure:"cleanup_batch_size"`
}

type LoggingConfig struct {
	Backend           string `mapstructure:"backend"`
	Level             string `mapstructure:"level"`
	FilePath          string `mapstructure:"file_path"`
	FileMaxSize       string `mapstructure:"file_max_size"`
	FileMaxAge        string `mapstructure:"file_max_age"`
	OpenSearchURL     string `mapstructure:"opensearch_url"`
	OpenSearchPrefix  string `mapstructure:"opensearch_prefix"`
	OpenSearchAuth    string `mapstructure:"opensearch_auth"`
}

type SaaSConfig struct {
	RegistrationEnabled  bool   `mapstructure:"registration_enabled"`
	DisposableEmailCheck bool   `mapstructure:"disposable_email_check"`
	BaseURL              string `mapstructure:"base_url"`
	PlatformAdminEmail   string `mapstructure:"platform_admin_email"` // auto-promote on startup
}

// Load reads configuration from config.yaml and environment variables.
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("CC")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}
