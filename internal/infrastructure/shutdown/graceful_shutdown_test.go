package shutdown

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLogger para testing
type MockLogger struct {
	mu       sync.Mutex
	messages []string
}

func (m *MockLogger) Info(msg string, keysAndValues ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
}

func (m *MockLogger) Warn(msg string, keysAndValues ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
}

func (m *MockLogger) Error(msg string, keysAndValues ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
}

func (m *MockLogger) GetMessages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.messages))
	copy(result, m.messages)
	return result
}

func TestNewGracefulShutdown(t *testing.T) {
	t.Run("con timeout válido", func(t *testing.T) {
		logger := &MockLogger{}
		timeout := 5 * time.Second

		gs := NewGracefulShutdown(timeout, logger)

		assert.NotNil(t, gs)
		assert.Equal(t, timeout, gs.timeout)
		assert.Equal(t, logger, gs.logger)
		assert.Equal(t, 0, gs.TaskCount())
	})

	t.Run("con timeout cero usa valor por defecto", func(t *testing.T) {
		logger := &MockLogger{}

		gs := NewGracefulShutdown(0, logger)

		assert.NotNil(t, gs)
		assert.Equal(t, 30*time.Second, gs.timeout)
	})

	t.Run("con timeout negativo usa valor por defecto", func(t *testing.T) {
		logger := &MockLogger{}

		gs := NewGracefulShutdown(-1*time.Second, logger)

		assert.NotNil(t, gs)
		assert.Equal(t, 30*time.Second, gs.timeout)
	})

	t.Run("sin logger", func(t *testing.T) {
		gs := NewGracefulShutdown(5*time.Second, nil)

		assert.NotNil(t, gs)
		assert.Nil(t, gs.logger)
	})
}

func TestRegister(t *testing.T) {
	t.Run("registra una tarea", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(5*time.Second, logger)

		executed := false
		gs.Register("test_task", func(ctx context.Context) error {
			executed = true
			return nil
		})

		assert.Equal(t, 1, gs.TaskCount())

		err := gs.Shutdown(context.Background())
		assert.NoError(t, err)
		assert.True(t, executed)
	})

	t.Run("registra múltiples tareas", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(5*time.Second, logger)

		gs.Register("task1", func(ctx context.Context) error { return nil })
		gs.Register("task2", func(ctx context.Context) error { return nil })
		gs.Register("task3", func(ctx context.Context) error { return nil })

		assert.Equal(t, 3, gs.TaskCount())
	})

	t.Run("es thread-safe", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(5*time.Second, logger)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				gs.Register("task", func(ctx context.Context) error { return nil })
			}(i)
		}

		wg.Wait()
		assert.Equal(t, 100, gs.TaskCount())
	})
}

func TestShutdown(t *testing.T) {
	t.Run("ejecuta tareas en orden LIFO", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(5*time.Second, logger)

		var executionOrder []string
		var mu sync.Mutex

		gs.Register("first", func(ctx context.Context) error {
			mu.Lock()
			executionOrder = append(executionOrder, "first")
			mu.Unlock()
			return nil
		})

		gs.Register("second", func(ctx context.Context) error {
			mu.Lock()
			executionOrder = append(executionOrder, "second")
			mu.Unlock()
			return nil
		})

		gs.Register("third", func(ctx context.Context) error {
			mu.Lock()
			executionOrder = append(executionOrder, "third")
			mu.Unlock()
			return nil
		})

		err := gs.Shutdown(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, []string{"third", "second", "first"}, executionOrder)
	})

	t.Run("maneja errores en tareas", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(5*time.Second, logger)

		expectedErr := errors.New("task error")

		gs.Register("task_ok", func(ctx context.Context) error { return nil })
		gs.Register("task_error", func(ctx context.Context) error { return expectedErr })
		gs.Register("task_ok_2", func(ctx context.Context) error { return nil })

		err := gs.Shutdown(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task_error")
	})

	t.Run("continúa ejecutando tareas después de error", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(5*time.Second, logger)

		executed := make(map[string]bool)
		var mu sync.Mutex

		gs.Register("task1", func(ctx context.Context) error {
			mu.Lock()
			executed["task1"] = true
			mu.Unlock()
			return nil
		})

		gs.Register("task2", func(ctx context.Context) error {
			mu.Lock()
			executed["task2"] = true
			mu.Unlock()
			return errors.New("error")
		})

		gs.Register("task3", func(ctx context.Context) error {
			mu.Lock()
			executed["task3"] = true
			mu.Unlock()
			return nil
		})

		err := gs.Shutdown(context.Background())
		assert.Error(t, err)

		// Todas las tareas deben ejecutarse
		assert.True(t, executed["task1"])
		assert.True(t, executed["task2"])
		assert.True(t, executed["task3"])
	})

	t.Run("respeta timeout", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(100*time.Millisecond, logger)

		taskStarted := make(chan struct{})
		taskCompleted := false

		gs.Register("slow_task", func(ctx context.Context) error {
			close(taskStarted)
			select {
			case <-time.After(1 * time.Second):
				taskCompleted = true
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		start := time.Now()
		err := gs.Shutdown(context.Background())
		elapsed := time.Since(start)

		// Esperar a que la tarea inicie
		<-taskStarted

		// Debe completar antes del timeout de la tarea lenta
		assert.Less(t, elapsed, 500*time.Millisecond)
		assert.Error(t, err)
		assert.False(t, taskCompleted)
	})

	t.Run("sin tareas registradas", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(5*time.Second, logger)

		err := gs.Shutdown(context.Background())
		assert.NoError(t, err)

		messages := logger.GetMessages()
		assert.Contains(t, messages, "No hay tareas de shutdown registradas")
	})

	t.Run("con contexto padre cancelado hereda cancelación", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(5*time.Second, logger)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancelar inmediatamente

		executed := false
		gs.Register("task", func(taskCtx context.Context) error {
			executed = true
			// El contexto interno hereda la cancelación del padre
			select {
			case <-taskCtx.Done():
				return taskCtx.Err()
			case <-time.After(10 * time.Millisecond):
				return nil
			}
		})

		err := gs.Shutdown(ctx)
		// Debe retornar error porque el contexto está cancelado
		assert.Error(t, err)
		assert.True(t, executed)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("sin logger", func(t *testing.T) {
		gs := NewGracefulShutdown(5*time.Second, nil)

		executed := false
		gs.Register("task", func(ctx context.Context) error {
			executed = true
			return nil
		})

		err := gs.Shutdown(context.Background())
		assert.NoError(t, err)
		assert.True(t, executed)
	})
}

func TestTaskCount(t *testing.T) {
	t.Run("cuenta inicial es cero", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(5*time.Second, logger)

		assert.Equal(t, 0, gs.TaskCount())
	})

	t.Run("incrementa con cada registro", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(5*time.Second, logger)

		for i := 1; i <= 5; i++ {
			gs.Register("task", func(ctx context.Context) error { return nil })
			assert.Equal(t, i, gs.TaskCount())
		}
	})

	t.Run("no cambia después de shutdown", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(5*time.Second, logger)

		gs.Register("task1", func(ctx context.Context) error { return nil })
		gs.Register("task2", func(ctx context.Context) error { return nil })

		countBefore := gs.TaskCount()
		err := gs.Shutdown(context.Background())
		countAfter := gs.TaskCount()

		assert.NoError(t, err)
		assert.Equal(t, countBefore, countAfter)
		assert.Equal(t, 2, countAfter)
	})
}

