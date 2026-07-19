package config

import (
	"fmt"
	"os"

	sharedconfig "github.com/EduGoGroup/edugo-shared/config"
)

func Load() (*Config, error) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}

	var cfg Config

	loader := sharedconfig.NewLoader(
		sharedconfig.WithEnvPrefix("EDUGO_WORKER"),
		sharedconfig.WithConfigPath("./config"),
		sharedconfig.WithConfigPath("../config"),
		sharedconfig.WithEnvironmentOverride(env),
		sharedconfig.WithDefaults(map[string]interface{}{
			"logging.version": "dev",
		}),
		sharedconfig.WithExplicitBindings(map[string]string{
			"messaging.rabbitmq.url": "RABBITMQ_URL",
			"nlp.api_key":            "OPENAI_API_KEY",
			// M2M (plan 039 F4): service JWT + base URLs de dominio.
			"service_jwt.secret":    "SERVICE_JWT_SECRET",
			"api_academic.base_url": "API_ACADEMIC_BASE_URL",
			"api_learning.base_url": "API_LEARNING_BASE_URL",
			// Carril de materiales (plan 043 F3c): cola del riel material→evaluación. El
			// broker real es CloudAMQP (staging); este override permite fijar el nombre por
			// entorno sin tocar el YAML.
			"messaging.rabbitmq.queues.material_assessment_requested": "MATERIAL_ASSESSMENT_REQUESTED_QUEUE",
			// Carril de materiales (plan 043 F2): descarga + porcionado determinista.
			"material_pipeline.download_max_bytes":          "MATERIAL_PIPELINE_DOWNLOAD_MAX_BYTES",
			"material_pipeline.chunk_target_words":          "MATERIAL_PIPELINE_CHUNK_TARGET_WORDS",
			"material_pipeline.chunk_max_words":             "MATERIAL_PIPELINE_CHUNK_MAX_WORDS",
			"material_pipeline.chunk_min_words":             "MATERIAL_PIPELINE_CHUNK_MIN_WORDS",
			"material_pipeline.chunk_merge_threshold_words": "MATERIAL_PIPELINE_CHUNK_MERGE_THRESHOLD_WORDS",
			// Umbrales de dedupe (plan 044 D-044.2): coseno de embeddings; calibrados en F1b.
			"material_pipeline.dedupe_high": "MATERIAL_PIPELINE_DEDUPE_HIGH",
			"material_pipeline.dedupe_low":  "MATERIAL_PIPELINE_DEDUPE_LOW",
			// Reduce fase 2 (plan 044 D-044.3/D-044.4): umbral de relevancia, modo del paso
			// (local|api) y candado verbatim local_only.
			"material_pipeline.relevance_min":       "MATERIAL_PIPELINE_RELEVANCE_MIN",
			"material_pipeline.relevance_mode":      "MATERIAL_PIPELINE_RELEVANCE_MODE",
			"material_pipeline.verbatim_max_words":  "MATERIAL_PIPELINE_VERBATIM_MAX_WORDS",
			"material_pipeline.relevance_max_ideas": "MATERIAL_PIPELINE_RELEVANCE_MAX_IDEAS",
			// Selección final (plan 044 D-044.5): cupo de preguntas cuando el job no expone
			// target_questions por M2M.
			"material_pipeline.target_questions_default": "MATERIAL_PIPELINE_TARGET_QUESTIONS_DEFAULT",
			// LLM (plan 039 D-039.3): credenciales/URL/modelo de EduGo, no por escuela.
			"llm.local.base_url": "LLM_LOCAL_BASE_URL",
			"llm.local.model":    "LLM_LOCAL_MODEL",
			"llm.api.provider":   "LLM_API_PROVIDER",
			"llm.api.api_key":    "LLM_API_KEY",
			"llm.api.model":      "LLM_API_MODEL",
			// Embeddings local (plan 044 D-044.1): host/modelo/timeout del cliente Ollama
			// de embeddings, separado del provider LLM.
			"llm.embed.base_url": "LLM_EMBED_BASE_URL",
			"llm.embed.model":    "LLM_EMBED_MODEL",
			"llm.embed.timeout":  "LLM_EMBED_TIMEOUT",
		}),
	)

	if err := loader.Load(&cfg); err != nil {
		return nil, fmt.Errorf("error loading worker config: %w", err)
	}

	// Derive logging.env from APP_ENV so logs are labeled with the correct environment
	if cfg.Logging.Env == "" {
		cfg.Logging.Env = env
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
