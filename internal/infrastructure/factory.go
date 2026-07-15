package infrastructure

import (
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/config"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp/fallback"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
)

// Factory crea instancias de servicios de infraestructura
// Centraliza la creación de clientes PDF, NLP, etc.
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

// CreatePDFExtractor crea un extractor de PDF según configuración
func (f *Factory) CreatePDFExtractor() (pdf.Extractor, error) {
	f.logger.Info("creando extractor PDF")
	return pdf.NewExtractor(f.logger), nil
}

// CreateNLPClient crea el cliente NLP del worker.
// Hoy el ecosistema solo tiene el SmartFallback (NLP heurístico sin proveedor
// externo); un proveedor real de NLP es trabajo futuro (F2+).
func (f *Factory) CreateNLPClient() (nlp.Client, error) {
	f.logger.Info("creando cliente NLP", "provider", "smart_fallback")
	return fallback.NewSmartClient(f.logger), nil
}
