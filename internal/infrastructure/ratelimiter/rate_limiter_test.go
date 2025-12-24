package ratelimiter

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name              string
		requestsPerSecond float64
		burstSize         float64
		expectedRate      float64
		expectedMaxTokens float64
	}{
		{
			name:              "valores normales",
			requestsPerSecond: 10,
			burstSize:         20,
			expectedRate:      10,
			expectedMaxTokens: 20,
		},
		{
			name:              "valores cero deben usar defaults",
			requestsPerSecond: 0,
			burstSize:         0,
			expectedRate:      1,
			expectedMaxTokens: 1,
		},
		{
			name:              "valores negativos deben usar defaults",
			requestsPerSecond: -5,
			burstSize:         -10,
			expectedRate:      1,
			expectedMaxTokens: 1,
		},
		{
			name:              "valores decimales",
			requestsPerSecond: 2.5,
			burstSize:         5.5,
			expectedRate:      2.5,
			expectedMaxTokens: 5.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := New(tt.requestsPerSecond, tt.burstSize)

			assert.Equal(t, tt.expectedRate, rl.refillRate)
			assert.Equal(t, tt.expectedMaxTokens, rl.maxTokens)
			assert.Equal(t, tt.expectedMaxTokens, rl.tokens, "debe iniciar con bucket lleno")
		})
	}
}

func TestAllow_BasicConsumption(t *testing.T) {
	// Rate limiter con 10 tokens iniciales
	rl := New(10, 10)

	// Debe permitir 10 requests seguidos
	for i := 0; i < 10; i++ {
		assert.True(t, rl.Allow(), "debe permitir request %d", i+1)
	}

	// El 11º request debe fallar (no hay tokens)
	assert.False(t, rl.Allow(), "debe rechazar request cuando no hay tokens")
}

func TestAllow_Refill(t *testing.T) {
	// Rate limiter: 10 requests/segundo
	rl := New(10, 10)

	// Consumir todos los tokens
	for i := 0; i < 10; i++ {
		require.True(t, rl.Allow())
	}

	// No debe haber tokens
	assert.False(t, rl.Allow())

	// Esperar 100ms (debería recargar ~1 token)
	time.Sleep(100 * time.Millisecond)

	// Ahora debe permitir 1 request
	assert.True(t, rl.Allow(), "debe tener 1 token después de 100ms")

	// No debe permitir otro
	assert.False(t, rl.Allow())
}

func TestAllow_BurstCapacity(t *testing.T) {
	// Rate limiter: 5 req/s, burst de 20
	rl := New(5, 20)

	// Debe permitir 20 requests en ráfaga
	for i := 0; i < 20; i++ {
		assert.True(t, rl.Allow(), "debe permitir burst request %d", i+1)
	}

	// El 21º debe fallar
	assert.False(t, rl.Allow())
}

func TestAllow_RefillDoesNotExceedMax(t *testing.T) {
	// Rate limiter con burst pequeño
	rl := New(100, 5)

	// Consumir 2 tokens
	require.True(t, rl.Allow())
	require.True(t, rl.Allow())

	// Esperar suficiente tiempo para recargar más de 5 tokens
	time.Sleep(200 * time.Millisecond)

	// Debería tener máximo 5 tokens (no 20+)
	count := 0
	for rl.Allow() {
		count++
		if count > 10 {
			t.Fatal("tokens excedieron el máximo esperado")
		}
	}

	assert.Equal(t, 5, count, "debe tener exactamente 5 tokens después de refill")
}

func TestWait_Success(t *testing.T) {
	// Rate limiter: 50 req/s (refill rápido para test)
	rl := New(50, 5)

	// Consumir todos los tokens
	for i := 0; i < 5; i++ {
		require.True(t, rl.Allow())
	}

	ctx := context.Background()

	// Wait debe esperar y retornar sin error
	err := rl.Wait(ctx)
	assert.NoError(t, err)

	// Debe haber consumido el token
	assert.False(t, rl.Allow(), "token debe haber sido consumido por Wait")
}

