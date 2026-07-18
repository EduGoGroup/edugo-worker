package config

import (
	"fmt"
	"time"

	rabbit "github.com/EduGoGroup/edugo-shared/messaging/rabbit"
)

type Config struct {
	Messaging        MessagingConfig        `mapstructure:"messaging"`
	NLP              NLPConfig              `mapstructure:"nlp"`
	PDF              PDFConfig              `mapstructure:"pdf"`
	Logging          LoggingConfig          `mapstructure:"logging"`
	APIIdentity      APIIdentityConfig      `mapstructure:"api_identity"`
	APIAcademic      APIAcademicConfig      `mapstructure:"api_academic"`
	APILearning      APILearningConfig      `mapstructure:"api_learning"`
	MaterialPipeline MaterialPipelineConfig `mapstructure:"material_pipeline"`
	ServiceJWT       ServiceJWTConfig       `mapstructure:"service_jwt"`
	LLM              LLMConfig              `mapstructure:"llm"`
	Metrics          MetricsConfig          `mapstructure:"metrics"`
	Health           HealthConfig           `mapstructure:"health"`
	CircuitBreakers  CircuitBreakersConfig  `mapstructure:"circuit_breakers"`
	RateLimiter      RateLimiterConfig      `mapstructure:"rate_limiter"`
	Shutdown         ShutdownConfig         `mapstructure:"shutdown"`
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
	// MaterialUploaded es la cola del carril de materiales. Plan 040: el worker
	// dejó de consumirla (su processor/binding se retiraron); el plan 041 la revive
	// con su propio processor. Se conserva en config para ese retorno.
	MaterialUploaded string `mapstructure:"material_uploaded"`
	// AttemptReviewRequested es la cola del carril de revisión asistida (plan 040):
	// recibe los eventos attempt.review_requested que publica learning al hacer submit.
	AttemptReviewRequested string `mapstructure:"attempt_review_requested"`
	// QuestionPrepRequested es la cola del carril de preparación (plan 042 F2a):
	// recibe los eventos question.prep_requested que publica learning al crear/editar
	// una pregunta short_answer/open_ended. Canal propio por riel (D-042.3): NO comparte
	// cola con el carril de revisión.
	QuestionPrepRequested string `mapstructure:"question_prep_requested"`
	// MaterialAssessmentRequested es la cola del carril material→evaluación (plan 043 F3c):
	// recibe los eventos material.assessment_requested que publica learning al pedir la
	// generación de una evaluación desde un material. Canal propio por riel (D-043): NO
	// comparte cola con revisión ni preparación.
	MaterialAssessmentRequested string `mapstructure:"material_assessment_requested"`
}

