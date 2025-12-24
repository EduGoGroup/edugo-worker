package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetRateLimiterConfigWithDefaults_ConValoresConfigurados(t *testing.T) {
	// Configuración con valores específicos
	cfg := &Config{
		RateLimiter: RateLimiterConfig{
			Enabled: true,
			Default: EventRateLimitConfig{
				RequestsPerSecond: 50.0,
				BurstSize:         100.0,
			},
			ByEventType: map[string]EventRateLimitConfig{
				"material_uploaded": {
					RequestsPerSecond: 30.0,
					BurstSize:         60.0,
				},
				"assessment_attempt": {
					RequestsPerSecond: 40.0,
					BurstSize:         80.0,
				},
			},
		},
	}

	result := cfg.GetRateLimiterConfigWithDefaults()

	assert.True(t, result.Enabled, "El rate limiter debería estar habilitado")
	assert.Equal(t, 50.0, result.Default.RequestsPerSecond, "Requests por segundo por defecto deberían ser 50")
	assert.Equal(t, 100.0, result.Default.BurstSize, "Burst size por defecto debería ser 100")

	// Verificar configuraciones específicas por tipo de evento
	materialConfig, exists := result.ByEventType["material_uploaded"]
	assert.True(t, exists, "Debería existir configuración para material_uploaded")
	assert.Equal(t, 30.0, materialConfig.RequestsPerSecond)
	assert.Equal(t, 60.0, materialConfig.BurstSize)

	assessmentConfig, exists := result.ByEventType["assessment_attempt"]
	assert.True(t, exists, "Debería existir configuración para assessment_attempt")
	assert.Equal(t, 40.0, assessmentConfig.RequestsPerSecond)
	assert.Equal(t, 80.0, assessmentConfig.BurstSize)
}

func TestGetRateLimiterConfigWithDefaults_SinConfiguracion(t *testing.T) {
	// Sin configuración explícita (valores en cero)
	cfg := &Config{
		RateLimiter: RateLimiterConfig{
			Enabled: false,
		},
	}

	result := cfg.GetRateLimiterConfigWithDefaults()

	// Verificar valores por defecto establecidos en el código
	assert.False(t, result.Enabled, "El rate limiter debería estar deshabilitado por defecto")
	assert.Equal(t, 10.0, result.Default.RequestsPerSecond, "Requests por segundo por defecto deberían ser 10")
	assert.Equal(t, 20.0, result.Default.BurstSize, "Burst size por defecto debería ser 20")
	assert.Nil(t, result.ByEventType, "No debería haber configuraciones por tipo de evento")
}

func TestGetRateLimiterConfigWithDefaults_ConfiguracionParcial(t *testing.T) {
	// Solo requests per second configurado, burst size en cero
	cfg := &Config{
		RateLimiter: RateLimiterConfig{
			Enabled: true,
			Default: EventRateLimitConfig{
				RequestsPerSecond: 25.0,
				BurstSize:         0, // No configurado
			},
		},
	}

	result := cfg.GetRateLimiterConfigWithDefaults()

	assert.True(t, result.Enabled)
	assert.Equal(t, 25.0, result.Default.RequestsPerSecond, "Debería respetar el valor configurado")
	assert.Equal(t, 20.0, result.Default.BurstSize, "Debería aplicar el valor por defecto para BurstSize")
}

