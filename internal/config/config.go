package config

import (
	"fmt"
	"time"
)

type Config struct {
	Database  DatabaseConfig  `mapstructure:"database"`
	Messaging MessagingConfig `mapstructure:"messaging"`
	NLP       NLPConfig       `mapstructure:"nlp"`
	Storage   StorageConfig   `mapstructure:"storage"`
	PDF       PDFConfig       `mapstructure:"pdf"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	APIAdmin  APIAdminConfig  `mapstructure:"api_admin"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
}

type DatabaseConfig struct {
	Postgres PostgresConfig `mapstructure:"postgres"`
	MongoDB  MongoDBConfig  `mapstructure:"mongodb"`
}

type PostgresConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	Database       string `mapstructure:"database"`
	User           string `mapstructure:"user"`
	Password       string `mapstructure:"password"`
	MaxConnections int    `mapstructure:"max_connections"`
	SSLMode        string `mapstructure:"ssl_mode"`
}

type MongoDBConfig struct {
	URI      string        `mapstructure:"uri"`
	Database string        `mapstructure:"database"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

type MessagingConfig struct {
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
}

type RabbitMQConfig struct {
	URL           string         `mapstructure:"url"`
	Queues        QueuesConfig   `mapstructure:"queues"`
	Exchanges     ExchangeConfig `mapstructure:"exchanges"`
	PrefetchCount int            `mapstructure:"prefetch_count"`
}

type QueuesConfig struct {
	MaterialUploaded  string `mapstructure:"material_uploaded"`
	AssessmentAttempt string `mapstructure:"assessment_attempt"`
}

type ExchangeConfig struct {
	Materials string `mapstructure:"materials"`
}

type NLPConfig struct {
	Provider    string        `mapstructure:"provider"`
	APIKey      string        `mapstructure:"api_key"`
	Model       string        `mapstructure:"model"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Temperature float64       `mapstructure:"temperature"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

type StorageConfig struct {
	Provider string        `mapstructure:"provider"`
	S3       S3Config      `mapstructure:"s3"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

type S3Config struct {
	Region          string        `mapstructure:"region"`
	Bucket          string        `mapstructure:"bucket"`
	Endpoint        string        `mapstructure:"endpoint"` // Para MinIO
	AccessKeyID     string        `mapstructure:"access_key_id"`
	SecretAccessKey string        `mapstructure:"secret_access_key"`
	UsePathStyle    bool          `mapstructure:"use_path_style"` // Para MinIO
	Timeout         time.Duration `mapstructure:"timeout"`
}

type PDFConfig struct {
	MaxSizeMB    int           `mapstructure:"max_size_mb"`
	AllowedTypes []string      `mapstructure:"allowed_types"`
	Timeout      time.Duration `mapstructure:"timeout"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type MetricsConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Port    int  `mapstructure:"port"`
}

// APIAdminConfig configuración para conexión con api-admin (autenticación centralizada)
type APIAdminConfig struct {
	BaseURL      string        `mapstructure:"base_url"`
	Timeout      time.Duration `mapstructure:"timeout"`
	CacheTTL     time.Duration `mapstructure:"cache_ttl"`
	CacheEnabled bool          `mapstructure:"cache_enabled"`
	MaxBulkSize  int           `mapstructure:"max_bulk_size"`
}

func (c *Config) Validate() error {
	if c.Database.Postgres.Password == "" {
		return fmt.Errorf("POSTGRES_PASSWORD is required")
	}
	if c.Database.MongoDB.URI == "" {
		return fmt.Errorf("MONGODB_URI is required")
	}
	if c.Messaging.RabbitMQ.URL == "" {
		return fmt.Errorf("RABBITMQ_URL is required")
	}
	// NLP.APIKey es opcional - si no está, usamos SmartFallback
	return nil
}

// GetAPIAdminConfigWithDefaults retorna la configuración de api-admin con valores por defecto
func (c *Config) GetAPIAdminConfigWithDefaults() APIAdminConfig {
	cfg := c.APIAdmin
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8081"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 60 * time.Second
	}
	if cfg.MaxBulkSize == 0 {
		cfg.MaxBulkSize = 50
	}
	return cfg
}

// GetMetricsConfigWithDefaults retorna la configuración de métricas con valores por defecto
func (c *Config) GetMetricsConfigWithDefaults() MetricsConfig {
	cfg := c.Metrics
	if cfg.Port == 0 {
		cfg.Port = 9090
	}
	return cfg
}
