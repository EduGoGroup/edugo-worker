package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/EduGoGroup/edugo-shared/lifecycle"
	"github.com/EduGoGroup/edugo-shared/logger"
	rabbit "github.com/EduGoGroup/edugo-shared/messaging/rabbit"
	sharedMetrics "github.com/EduGoGroup/edugo-shared/metrics"
	"github.com/EduGoGroup/edugo-worker/internal/application/processor"
	"github.com/EduGoGroup/edugo-worker/internal/client"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/config"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/health"
	httpInfra "github.com/EduGoGroup/edugo-worker/internal/infrastructure/http"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	llmapi "github.com/EduGoGroup/edugo-worker/internal/llm/api"
	"github.com/EduGoGroup/edugo-worker/internal/llm/ollama"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Resources contiene todos los recursos inicializados para el worker
type Resources struct {
	Logger            logger.Logger
	RabbitMQConn      *rabbit.Connection
	RabbitMQChannel   *amqp.Channel
	AuthClient        *client.AuthClient
	SettingsClient    *m2m.SettingsClient
	LearningClient    *m2m.LearningClient
	LLMProvider       llm.LLMProvider
	LifecycleManager  *lifecycle.Manager
	ProcessorRegistry *processor.Registry
	MetricsServer     *httpInfra.MetricsServer
	HealthChecker     *health.Checker
	SharedMetrics     *sharedMetrics.Metrics
}

// ResourceBuilder construye Resources de forma incremental usando el patrón Builder
type ResourceBuilder struct {
	config *config.Config
	ctx    context.Context

	// Recursos de infraestructura base
	logger           logger.Logger
	rabbitSharedConn *rabbit.Connection
	rabbitConn       *amqp.Connection
	rabbitChannel    *amqp.Channel

	// Servicios de infraestructura externa
	pdfExtractor pdf.Extractor
	nlpClient    nlp.Client

	// Recursos de aplicación
	authClient        *client.AuthClient
	settingsClient    *m2m.SettingsClient
	learningClient    *m2m.LearningClient
	llmProvider       llm.LLMProvider
	llmProviders      map[string]llm.LLMProvider
	processorRegistry *processor.Registry
	metricsServer     *httpInfra.MetricsServer
	healthChecker     *health.Checker
	sharedMetrics     *sharedMetrics.Metrics

	// Lifecycle
	lifecycleManager *lifecycle.Manager
	cleanupFuncs     []func() error

	// Control de errores
	err error
}

// NewResourceBuilder crea un nuevo builder de recursos
func NewResourceBuilder(ctx context.Context, cfg *config.Config) *ResourceBuilder {
	return &ResourceBuilder{
		config:       cfg,
		ctx:          ctx,
		cleanupFuncs: make([]func() error, 0),
	}
}

// WithLogger configura el logger (debe ser llamado primero)
func (b *ResourceBuilder) WithLogger() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	// Crear logger usando shared/logger (slog centralizado)
	slogLogger := logger.NewSlogProvider(logger.SlogConfig{
		Level:   b.config.Logging.Level,
		Format:  b.config.Logging.Format,
		Service: "edugo-worker",
		Env:     b.config.Logging.Env,
		Version: b.config.Logging.Version,
	})
	b.logger = logger.NewSlogAdapter(slogLogger)

	// Registrar cleanup
	b.addCleanup(func() error {
		b.logger.Info("syncing logger")
		return b.logger.Sync()
	})

	b.logger.Info("Logger initialized successfully")
	return b
}

// WithSharedMetrics configura el facade de métricas centralizado (shared/metrics).
// Es complementario al servidor Prometheus existente (internal/infrastructure/metrics).
func (b *ResourceBuilder) WithSharedMetrics() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	b.sharedMetrics = sharedMetrics.New("edugo-worker")

	if b.logger != nil {
		b.logger.Info("shared metrics facade initialized", "service", "edugo-worker")
	}

	return b
}