func TestGetRateLimiterConfigWithDefaults_MultipleEventTypes(t *testing.T) {
	// Múltiples tipos de eventos con diferentes configuraciones
	cfg := &Config{
		RateLimiter: RateLimiterConfig{
			Enabled: true,
			Default: EventRateLimitConfig{
				RequestsPerSecond: 15.0,
				BurstSize:         30.0,
			},
			ByEventType: map[string]EventRateLimitConfig{
				"event_type_1": {
					RequestsPerSecond: 5.0,
					BurstSize:         10.0,
				},
				"event_type_2": {
					RequestsPerSecond: 20.0,
					BurstSize:         40.0,
				},
				"event_type_3": {
					RequestsPerSecond: 100.0,
					BurstSize:         200.0,
				},
			},
		},
	}

	result := cfg.GetRateLimiterConfigWithDefaults()

	assert.Len(t, result.ByEventType, 3, "Deberían existir 3 tipos de eventos configurados")

	// Verificar cada tipo de evento
	for eventType, expectedConfig := range cfg.RateLimiter.ByEventType {
		actualConfig, exists := result.ByEventType[eventType]
		assert.True(t, exists, "Debería existir configuración para %s", eventType)
		assert.Equal(t, expectedConfig.RequestsPerSecond, actualConfig.RequestsPerSecond)
		assert.Equal(t, expectedConfig.BurstSize, actualConfig.BurstSize)
	}
}

func TestEventRateLimitConfig_ValoresZero(t *testing.T) {
	// Validar comportamiento cuando ambos valores son cero
	cfg := &Config{
		RateLimiter: RateLimiterConfig{
			Default: EventRateLimitConfig{
				RequestsPerSecond: 0,
				BurstSize:         0,
			},
		},
	}

	result := cfg.GetRateLimiterConfigWithDefaults()

	// Ambos deberían tomar valores por defecto
	assert.Equal(t, 10.0, result.Default.RequestsPerSecond)
	assert.Equal(t, 20.0, result.Default.BurstSize)
}

func TestGetAPIAdminConfigWithDefaults_ConValoresConfigurados(t *testing.T) {
	cfg := &Config{
		APIAdmin: APIAdminConfig{
			BaseURL:      "http://api-admin:8080",
			Timeout:      10 * time.Second,
			CacheTTL:     120 * time.Second,
			CacheEnabled: true,
			MaxBulkSize:  100,
		},
	}

	result := cfg.GetAPIAdminConfigWithDefaults()

	assert.Equal(t, "http://api-admin:8080", result.BaseURL)
	assert.Equal(t, 10*time.Second, result.Timeout)
	assert.Equal(t, 120*time.Second, result.CacheTTL)
	assert.True(t, result.CacheEnabled)
	assert.Equal(t, 100, result.MaxBulkSize)
}

func TestGetAPIAdminConfigWithDefaults_SinConfiguracion(t *testing.T) {
	cfg := &Config{}

	result := cfg.GetAPIAdminConfigWithDefaults()

	assert.Equal(t, "http://localhost:8081", result.BaseURL, "Debería usar BaseURL por defecto")
	assert.Equal(t, 5*time.Second, result.Timeout, "Debería usar timeout por defecto")
	assert.Equal(t, 60*time.Second, result.CacheTTL, "Debería usar CacheTTL por defecto")
	assert.Equal(t, 50, result.MaxBulkSize, "Debería usar MaxBulkSize por defecto")
}

func TestGetMetricsConfigWithDefaults_ConValoresConfigurados(t *testing.T) {
	cfg := &Config{
		Metrics: MetricsConfig{
			Enabled: true,
			Port:    8080,
		},
	}

	result := cfg.GetMetricsConfigWithDefaults()

	assert.True(t, result.Enabled)
	assert.Equal(t, 8080, result.Port)
}

func TestGetMetricsConfigWithDefaults_SinConfiguracion(t *testing.T) {
	cfg := &Config{}

	result := cfg.GetMetricsConfigWithDefaults()

	assert.Equal(t, 9090, result.Port, "Debería usar puerto 9090 por defecto")
}

func TestGetHealthConfigWithDefaults_ConValoresConfigurados(t *testing.T) {
	cfg := &Config{
		Health: HealthConfig{
			Timeouts: HealthTimeoutsConfig{
				MongoDB:  10 * time.Second,
				Postgres: 8 * time.Second,
				RabbitMQ: 6 * time.Second,
			},
		},
	}

	result := cfg.GetHealthConfigWithDefaults()

	assert.Equal(t, 10*time.Second, result.Timeouts.MongoDB)
	assert.Equal(t, 8*time.Second, result.Timeouts.Postgres)
	assert.Equal(t, 6*time.Second, result.Timeouts.RabbitMQ)
}

