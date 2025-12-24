package infrastructure

import (
	"context"
	"fmt"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/config"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp/fallback"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/storage"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/storage/s3"
)

// Factory crea instancias de servicios de infraestructura
// Centraliza la creación de clientes S3, PDF, NLP, etc.
type Factory struct {
	config config.Config
	logger logger.Logger
}

// NewFactory crea una nueva factory de servicios
func NewFactory(cfg config.Config, logger logger.Logger) *Factory {
	return &Factory{
		config: cfg,
		logger: logger,
	}
}

// CreateStorageClient crea un cliente de almacenamiento según configuración
// Soporta: S3, MinIO, (futuro: filesystem local, Azure Blob, etc.)
func (f *Factory) CreateStorageClient(ctx context.Context) (storage.Client, error) {
	provider := f.config.Storage.Provider
	if provider == "" {
		provider = "s3"
	}

	f.logger.Info("creando cliente de almacenamiento", "provider", provider)

	switch provider {
	case "s3", "minio":
		s3Cfg := f.config.Storage.S3
		client, err := s3.NewClient(
			ctx,
			s3Cfg.Region,
			s3Cfg.Bucket,
			s3Cfg.Endpoint,
			s3Cfg.AccessKeyID,
			s3Cfg.SecretAccessKey,
			s3Cfg.UsePathStyle,
			f.logger,
		)
		if err != nil {
			return nil, fmt.Errorf("error creando cliente S3: %w", err)
		}
		return client, nil

	default:
		return nil, fmt.Errorf("proveedor de almacenamiento no soportado: %s", provider)
	}
}

// CreatePDFExtractor crea un extractor de PDF según configuración
func (f *Factory) CreatePDFExtractor() (pdf.Extractor, error) {
	f.logger.Info("creando extractor PDF")
	return pdf.NewExtractor(f.logger), nil
}

// CreateNLPClient crea un cliente NLP según configuración
// Decide entre OpenAI real o SmartFallback basado en si hay API key
func (f *Factory) CreateNLPClient() (nlp.Client, error) {
	provider := f.config.NLP.Provider
	hasAPIKey := f.config.NLP.APIKey != ""

	f.logger.Info("creando cliente NLP", "provider", provider, "hasAPIKey", hasAPIKey)

	// Si hay API key y el proveedor es openai, usarlo (futuro)
	// Por ahora, siempre usamos SmartFallback hasta tener API key
	if hasAPIKey && provider == "openai" {
		// TODO: Implementar cliente OpenAI real cuando tengamos API key
		// return openai.NewClient(f.config.NLP, f.logger), nil
		f.logger.Warn("cliente OpenAI no implementado aún, usando SmartFallback")
	}

	// Usar SmartFallback por defecto
	f.logger.Info("usando SmartFallback para NLP")
	return fallback.NewSmartClient(f.logger), nil
}