// WithRabbitMQ configura la conexión a RabbitMQ usando el wrapper compartido
func (b *ResourceBuilder) WithRabbitMQ() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	if b.logger == nil {
		b.err = fmt.Errorf("logger required before RabbitMQ")
		return b
	}

	b.logger.Info("connecting to RabbitMQ")

	// Crear conexión usando el wrapper compartido que gestiona conexión + canal
	rabbitConn, err := rabbit.Connect(b.config.Messaging.RabbitMQ.URL)
	if err != nil {
		b.err = fmt.Errorf("failed to connect to RabbitMQ: %w", err)
		return b
	}

	// Guardar referencias: derivamos las raw desde el wrapper compartido
	b.rabbitSharedConn = rabbitConn
	b.rabbitConn = rabbitConn.GetConnection()
	b.rabbitChannel = rabbitConn.GetChannel()

	// Registrar cleanup: cerrar el wrapper compartido cierra canal + conexión
	b.addCleanup(func() error {
		b.logger.Info("closing RabbitMQ connection")
		if err := b.rabbitSharedConn.Close(); err != nil {
			return fmt.Errorf("failed to close RabbitMQ: %w", err)
		}
		return nil
	})

	b.logger.Info("✅ RabbitMQ connected successfully")
	return b
}

// WithAuthClient configura el cliente de autenticación
func (b *ResourceBuilder) WithAuthClient() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	apiIdentityCfg := b.config.GetAPIIdentityConfigWithDefaults()
	b.authClient = client.NewAuthClient(client.AuthClientConfig{
		BaseURL:      apiIdentityCfg.BaseURL,
		Timeout:      apiIdentityCfg.Timeout,
		CacheTTL:     apiIdentityCfg.CacheTTL,
		CacheEnabled: apiIdentityCfg.CacheEnabled,
		MaxBulkSize:  apiIdentityCfg.MaxBulkSize,
	})

	if b.logger != nil {
		b.logger.Info("✅ AuthClient initialized successfully",
			"base_url", apiIdentityCfg.BaseURL,
			"cache_enabled", apiIdentityCfg.CacheEnabled,
		)
	}

	return b
}

// WithM2MClients configura los clientes máquina-a-máquina del worker (plan 039
// F4, D-039.7): el provider de service JWT (HS256) + el cliente de lectura de
// settings hacia academic (con caché TTL corta) + el stub de learning (lo llena
// el plan 040). No hace llamadas de red: solo construye. Un Secret vacío no
// impide el arranque (academic rechazará el token si se usa sin secret).
func (b *ResourceBuilder) WithM2MClients() *ResourceBuilder {
	if b.err != nil {
		return b
	}
	if b.logger == nil {
		b.err = fmt.Errorf("logger required before M2M clients")
		return b
	}

	jwtCfg := b.config.GetServiceJWTConfigWithDefaults()
	academicCfg := b.config.GetAPIAcademicConfigWithDefaults()
	learningCfg := b.config.GetAPILearningConfigWithDefaults()

	// Token provider hacia academic (audience/scope de settings).
	academicToken, err := m2m.NewServiceTokenProvider(m2m.ServiceTokenConfig{
		Secret:   jwtCfg.Secret,
		Issuer:   jwtCfg.Issuer,
		Audience: jwtCfg.Audience,
		ClientID: jwtCfg.ClientID,
		Scopes:   []string{"schools.settings.read"},
		TTL:      jwtCfg.TTL,
	})
	if err != nil {
		b.err = fmt.Errorf("failed to create academic service token provider: %w", err)
		return b
	}

	// Token provider hacia learning: distinta audience (edugo-api-learning) y scope
	// (attempts.review). El audience va en el token, así que necesita su propia
	// instancia; comparte secret/issuer/clientID/TTL con el de academic.
	learningToken, err := m2m.NewServiceTokenProvider(m2m.ServiceTokenConfig{
		Secret:   jwtCfg.Secret,
		Issuer:   jwtCfg.Issuer,
		Audience: audienceLearning,
		ClientID: jwtCfg.ClientID,
		Scopes:   []string{scopeAttemptsReview},
		TTL:      jwtCfg.TTL,
	})
	if err != nil {
		b.err = fmt.Errorf("failed to create learning service token provider: %w", err)
		return b
	}

	b.settingsClient = m2m.NewSettingsClient(m2m.SettingsClientConfig{
		BaseURL:       academicCfg.BaseURL,
		Timeout:       academicCfg.Timeout,
		CacheTTL:      academicCfg.CacheTTL,
		TokenProvider: academicToken,
	})

	b.learningClient = m2m.NewLearningClient(m2m.LearningClientConfig{
		BaseURL:       learningCfg.BaseURL,
		Timeout:       learningCfg.Timeout,
		TokenProvider: learningToken,
	})

	b.logger.Info("✅ M2M clients initialized",
		"academic_base_url", academicCfg.BaseURL,
		"academic_audience", jwtCfg.Audience,
		"learning_base_url", learningCfg.BaseURL,
		"learning_audience", audienceLearning,
		"secret_present", jwtCfg.Secret != "",
	)
	return b
}