func TestGetHealthConfigWithDefaults_SinConfiguracion(t *testing.T) {
	cfg := &Config{}

	result := cfg.GetHealthConfigWithDefaults()

	assert.Equal(t, 5*time.Second, result.Timeouts.MongoDB, "MongoDB timeout por defecto debería ser 5s")
	assert.Equal(t, 3*time.Second, result.Timeouts.Postgres, "Postgres timeout por defecto debería ser 3s")
	assert.Equal(t, 3*time.Second, result.Timeouts.RabbitMQ, "RabbitMQ timeout por defecto debería ser 3s")
}

func TestCircuitBreakerConfig_GetWithDefaults_ConValoresConfigurados(t *testing.T) {
	cb := CircuitBreakerConfig{
		MaxFailures:      10,
		Timeout:          120 * time.Second,
		MaxRequests:      5,
		SuccessThreshold: 3,
	}

	result := cb.GetWithDefaults()

	assert.Equal(t, uint32(10), result.MaxFailures)
	assert.Equal(t, 120*time.Second, result.Timeout)
	assert.Equal(t, uint32(5), result.MaxRequests)
	assert.Equal(t, uint32(3), result.SuccessThreshold)
}

func TestCircuitBreakerConfig_GetWithDefaults_SinConfiguracion(t *testing.T) {
	cb := CircuitBreakerConfig{}

	result := cb.GetWithDefaults()

	assert.Equal(t, uint32(5), result.MaxFailures, "MaxFailures por defecto debería ser 5")
	assert.Equal(t, 60*time.Second, result.Timeout, "Timeout por defecto debería ser 60s")
	assert.Equal(t, uint32(1), result.MaxRequests, "MaxRequests por defecto debería ser 1")
	assert.Equal(t, uint32(2), result.SuccessThreshold, "SuccessThreshold por defecto debería ser 2")
}

func TestValidate_ConfiguracionCompleta(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			Postgres: PostgresConfig{
				Password: "test-password",
			},
			MongoDB: MongoDBConfig{
				URI: "mongodb://localhost:27017",
			},
		},
		Messaging: MessagingConfig{
			RabbitMQ: RabbitMQConfig{
				URL: "amqp://localhost:5672",
			},
		},
	}

	err := cfg.Validate()
	assert.NoError(t, err, "La validación debería pasar con configuración completa")
}

func TestValidate_SinPostgresPassword(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			MongoDB: MongoDBConfig{
				URI: "mongodb://localhost:27017",
			},
		},
		Messaging: MessagingConfig{
			RabbitMQ: RabbitMQConfig{
				URL: "amqp://localhost:5672",
			},
		},
	}

	err := cfg.Validate()
	assert.Error(t, err, "Debería fallar sin POSTGRES_PASSWORD")
	assert.Contains(t, err.Error(), "POSTGRES_PASSWORD is required")
}

func TestValidate_SinMongoDBURI(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			Postgres: PostgresConfig{
				Password: "test-password",
			},
		},
		Messaging: MessagingConfig{
			RabbitMQ: RabbitMQConfig{
				URL: "amqp://localhost:5672",
			},
		},
	}

	err := cfg.Validate()
	assert.Error(t, err, "Debería fallar sin MONGODB_URI")
	assert.Contains(t, err.Error(), "MONGODB_URI is required")
}

func TestValidate_SinRabbitMQURL(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			Postgres: PostgresConfig{
				Password: "test-password",
			},
			MongoDB: MongoDBConfig{
				URI: "mongodb://localhost:27017",
			},
		},
	}

	err := cfg.Validate()
	assert.Error(t, err, "Debería fallar sin RABBITMQ_URL")
	assert.Contains(t, err.Error(), "RABBITMQ_URL is required")
}
