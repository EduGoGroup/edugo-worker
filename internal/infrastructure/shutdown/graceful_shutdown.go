package shutdown

import (
	"time"

	sharedsd "github.com/EduGoGroup/edugo-shared/lifecycle/shutdown"
)

type ShutdownFunc = sharedsd.ShutdownFunc
type ShutdownTask = sharedsd.ShutdownTask
type Logger = sharedsd.Logger
type GracefulShutdown = sharedsd.GracefulShutdown

// NewGracefulShutdown creates a new graceful shutdown manager with the given timeout and logger.
func NewGracefulShutdown(timeout time.Duration, logger Logger) *GracefulShutdown {
	return sharedsd.NewGracefulShutdown(timeout, logger)
}