// Contrato M2M del carril de revisión (plan 040 F2): el service JWT hacia learning
// lleva esta audience y scope.
const (
	audienceLearning    = "edugo-api-learning"
	scopeAttemptsReview = "attempts.review"
)

// WithLLMProvider construye el provider LLM según la política de plataforma
// (plan 039 D-039.3/D-039.4). En 039 el worker no dispara generación/corrección
// (eso es 040/041): se elige un provider por defecto (local Ollama) listo para
// que los planes siguientes lo seleccionen por carril/escuela vía M2M. La config
// se inyecta al constructor; el provider NUNCA lee env.
func (b *ResourceBuilder) WithLLMProvider() *ResourceBuilder {
	if b.err != nil {
		return b
	}
	if b.logger == nil {
		b.err = fmt.Errorf("logger required before LLM provider")
		return b
	}

	llmCfg := b.config.GetLLMConfigWithDefaults()

	// Provider local (Ollama). Es también el default histórico expuesto en
	// Resources.LLMProvider.
	localProvider := ollama.New(ollama.Config{
		BaseURL: llmCfg.Local.BaseURL,
		Model:   llmCfg.Local.Model,
		Timeout: llmCfg.Local.Timeout,
	})
	b.llmProvider = localProvider

	// Mapa de providers por mode del carril de revisión (plan 040 F2): el processor
	// selecciona "local" u "api" según la política de la escuela. El provider por API
	// se construye best-effort: si la config no permite construirlo (proveedor no
	// soportado), se omite la clave "api" y el processor errará claro solo si una
	// escuela pide mode=api sin provider disponible —sin romper el carril local—.
	b.llmProviders = map[string]llm.LLMProvider{"local": localProvider}
	if apiProvider, err := BuildAPIProvider(llmCfg.API); err != nil {
		b.logger.Warn("provider LLM por API no disponible (mode=api fallará hasta corregir config)",
			"error", err.Error(), "api_provider", llmCfg.API.Provider)
	} else {
		b.llmProviders["api"] = apiProvider
	}

	b.logger.Info("✅ LLM providers initialized (selección por mode de escuela: local|api)",
		"local_provider", localProvider.Name(),
		"local_base_url", llmCfg.Local.BaseURL,
		"api_provider", llmCfg.API.Provider,
		"api_available", b.llmProviders["api"] != nil,
	)
	return b
}

// buildAPIProvider construye el provider por API a demanda (plan 040/041 lo usa
// cuando la política de una escuela pide modo "api"). Se expone para no atar el
// import del paquete api solo al harness. Devuelve error si la config no permite
// construirlo (proveedor no soportado, etc.).
func BuildAPIProvider(cfg config.LLMAPIConfig) (llm.LLMProvider, error) {
	return llmapi.New(llmapi.Config{
		Provider:  cfg.Provider,
		APIKey:    cfg.APIKey,
		Model:     cfg.Model,
		BaseURL:   cfg.BaseURL,
		Timeout:   cfg.Timeout,
		MaxTokens: cfg.MaxTokens,
	})
}

