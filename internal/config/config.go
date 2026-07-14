package config

import (
	"fmt"
	"time"

	rabbit "github.com/EduGoGroup/edugo-shared/messaging/rabbit"
)

type Config struct {
	Messaging       MessagingConfig       `mapstructure:"messaging"`
	NLP             NLPConfig             `mapstructure:"nlp"`
	Storage         StorageConfig         `mapstructure:"storage"`
	PDF             PDFConfig             `mapstructure:"pdf"`
	Logging         LoggingConfig         `mapstructure:"logging"`
	APIIdentity     APIIdentityConfig     `mapstructure:"api_identity"`
	APIAcademic     APIAcademicConfig     `mapstructure:"api_academic"`
	APILearning     APILearningConfig     `mapstructure:"api_learning"`
	ServiceJWT      ServiceJWTConfig      `mapstructure:"service_jwt"`
	LLM             LLMConfig             `mapstructure:"llm"`
	Metrics         MetricsConfig         `mapstructure:"metrics"`
	Health          HealthConfig          `mapstructure:"health"`
	CircuitBreakers CircuitBreakersConfig `mapstructure:"circuit_breakers"`
	RateLimiter     RateLimiterConfig     `mapstructure:"rate_limiter"`
	Shutdown        ShutdownConfig        `mapstructure:"shutdown"`
}

type MessagingConfig struct {
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
}

type RabbitMQConfig struct {
	URL           string         `mapstructure:"url"`
	Queues        QueuesConfig   `mapstructure:"queues"`
	Exchanges     ExchangeConfig `mapstructure:"exchanges"`
	PrefetchCount int            `mapstructure:"prefetch_count"`
	DLQ           DLQConfig      `mapstructure:"dlq"`
}

// DLQConfig configura el Dead Letter Queue del worker
type DLQConfig struct {
	Enabled               bool   `mapstructure:"enabled"`
	MaxRetries            int    `mapstructure:"max_retries"`
	RetryDelay            string `mapstructure:"retry_delay"`
	DLXExchange           string `mapstructure:"dlx_exchange"`
	DLXRoutingKey         string `mapstructure:"dlx_routing_key"`
	UseExponentialBackoff bool   `mapstructure:"use_exponential_backoff"`
}

// ToShared convierte la configuración local a la estructura compartida de rabbit.DLQConfig
func (c DLQConfig) ToShared() rabbit.DLQConfig {
	delay, _ := time.ParseDuration(c.RetryDelay)
	if delay == 0 {
		delay = 5 * time.Second
	}
	return rabbit.DLQConfig{
		Enabled:               c.Enabled,
		MaxRetries:            c.MaxRetries,
		RetryDelay:            delay,
		DLXExchange:           c.DLXExchange,
		DLXRoutingKey:         c.DLXRoutingKey,
		UseExponentialBackoff: c.UseExponentialBackoff,
	}
}

type QueuesConfig struct {
	MaterialUploaded string `mapstructure:"material_uploaded"`
}

type ExchangeConfig struct {
	Materials string `mapstructure:"materials"`
}