func TestWait_ContextCanceled(t *testing.T) {
	// Rate limiter muy lento para forzar timeout
	rl := New(0.1, 1)

	// Consumir el único token
	require.True(t, rl.Allow())

	// Contexto con timeout corto
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Wait debe retornar error de contexto
	err := rl.Wait(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestWait_ContextCanceledImmediately(t *testing.T) {
	rl := New(0.1, 1)
	require.True(t, rl.Allow())

	// Contexto ya cancelado
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancelar inmediatamente

	err := rl.Wait(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestTokens(t *testing.T) {
	rl := New(10, 10)

	// Inicialmente debe tener 10 tokens
	assert.Equal(t, 10.0, rl.Tokens())

	// Consumir 3 tokens
	rl.Allow()
	rl.Allow()
	rl.Allow()

	// Debe tener 7 tokens
	tokens := rl.Tokens()
	assert.InDelta(t, 7.0, tokens, 0.1, "debe tener ~7 tokens")

	// Esperar para refill
	time.Sleep(100 * time.Millisecond)

	// Debe tener más tokens (pero no más de 10)
	tokens = rl.Tokens()
	assert.Greater(t, tokens, 7.0, "debe haber recargado tokens")
	assert.LessOrEqual(t, tokens, 10.0, "no debe exceder maxTokens")
}

func TestReset(t *testing.T) {
	rl := New(10, 10)

	// Consumir todos los tokens
	for i := 0; i < 10; i++ {
		require.True(t, rl.Allow())
	}

	// No debe haber tokens
	assert.False(t, rl.Allow())

	// Reset
	rl.Reset()

	// Debe tener todos los tokens de nuevo
	assert.Equal(t, 10.0, rl.Tokens())

	// Debe permitir requests
	assert.True(t, rl.Allow())
}

func TestConcurrency(t *testing.T) {
	// Rate limiter con capacidad conocida
	rl := New(100, 100)

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// 200 goroutines intentando consumir tokens concurrentemente
	numGoroutines := 200
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			if rl.Allow() {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Solo debe haber permitido exactamente 100 (el número de tokens disponibles)
	assert.Equal(t, 100, successCount, "debe permitir exactamente el número de tokens disponibles")
}

func TestConcurrency_Wait(t *testing.T) {
	// Rate limiter con refill rápido
	rl := New(200, 10)

	var wg sync.WaitGroup
	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// 50 goroutines usando Wait
	numGoroutines := 50
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			if err := rl.Wait(ctx); err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			} else {
				mu.Lock()
				errorCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Todos deben haber sido procesados (success o timeout)
	assert.Equal(t, numGoroutines, successCount+errorCount)

	// La mayoría debe haber tenido éxito gracias al refill
	assert.Greater(t, successCount, numGoroutines/2, "la mayoría debe completar exitosamente")
}

func TestRefill_Precision(t *testing.T) {
	// Rate limiter: 10 req/s
	rl := New(10, 10)

	// Consumir todos los tokens
	for i := 0; i < 10; i++ {
		require.True(t, rl.Allow())
	}

	// Esperar exactamente 500ms (debería agregar ~5 tokens)
	time.Sleep(500 * time.Millisecond)

	// Contar tokens recargados
	count := 0
	for rl.Allow() {
		count++
	}

	// Debe haber aproximadamente 5 tokens (±1 por timing)
	assert.InDelta(t, 5, count, 1, "debe recargar ~5 tokens en 500ms")
}

func TestMultipleWaiters(t *testing.T) {
	// Rate limiter con capacidad limitada
	rl := New(20, 5)

	// Consumir todos los tokens
	for i := 0; i < 5; i++ {
		require.True(t, rl.Allow())
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan bool, 10)

	// 10 waiters esperando tokens
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := rl.Wait(ctx)
			results <- (err == nil)
		}()
	}

	// Esperar a que completen
	go func() {
		wg.Wait()
		close(results)
	}()

	// Contar éxitos
	successCount := 0
	timeout := time.After(2 * time.Second)

	for {
		select {
		case success, ok := <-results:
			if !ok {
				// Canal cerrado, todos completaron
				assert.Equal(t, 10, successCount, "todos los waiters deben completar")
				return
			}
			if success {
				successCount++
			}
		case <-timeout:
			t.Fatal("timeout esperando waiters")
		}
	}
}

// Benchmark para medir performance
func BenchmarkAllow(b *testing.B) {
	rl := New(1000000, 1000000) // Capacidad grande para evitar bloqueos

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow()
	}
}

func BenchmarkAllow_Parallel(b *testing.B) {
	rl := New(1000000, 1000000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rl.Allow()
		}
	})
}