// WithInfrastructure configura los servicios de infraestructura externa (PDF, NLP).
//
// El storage (S3/MinIO) se retiró del bootstrap (bug 0040): ningún processor lo
// consume tras la dieta del plan 037 y su validación de bucket (HeadBucket) mataba
// el arranque sin MinIO local. El worker es orquestador M2M (D-037/D-040.8); si un
// carril futuro necesita acceso directo a storage, el plan 041 lo reintroduce.
func (b *ResourceBuilder) WithInfrastructure() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	if b.logger == nil {
		b.err = fmt.Errorf("logger required before infrastructure")
		return b
	}

	b.logger.Info("initializing infrastructure services")

	// Crear factory
	factory := infrastructure.NewFactory(*b.config, b.logger)

	// Crear extractor PDF
	pdfExtractor, err := factory.CreatePDFExtractor()
	if err != nil {
		b.err = fmt.Errorf("failed to create PDF extractor: %w", err)
		return b
	}
	b.pdfExtractor = pdfExtractor

	// Crear cliente NLP
	nlpClient, err := factory.CreateNLPClient()
	if err != nil {
		b.err = fmt.Errorf("failed to create NLP client: %w", err)
		return b
	}
	b.nlpClient = nlpClient

	b.logger.Info("✅ Infrastructure services initialized successfully")
	return b
}

// WithProcessors crea el registry de processors y registra los del carril LLM.
//
// Plan 040 (F1): el registry deja de estar vacío. Se registra el primer processor
// post-dieta —AttemptReviewProcessor (event_type attempt.review_requested)— que lee
// la política de la escuela vía SettingsClient y corta-circuito si la revisión está
// apagada. Requiere que WithM2MClients haya corrido antes (settingsClient listo).
func (b *ResourceBuilder) WithProcessors() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	if b.logger == nil {
		b.err = fmt.Errorf("logger required before processors")
		return b
	}
	if b.settingsClient == nil {
		b.err = fmt.Errorf("settings client required before processors (call WithM2MClients first)")
		return b
	}
	if b.learningClient == nil {
		b.err = fmt.Errorf("learning client required before processors (call WithM2MClients first)")
		return b
	}
	if len(b.llmProviders) == 0 {
		b.err = fmt.Errorf("LLM providers required before processors (call WithLLMProvider first)")
		return b
	}

	b.processorRegistry = processor.NewRegistry(b.logger)
	b.processorRegistry.Register(processor.NewAttemptReviewProcessor(
		b.settingsClient, b.learningClient, b.llmProviders, b.logger))

	b.logger.Info("✅ Processor registry initialized (carril revisión LLM, plan 040)",
		"count", b.processorRegistry.Count())
	return b
}

// WithHealthChecks configura los health checks para las dependencias
func (b *ResourceBuilder) WithHealthChecks() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	if b.logger == nil {
		b.err = fmt.Errorf("logger required before health checks")
		return b
	}

	b.logger.Info("initializing health checks")

	// Crear checker
	checker := health.NewChecker()

	// Obtener configuración de health checks con valores por defecto
	healthConfig := b.config.GetHealthConfigWithDefaults()

	// Registrar health check de RabbitMQ si está disponible
	if b.rabbitChannel != nil {
		rabbitCheck := health.NewRabbitMQCheck(b.rabbitChannel, healthConfig.Timeouts.RabbitMQ)
		checker.Register(rabbitCheck)
		b.logger.Info("registered RabbitMQ health check")
	}

	// Guardar referencia
	b.healthChecker = checker

	b.logger.Info("✅ Health checks initialized successfully")
	return b
}

