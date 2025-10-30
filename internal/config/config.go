package config

import (
	"fmt"
	"time"
)

type Config struct {
	Database  DatabaseConfig  `mapstructure:"database"`
	Messaging MessagingConfig `mapstructure:"messaging"`
	NLP       NLPConfig       `mapstructure:"nlp"`
	Logging   LoggingConfig   `mapstructure:"logging"`
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
	URL           string        `mapstructure:"url"`
	Queues        QueuesConfig  `mapstructure:"queues"`
	Exchanges     ExchangeConfig `mapstructure:"exchanges"`
	PrefetchCount int           `mapstructure:"prefetch_count"`
}

type QueuesConfig struct {
	MaterialUploaded  string `mapstructure:"material_uploaded"`
	AssessmentAttempt string `mapstructure:"assessment_attempt"`
}

type ExchangeConfig struct {
	Materials string `mapstructure:"materials"`
}

type NLPConfig struct {
	Provider    string  `mapstructure:"provider"`
	APIKey      string  `mapstructure:"api_key"`
	Model       string  `mapstructure:"model"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	Temperature float64 `mapstructure:"temperature"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
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
	if c.NLP.APIKey == "" {
		return fmt.Errorf("OPENAI_API_KEY is required")
	}
	return nil
}
