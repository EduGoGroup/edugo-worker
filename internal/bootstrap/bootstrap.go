package bootstrap

import (
	"context"

	"github.com/EduGoGroup/edugo-worker/internal/config"
)

// Initialize inicializa la infraestructura usando shared/bootstrap
func Initialize(ctx context.Context, cfg *config.Config) (*Resources, func() error, error) {
	return bridgeToSharedBootstrap(ctx, cfg)
}