// WithMetricsServer configura el servidor HTTP de métricas Prometheus
func (b *ResourceBuilder) WithMetricsServer() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	if b.logger == nil {
		b.err = fmt.Errorf("logger required before metrics server")
		return b
	}

	// Obtener configuración con valores por defecto
	metricsCfg := b.config.GetMetricsConfigWithDefaults()

	// Si las métricas no están habilitadas, retornar sin hacer nada
	if !metricsCfg.Enabled {
		b.logger.Info("metrics server disabled")
		return b
	}

	b.logger.Info("initializing metrics server", "port", metricsCfg.Port)

	// Crear servidor de métricas con health checker si está disponible
	var metricsServer *httpInfra.MetricsServer
	if b.healthChecker != nil {
		metricsServer = httpInfra.NewMetricsServerWithConfig(httpInfra.MetricsServerConfig{
			Port:          metricsCfg.Port,
			HealthChecker: b.healthChecker,
		})
		b.logger.Info("metrics server configured with health endpoints")
	} else {
		metricsServer = httpInfra.NewMetricsServer(metricsCfg.Port)
	}

	// Iniciar servidor en goroutine
	go func() {
		b.logger.Info("starting metrics server", "port", metricsCfg.Port)
		if err := metricsServer.Start(); err != nil {
			b.logger.Error("metrics server error", "error", err.Error())
		}
	}()

	// Guardar referencia
	b.metricsServer = metricsServer

	// Registrar cleanup
	b.addCleanup(func() error {
		b.logger.Info("shutting down metrics server")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return b.metricsServer.Shutdown(ctx)
	})

	b.logger.Info("✅ Metrics server initialized successfully", "endpoint", fmt.Sprintf("http://localhost:%d/metrics", metricsCfg.Port))
	return b
}

// Build construye y retorna Resources con su función de cleanup
func (b *ResourceBuilder) Build() (*Resources, func() error, error) {
	// Verificar si hubo errores durante la construcción
	if b.err != nil {
		// Ejecutar cleanup de recursos parcialmente inicializados
		if cleanupErr := b.cleanup(); cleanupErr != nil {
			// Log cleanup error pero retornar el error original
			return nil, nil, fmt.Errorf("%w (cleanup also failed: %v)", b.err, cleanupErr)
		}
		return nil, nil, b.err
	}

	// Verificar que todos los recursos requeridos están inicializados
	if b.logger == nil {
		return nil, nil, fmt.Errorf("logger is required")
	}

	// Crear lifecycle manager con logger
	b.lifecycleManager = lifecycle.NewManager(b.logger)

	// Construir Resources
	resources := &Resources{
		Logger:            b.logger,
		RabbitMQConn:      b.rabbitSharedConn,
		RabbitMQChannel:   b.rabbitChannel,
		AuthClient:        b.authClient,
		SettingsClient:    b.settingsClient,
		LearningClient:    b.learningClient,
		LLMProvider:       b.llmProvider,
		LifecycleManager:  b.lifecycleManager,
		ProcessorRegistry: b.processorRegistry,
		MetricsServer:     b.metricsServer,
		HealthChecker:     b.healthChecker,
		SharedMetrics:     b.sharedMetrics,
	}

	// Crear función de cleanup
	cleanup := func() error {
		return b.cleanup()
	}

	b.logger.Info("✅ All resources initialized successfully")
	return resources, cleanup, nil
}

// addCleanup registra una función de cleanup
// Los cleanups se agregan al inicio para ejecutar en orden inverso (LIFO)
func (b *ResourceBuilder) addCleanup(fn func() error) {
	b.cleanupFuncs = append([]func() error{fn}, b.cleanupFuncs...)
}

// cleanup ejecuta todas las funciones de cleanup en orden LIFO
func (b *ResourceBuilder) cleanup() error {
	if b.logger != nil {
		b.logger.Info("starting resource cleanup")
	}

	var errors []error

	// Ejecutar cleanups en orden (LIFO - último creado, primero cerrado)
	for i, cleanupFn := range b.cleanupFuncs {
		if err := cleanupFn(); err != nil {
			errors = append(errors, fmt.Errorf("cleanup %d failed: %w", i, err))
		}
	}

	if len(errors) > 0 {
		errMsg := fmt.Sprintf("cleanup had %d errors", len(errors))
		if b.logger != nil {
			for _, err := range errors {
				b.logger.Error("cleanup error", "error", err.Error())
			}
			b.logger.Error(errMsg)
		}
		return fmt.Errorf("%s: %v", errMsg, errors)
	}

	if b.logger != nil {
		b.logger.Info("✅ Resource cleanup completed successfully")
	}

	return nil
}