func TestShutdownConcurrency(t *testing.T) {
	t.Run("ejecuta tareas secuencialmente", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(10*time.Second, logger)

		var concurrent int
		var maxConcurrent int
		var mu sync.Mutex

		for i := 0; i < 10; i++ {
			gs.Register("task", func(ctx context.Context) error {
				mu.Lock()
				concurrent++
				if concurrent > maxConcurrent {
					maxConcurrent = concurrent
				}
				mu.Unlock()

				time.Sleep(10 * time.Millisecond)

				mu.Lock()
				concurrent--
				mu.Unlock()

				return nil
			})
		}

		err := gs.Shutdown(context.Background())
		assert.NoError(t, err)

		// Las tareas deben ejecutarse secuencialmente
		assert.Equal(t, 1, maxConcurrent)
	})
}

func TestShutdownRealWorldScenario(t *testing.T) {
	t.Run("simula shutdown completo de aplicación", func(t *testing.T) {
		logger := &MockLogger{}
		gs := NewGracefulShutdown(5*time.Second, logger)

		// Simular componentes de aplicación
		var (
			consumerStopped bool
			metricsShutdown bool
			rabbitMQClosed  bool
			mongoDBClosed   bool
			postgresClosed  bool
			mu              sync.Mutex
		)

		// Orden de registro (primero en inicializarse)
		gs.Register("postgres", func(ctx context.Context) error {
			mu.Lock()
			postgresClosed = true
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		gs.Register("mongodb", func(ctx context.Context) error {
			mu.Lock()
			mongoDBClosed = true
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		gs.Register("rabbitmq", func(ctx context.Context) error {
			mu.Lock()
			rabbitMQClosed = true
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		gs.Register("metrics", func(ctx context.Context) error {
			mu.Lock()
			metricsShutdown = true
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		gs.Register("consumer", func(ctx context.Context) error {
			mu.Lock()
			consumerStopped = true
			mu.Unlock()
			time.Sleep(50 * time.Millisecond) // Esperar mensajes en proceso
			return nil
		})

		// Ejecutar shutdown
		start := time.Now()
		err := gs.Shutdown(context.Background())
		elapsed := time.Since(start)

		require.NoError(t, err)

		// Verificar que todos se cerraron
		assert.True(t, consumerStopped)
		assert.True(t, metricsShutdown)
		assert.True(t, rabbitMQClosed)
		assert.True(t, mongoDBClosed)
		assert.True(t, postgresClosed)

		// Debe tardar al menos el tiempo de todas las operaciones
		assert.GreaterOrEqual(t, elapsed, 90*time.Millisecond)

		// Verificar logs
		messages := logger.GetMessages()
		assert.Contains(t, messages, "Ejecutando tarea de shutdown")
		assert.Contains(t, messages, "Tarea de shutdown completada")
	})
}