type ExchangeConfig struct {
	// Materials lo publica learning (carril materiales). El worker sigue declarándolo
	// aunque no consuma su cola: el publisher no declara exchanges, así que un Rabbit
	// fresco rompería el publish de material si el worker dejara de declararlo.
	Materials string `mapstructure:"materials"`
	// Assessments lo publica learning para el carril de evaluaciones/revisión (plan 040).
	Assessments string `mapstructure:"assessments"`
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
	NLP CircuitBreakerConfig `mapstructure:"nlp"`
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

// MaterialPipelineConfig configura el carril material→evaluación (plan 043 F2):
// el límite de descarga del PDF y los parámetros del porcionado determinista
// (D-043.6). El chunking (F2d) los consume para armar trozos por conteo de palabras:
// apunta a ChunkTargetWords, corta antes de ChunkMaxWords, no baja de ChunkMinWords,
// y un resto por debajo de ChunkMergeThresholdWords se fusiona con el trozo anterior.
type MaterialPipelineConfig struct {
	// DownloadMaxBytes es el techo de bytes de la descarga del PDF (coherente con el
	// límite del extractor). Un archivo mayor se rechaza como permanente (no reintenta).
	DownloadMaxBytes int64 `mapstructure:"download_max_bytes"`
	// ChunkTargetWords es el tamaño objetivo (en palabras) de cada porción.
	ChunkTargetWords int `mapstructure:"chunk_target_words"`
	// ChunkMaxWords es el techo duro de palabras por porción (corte forzado).
	ChunkMaxWords int `mapstructure:"chunk_max_words"`
	// ChunkMinWords es el piso de palabras por porción antes de intentar cortar.
	ChunkMinWords int `mapstructure:"chunk_min_words"`
	// ChunkMergeThresholdWords: un resto final por debajo de este umbral se fusiona
	// con la porción anterior en lugar de quedar como un trozo diminuto.
	ChunkMergeThresholdWords int `mapstructure:"chunk_merge_threshold_words"`
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
	// Temperature del muestreo local. Default 0 (greedy determinista): la
	// corrección pide JSON estructurado, no prosa creativa, y el determinismo la
	// hace reproducible. Env: LLM_LOCAL_TEMPERATURE.
	Temperature float64 `mapstructure:"temperature"`
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

// GetMaterialPipelineConfigWithDefaults retorna la config del carril de materiales
// con defaults (descarga 100MB; porcionado objetivo 650 / max 800 / min 500 palabras,
// umbral de fusión 150). Los defaults de chunking encarnan D-043.6.
func (c *Config) GetMaterialPipelineConfigWithDefaults() MaterialPipelineConfig {
	cfg := c.MaterialPipeline
	if cfg.DownloadMaxBytes == 0 {
		cfg.DownloadMaxBytes = 100 * 1024 * 1024
	}
	if cfg.ChunkTargetWords == 0 {
		cfg.ChunkTargetWords = 650
	}
	if cfg.ChunkMaxWords == 0 {
		cfg.ChunkMaxWords = 800
	}
	if cfg.ChunkMinWords == 0 {
		cfg.ChunkMinWords = 500
	}
	if cfg.ChunkMergeThresholdWords == 0 {
		cfg.ChunkMergeThresholdWords = 150
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
		// El único consumidor activo del worker es el carril de revisión (plan 040);
		// la cola muerta se nombra por él. Es también el nombre de la cola DLQ que
		// declara el consumer compartido (setupDLQ usa la routing key como nombre).
		cfg.DLXRoutingKey = "edugo.attempt.review_requested.dlq"
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
	if cfg.Assessments == "" {
		cfg.Assessments = "edugo.assessments"
	}
	return cfg
}

// GetQueuesConfigWithDefaults retorna la configuración de colas con valores por defecto.
func (c *Config) GetQueuesConfigWithDefaults() QueuesConfig {
	cfg := c.Messaging.RabbitMQ.Queues
	if cfg.MaterialUploaded == "" {
		cfg.MaterialUploaded = "edugo.material.uploaded"
	}
	if cfg.AttemptReviewRequested == "" {
		cfg.AttemptReviewRequested = "edugo.attempt.review_requested"
	}
	if cfg.QuestionPrepRequested == "" {
		cfg.QuestionPrepRequested = "edugo.question.prep_requested"
	}
	if cfg.MaterialAssessmentRequested == "" {
		cfg.MaterialAssessmentRequested = "edugo.material.assessment.requested"
	}
	return cfg
}

// PrepDLQName es el nombre de la cola muerta del carril de preparación. Sigue la
// convención del carril de revisión (cola + ".dlq"): dead-letters propios por riel.
func (c QueuesConfig) PrepDLQName() string {
	q := c.QuestionPrepRequested
	if q == "" {
		q = "edugo.question.prep_requested"
	}
	return q + ".dlq"
}

// MaterialAssessmentDLQName es el nombre de la cola muerta del carril material→
// evaluación (plan 043 F3c). Sigue la convención de los otros rieles (cola + ".dlq"):
// dead-letters propios por riel.
func (c QueuesConfig) MaterialAssessmentDLQName() string {
	q := c.MaterialAssessmentRequested
	if q == "" {
		q = "edugo.material.assessment.requested"
	}
	return q + ".dlq"
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