type NLPConfig struct {
	// Provider activo: "openai", "anthropic", "mock"
	Provider string `mapstructure:"provider"`

	// Configuraciones específicas por provider
	OpenAI    OpenAIConfig    `mapstructure:"openai"`
	Anthropic AnthropicConfig `mapstructure:"anthropic"`

	// Configuración general (fallback para compatibilidad)
	APIKey      string        `mapstructure:"api_key"`
	Model       string        `mapstructure:"model"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Temperature float64       `mapstructure:"temperature"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

// OpenAIConfig configuración específica para OpenAI
type OpenAIConfig struct {
	APIKey      string        `mapstructure:"api_key"`
	Model       string        `mapstructure:"model"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Temperature float64       `mapstructure:"temperature"`
	Timeout     time.Duration `mapstructure:"timeout"`
	BaseURL     string        `mapstructure:"base_url"` // Para Azure OpenAI u otros proxies
}

// AnthropicConfig configuración específica para Anthropic Claude
type AnthropicConfig struct {
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
	Level   string `mapstructure:"level"`
	Format  string `mapstructure:"format"`
	Env     string `mapstructure:"env"`
	Version string `mapstructure:"version"`
}

type MetricsConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Port    int  `mapstructure:"port"`
}

type HealthConfig struct {
	Timeouts HealthTimeoutsConfig `mapstructure:"timeouts"`
}

type HealthTimeoutsConfig struct {
	RabbitMQ time.Duration `mapstructure:"rabbitmq"`
}

type CircuitBreakersConfig struct {
	NLP     CircuitBreakerConfig `mapstructure:"nlp"`
	Storage CircuitBreakerConfig `mapstructure:"storage"`
}

type CircuitBreakerConfig struct {
	MaxFailures      uint32        `mapstructure:"max_failures"`
	Timeout          time.Duration `mapstructure:"timeout"`
	MaxRequests      uint32        `mapstructure:"max_requests"`
	SuccessThreshold uint32        `mapstructure:"success_threshold"`
}

// APIIdentityConfig configuración para conexión con api-identity (autenticación centralizada)
type APIIdentityConfig struct {
	BaseURL      string        `mapstructure:"base_url"`
	Timeout      time.Duration `mapstructure:"timeout"`
	CacheTTL     time.Duration `mapstructure:"cache_ttl"`
	CacheEnabled bool          `mapstructure:"cache_enabled"`
	MaxBulkSize  int           `mapstructure:"max_bulk_size"`
}

// APIAcademicConfig configura el cliente M2M hacia edugo-api-academic (lectura de
// settings de escuela, plan 039 D-039.6). CacheTTL corto mitiga el riesgo de leer
// config por M2M en runtime (design 039 §7).
type APIAcademicConfig struct {
	BaseURL  string        `mapstructure:"base_url"`
	Timeout  time.Duration `mapstructure:"timeout"`
	CacheTTL time.Duration `mapstructure:"cache_ttl"`
}

// APILearningConfig configura el cliente M2M hacia edugo-api-learning. En 039 es
// solo andamiaje (stub); el plan 040 define qué endpoints consume.
type APILearningConfig struct {
	BaseURL string        `mapstructure:"base_url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// ServiceJWTConfig configura la firma del service JWT M2M (HS256) que el worker
// presenta a las APIs de dominio. Secret = SERVICE_JWT_SECRET (env, Secret
// Manager en cloud), distinto del secret de usuarios. Issuer/Audience siguen la
// convención del ecosistema (academic espera iss=edugo-identity, aud=edugo-api-academic).
type ServiceJWTConfig struct {
	Secret   string        `mapstructure:"secret"`
	Issuer   string        `mapstructure:"issuer"`
	Audience string        `mapstructure:"audience"`
	ClientID string        `mapstructure:"client_id"`
	TTL      time.Duration `mapstructure:"ttl"`
}

// LLMConfig agrupa la configuración de los providers LLM (plan 039 D-039.3). Las
// credenciales/URL/modelo son de EduGo (NO por escuela); lo por-escuela es solo
// la política, que se lee vía M2M. Estos valores se inyectan al constructor del
// provider —el provider NUNCA lee env directo—.
type LLMConfig struct {
	Local LLMLocalConfig `mapstructure:"local"`
	API   LLMAPIConfig   `mapstructure:"api"`
}

// LLMLocalConfig configura el provider local (Ollama). Env: LLM_LOCAL_BASE_URL,
// LLM_LOCAL_MODEL.
type LLMLocalConfig struct {
	BaseURL string        `mapstructure:"base_url"`
	Model   string        `mapstructure:"model"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// LLMAPIConfig configura el provider por API (Claude/Gemini). Env:
// LLM_API_PROVIDER, LLM_API_KEY (Secret Manager en cloud), LLM_API_MODEL.
type LLMAPIConfig struct {
	Provider  string        `mapstructure:"provider"`
	APIKey    string        `mapstructure:"api_key"`
	Model     string        `mapstructure:"model"`
	BaseURL   string        `mapstructure:"base_url"`
	Timeout   time.Duration `mapstructure:"timeout"`
	MaxTokens int           `mapstructure:"max_tokens"`
}

func (c *Config) Validate() error {
	if c.Messaging.RabbitMQ.URL == "" {
		return fmt.Errorf("RABBITMQ_URL is required")
	}
	// NLP.APIKey es opcional - si no está, usamos SmartFallback
	return nil
}

// GetActiveNLPConfig retorna la configuración del provider NLP activo con valores por defecto
func (c *Config) GetActiveNLPConfig() (apiKey, model string, maxTokens int, temperature float64, timeout time.Duration, baseURL string) {
	switch c.NLP.Provider {
	case "openai":
		// Usar configuración específica de OpenAI si existe
		if c.NLP.OpenAI.APIKey != "" {
			apiKey = c.NLP.OpenAI.APIKey
			model = c.NLP.OpenAI.Model
			maxTokens = c.NLP.OpenAI.MaxTokens
			temperature = c.NLP.OpenAI.Temperature
			timeout = c.NLP.OpenAI.Timeout
			baseURL = c.NLP.OpenAI.BaseURL
		} else {
			// Fallback a configuración general
			apiKey = c.NLP.APIKey
			model = c.NLP.Model
			maxTokens = c.NLP.MaxTokens
			temperature = c.NLP.Temperature
			timeout = c.NLP.Timeout
		}

	case "anthropic":
		// Usar configuración específica de Anthropic si existe
		if c.NLP.Anthropic.APIKey != "" {
			apiKey = c.NLP.Anthropic.APIKey
			model = c.NLP.Anthropic.Model
			maxTokens = c.NLP.Anthropic.MaxTokens
			temperature = c.NLP.Anthropic.Temperature
			timeout = c.NLP.Anthropic.Timeout
		} else {
			// Fallback a configuración general
			apiKey = c.NLP.APIKey
			model = c.NLP.Model
			maxTokens = c.NLP.MaxTokens
			temperature = c.NLP.Temperature
			timeout = c.NLP.Timeout
		}

	default:
		// "mock" o cualquier otro provider usa configuración general
		apiKey = c.NLP.APIKey
		model = c.NLP.Model
		maxTokens = c.NLP.MaxTokens
		temperature = c.NLP.Temperature
		timeout = c.NLP.Timeout
	}

	// Aplicar defaults si no están configurados
	if model == "" {
		switch c.NLP.Provider {
		case "openai":
			model = "gpt-4-turbo-preview"
		case "anthropic":
			model = "claude-3-sonnet-20240229"
		}
	}
	if maxTokens == 0 {
		maxTokens = 4096
	}
	if temperature == 0 {
		temperature = 0.7
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return apiKey, model, maxTokens, temperature, timeout, baseURL
}

// GetAPIIdentityConfigWithDefaults retorna la configuración de api-identity con valores por defecto
func (c *Config) GetAPIIdentityConfigWithDefaults() APIIdentityConfig {
	cfg := c.APIIdentity
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8070/api"
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

// GetAPIAcademicConfigWithDefaults retorna la config del cliente M2M de academic
// con defaults (base_url local :8060, timeout 5s, cache 60s).
func (c *Config) GetAPIAcademicConfigWithDefaults() APIAcademicConfig {
	cfg := c.APIAcademic
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8060"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 60 * time.Second
	}
	return cfg
}

// GetAPILearningConfigWithDefaults retorna la config del stub M2M de learning con
// defaults (base_url local :8065, timeout 5s).
func (c *Config) GetAPILearningConfigWithDefaults() APILearningConfig {
	cfg := c.APILearning
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8065"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	return cfg
}

// GetServiceJWTConfigWithDefaults retorna la config del service JWT con defaults.
// Issuer/Audience/ClientID por convención del ecosistema; Secret NO tiene default
// (viene de env SERVICE_JWT_SECRET, vacío en local sin M2M real).
func (c *Config) GetServiceJWTConfigWithDefaults() ServiceJWTConfig {
	cfg := c.ServiceJWT
	if cfg.Issuer == "" {
		cfg.Issuer = "edugo-identity"
	}
	if cfg.Audience == "" {
		cfg.Audience = "edugo-api-academic"
	}
	if cfg.ClientID == "" {
		cfg.ClientID = "edugo-worker"
	}
	if cfg.TTL == 0 {
		cfg.TTL = 15 * time.Minute
	}
	return cfg
}

// GetLLMConfigWithDefaults retorna la config LLM con defaults conservadores.
func (c *Config) GetLLMConfigWithDefaults() LLMConfig {
	cfg := c.LLM
	if cfg.Local.BaseURL == "" {
		cfg.Local.BaseURL = "http://localhost:11434"
	}
	if cfg.Local.Timeout == 0 {
		cfg.Local.Timeout = 120 * time.Second
	}
	if cfg.API.Provider == "" {
		cfg.API.Provider = "anthropic"
	}
	if cfg.API.Timeout == 0 {
		cfg.API.Timeout = 60 * time.Second
	}
	if cfg.API.MaxTokens == 0 {
		cfg.API.MaxTokens = 4096
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

// GetHealthConfigWithDefaults retorna la configuración de health checks con valores por defecto
func (c *Config) GetHealthConfigWithDefaults() HealthConfig {
	cfg := c.Health
	if cfg.Timeouts.RabbitMQ == 0 {
		cfg.Timeouts.RabbitMQ = 3 * time.Second
	}
	return cfg
}

// GetDLQConfigWithDefaults retorna la configuración DLQ con valores por defecto
func (c *Config) GetDLQConfigWithDefaults() DLQConfig {
	cfg := c.Messaging.RabbitMQ.DLQ

	if !cfg.Enabled {
		cfg.Enabled = true
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay == "" {
		cfg.RetryDelay = "5s"
	}
	if cfg.DLXExchange == "" {
		cfg.DLXExchange = "edugo_dlx"
	}
	if cfg.DLXRoutingKey == "" {
		cfg.DLXRoutingKey = "edugo.material.uploaded.dlq"
	}
	if !cfg.UseExponentialBackoff {
		cfg.UseExponentialBackoff = true
	}

	return cfg
}

// GetExchangesConfigWithDefaults retorna la configuración de exchanges con valores por defecto
func (c *Config) GetExchangesConfigWithDefaults() ExchangeConfig {
	cfg := c.Messaging.RabbitMQ.Exchanges
	if cfg.Materials == "" {
		cfg.Materials = "edugo.materials"
	}
	return cfg
}

// GetCircuitBreakerConfigWithDefaults retorna la configuración de un circuit breaker con valores por defecto
func (c *CircuitBreakerConfig) GetWithDefaults() CircuitBreakerConfig {
	cfg := *c
	if cfg.MaxFailures == 0 {
		cfg.MaxFailures = 5
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.MaxRequests == 0 {
		cfg.MaxRequests = 1
	}
	if cfg.SuccessThreshold == 0 {
		cfg.SuccessThreshold = 2
	}
	return cfg
}

// RateLimiterConfig configuración del rate limiter
type RateLimiterConfig struct {
	Enabled     bool                            `mapstructure:"enabled"`
	ByEventType map[string]EventRateLimitConfig `mapstructure:"by_event_type"`
	Default     EventRateLimitConfig            `mapstructure:"default"`
}

// EventRateLimitConfig configuración de rate limiting para un tipo de evento
type EventRateLimitConfig struct {
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`
	BurstSize         float64 `mapstructure:"burst_size"`
}

// GetRateLimiterConfigWithDefaults retorna la configuración del rate limiter con valores por defecto
func (c *Config) GetRateLimiterConfigWithDefaults() RateLimiterConfig {
	cfg := c.RateLimiter

	// Si no hay configuración por defecto, establecer una razonable
	if cfg.Default.RequestsPerSecond == 0 {
		cfg.Default.RequestsPerSecond = 10
	}
	if cfg.Default.BurstSize == 0 {
		cfg.Default.BurstSize = 20
	}

	return cfg
}

// ShutdownConfig configuración del graceful shutdown
type ShutdownConfig struct {
	Timeout         time.Duration `mapstructure:"timeout"`
	WaitForMessages bool          `mapstructure:"wait_for_messages"`
}

// GetShutdownConfigWithDefaults retorna la configuración de shutdown con valores por defecto
func (c *Config) GetShutdownConfigWithDefaults() ShutdownConfig {
	cfg := c.Shutdown

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	// WaitForMessages viene directamente de la configuración sin defaults
	// El valor en config.yaml define el comportamiento

	return cfg
}
