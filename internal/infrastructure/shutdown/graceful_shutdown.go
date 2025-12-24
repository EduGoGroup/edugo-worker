package shutdown

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ShutdownFunc representa una función de limpieza
type ShutdownFunc func(ctx context.Context) error

// ShutdownTask representa una tarea de shutdown con nombre y función
type ShutdownTask struct {
	Name string
	Fn   ShutdownFunc
}

// GracefulShutdown gestiona el apagado ordenado de la aplicación
type GracefulShutdown struct {
	timeout time.Duration
	tasks   []ShutdownTask
	mu      sync.RWMutex
	logger  Logger
}

// Logger es una interfaz simple para logging
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// NewGracefulShutdown crea un nuevo gestor de graceful shutdown
func NewGracefulShutdown(timeout time.Duration, logger Logger) *GracefulShutdown {
	if timeout <= 0 {
		timeout = 30 * time.Second // timeout por defecto
	}

	return &GracefulShutdown{
		timeout: timeout,
		tasks:   make([]ShutdownTask, 0),
		logger:  logger,
	}
}

// Register registra una función de limpieza con un nombre identificativo
// Las funciones se ejecutarán en orden LIFO (último registrado, primero ejecutado)
func (gs *GracefulShutdown) Register(name string, fn ShutdownFunc) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.tasks = append(gs.tasks, ShutdownTask{
		Name: name,
		Fn:   fn,
	})
}

// Shutdown ejecuta todas las funciones de limpieza registradas en orden LIFO
func (gs *GracefulShutdown) Shutdown(ctx context.Context) error {
	gs.mu.RLock()
	tasks := make([]ShutdownTask, len(gs.tasks))
	copy(tasks, gs.tasks)
	gs.mu.RUnlock()

	if len(tasks) == 0 {
		if gs.logger != nil {
			gs.logger.Info("No hay tareas de shutdown registradas")
		}
		return nil
	}

	// Crear contexto con timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, gs.timeout)
	defer cancel()

	// Ejecutar tareas en orden LIFO
	var errors []error
	for i := len(tasks) - 1; i >= 0; i-- {
		task := tasks[i]

		if gs.logger != nil {
			gs.logger.Info("Ejecutando tarea de shutdown", "task", task.Name)
		}

		if err := task.Fn(shutdownCtx); err != nil {
			if gs.logger != nil {
				gs.logger.Error("Error en tarea de shutdown",
					"task", task.Name,
					"error", err.Error())
			}
			errors = append(errors, fmt.Errorf("%s: %w", task.Name, err))
		} else {
			if gs.logger != nil {
				gs.logger.Info("Tarea de shutdown completada", "task", task.Name)
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errores durante shutdown: %v", errors)
	}

	return nil
}

// WaitForSignal espera señales SIGTERM o SIGINT y ejecuta el shutdown
func (gs *GracefulShutdown) WaitForSignal() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	if gs.logger != nil {
		gs.logger.Info("Señal de apagado recibida", "signal", sig.String())
	}

	return gs.Shutdown(context.Background())
}

// TaskCount retorna el número de tareas registradas
func (gs *GracefulShutdown) TaskCount() int {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return len(gs.tasks)
}
