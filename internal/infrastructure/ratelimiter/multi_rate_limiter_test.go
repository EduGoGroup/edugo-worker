package ratelimiter

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMulti(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 10, BurstSize: 20},
		"event2": {RequestsPerSecond: 5, BurstSize: 10},
	}

	defaultCfg := &Config{RequestsPerSecond: 1, BurstSize: 2}

	ml := NewMulti(configs, defaultCfg)

	assert.NotNil(t, ml)
	assert.Len(t, ml.limiters, 2)
	assert.NotNil(t, ml.defaultConfig)

	// Verificar que se crearon los limiters
	assert.True(t, ml.HasLimiter("event1"))
	assert.True(t, ml.HasLimiter("event2"))
	assert.False(t, ml.HasLimiter("event3"))
}

func TestNewMulti_NoDefault(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 10, BurstSize: 20},
	}

	ml := NewMulti(configs, nil)

	assert.NotNil(t, ml)
	assert.Nil(t, ml.defaultConfig)
	assert.True(t, ml.HasLimiter("event1"))
}

func TestNewMulti_EmptyConfigs(t *testing.T) {
	ml := NewMulti(map[string]Config{}, nil)

	assert.NotNil(t, ml)
	assert.Empty(t, ml.limiters)
}

func TestAllow_ConfiguredEvent(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 10, BurstSize: 5},
	}

	ml := NewMulti(configs, nil)

	// Debe permitir 5 requests (burst size)
	for i := 0; i < 5; i++ {
		assert.True(t, ml.Allow("event1"), "debe permitir request %d", i+1)
	}

	// El 6º debe fallar
	assert.False(t, ml.Allow("event1"))
}

func TestAllow_UnconfiguredEvent_WithDefault(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 10, BurstSize: 5},
	}
	defaultCfg := &Config{RequestsPerSecond: 20, BurstSize: 10}

	ml := NewMulti(configs, defaultCfg)

	// event2 no está configurado, debe usar default
	for i := 0; i < 10; i++ {
		assert.True(t, ml.Allow("event2"), "debe permitir request %d usando default", i+1)
	}

	// El 11º debe fallar (burst size del default es 10)
	assert.False(t, ml.Allow("event2"))

	// Verificar que se creó el limiter dinámicamente
	assert.True(t, ml.HasLimiter("event2"))
}

func TestAllow_UnconfiguredEvent_NoDefault(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 10, BurstSize: 5},
	}

	ml := NewMulti(configs, nil)

	// event2 no está configurado y no hay default, debe permitir siempre
	for i := 0; i < 100; i++ {
		assert.True(t, ml.Allow("event2"), "debe permitir request %d sin límite", i+1)
	}

	// No debe crear limiter para eventos no configurados sin default
	assert.False(t, ml.HasLimiter("event2"))
}

func TestAllow_DifferentEventsIndependent(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 10, BurstSize: 5},
		"event2": {RequestsPerSecond: 10, BurstSize: 10},
	}

	ml := NewMulti(configs, nil)

	// Consumir todos los tokens de event1
	for i := 0; i < 5; i++ {
		require.True(t, ml.Allow("event1"))
	}
	assert.False(t, ml.Allow("event1"))

	// event2 debe seguir teniendo tokens
	assert.True(t, ml.Allow("event2"))
}

func TestWait_ConfiguredEvent(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 50, BurstSize: 5},
	}

	ml := NewMulti(configs, nil)

	// Consumir todos los tokens
	for i := 0; i < 5; i++ {
		require.True(t, ml.Allow("event1"))
	}

	ctx := context.Background()

	// Wait debe esperar y completar
	err := ml.Wait(ctx, "event1")
	assert.NoError(t, err)

	// Debe haber consumido el token
	assert.False(t, ml.Allow("event1"))
}

func TestWait_UnconfiguredEvent_WithDefault(t *testing.T) {
	defaultCfg := &Config{RequestsPerSecond: 50, BurstSize: 3}
	ml := NewMulti(map[string]Config{}, defaultCfg)

	// Consumir tokens del evento no configurado
	for i := 0; i < 3; i++ {
		require.True(t, ml.Allow("event_new"))
	}

	ctx := context.Background()

	// Wait debe crear el limiter y esperar
	err := ml.Wait(ctx, "event_new")
	assert.NoError(t, err)

	// Verificar que se creó el limiter
	assert.True(t, ml.HasLimiter("event_new"))
}

func TestWait_UnconfiguredEvent_NoDefault(t *testing.T) {
	ml := NewMulti(map[string]Config{}, nil)

	ctx := context.Background()

	// Wait debe retornar inmediatamente sin error
	err := ml.Wait(ctx, "event_unknown")
	assert.NoError(t, err)

	// No debe crear limiter
	assert.False(t, ml.HasLimiter("event_unknown"))
}

func TestMultiWait_ContextCanceled(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 0.1, BurstSize: 1},
	}

	ml := NewMulti(configs, nil)

	// Consumir el único token
	require.True(t, ml.Allow("event1"))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Wait debe fallar por timeout
	err := ml.Wait(ctx, "event1")
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestTokens_ConfiguredEvent(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 10, BurstSize: 10},
	}

	ml := NewMulti(configs, nil)

	// Debe tener 10 tokens inicialmente
	assert.Equal(t, 10.0, ml.Tokens("event1"))

	// Consumir 3 tokens
	ml.Allow("event1")
	ml.Allow("event1")
	ml.Allow("event1")

	// Debe tener ~7 tokens
	tokens := ml.Tokens("event1")
	assert.InDelta(t, 7.0, tokens, 0.1)
}

func TestTokens_UnconfiguredEvent_NoDefault(t *testing.T) {
	ml := NewMulti(map[string]Config{}, nil)

	// Debe retornar -1 para eventos no configurados sin default
	assert.Equal(t, -1.0, ml.Tokens("event_unknown"))
}

func TestTokens_UnconfiguredEvent_WithDefault(t *testing.T) {
	defaultCfg := &Config{RequestsPerSecond: 10, BurstSize: 15}
	ml := NewMulti(map[string]Config{}, defaultCfg)

	// Primera llamada debe crear el limiter con default
	tokens := ml.Tokens("event_new")
	assert.Equal(t, 15.0, tokens)

	// Verificar que se creó
	assert.True(t, ml.HasLimiter("event_new"))
}

func TestReset_ConfiguredEvent(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 10, BurstSize: 10},
	}

	ml := NewMulti(configs, nil)

	// Consumir tokens
	for i := 0; i < 10; i++ {
		require.True(t, ml.Allow("event1"))
	}

	assert.False(t, ml.Allow("event1"))

	// Reset
	ml.Reset("event1")

	// Debe tener tokens de nuevo
	assert.True(t, ml.Allow("event1"))
	assert.InDelta(t, 9.0, ml.Tokens("event1"), 0.01)
}

func TestReset_UnconfiguredEvent(t *testing.T) {
	ml := NewMulti(map[string]Config{}, nil)

	// Reset en evento no configurado no debe causar pánico
	assert.NotPanics(t, func() {
		ml.Reset("event_unknown")
	})
}

func TestResetAll(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 10, BurstSize: 5},
		"event2": {RequestsPerSecond: 10, BurstSize: 5},
	}

	ml := NewMulti(configs, nil)

	// Consumir tokens de ambos
	for i := 0; i < 5; i++ {
		require.True(t, ml.Allow("event1"))
		require.True(t, ml.Allow("event2"))
	}

	assert.False(t, ml.Allow("event1"))
	assert.False(t, ml.Allow("event2"))

	// Reset all
	ml.ResetAll()

	// Ambos deben tener tokens
	assert.True(t, ml.Allow("event1"))
	assert.True(t, ml.Allow("event2"))
}

func TestEventTypes(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 10, BurstSize: 5},
		"event2": {RequestsPerSecond: 10, BurstSize: 5},
		"event3": {RequestsPerSecond: 10, BurstSize: 5},
	}

	ml := NewMulti(configs, nil)

	types := ml.EventTypes()

	assert.Len(t, types, 3)
	assert.Contains(t, types, "event1")
	assert.Contains(t, types, "event2")
	assert.Contains(t, types, "event3")
}

func TestEventTypes_Empty(t *testing.T) {
	ml := NewMulti(map[string]Config{}, nil)

	types := ml.EventTypes()
	assert.Empty(t, types)
}

func TestHasLimiter(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 10, BurstSize: 5},
	}

	ml := NewMulti(configs, nil)

	assert.True(t, ml.HasLimiter("event1"))
	assert.False(t, ml.HasLimiter("event2"))
}

func TestConcurrency_MultipleEvents(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 100, BurstSize: 50},
		"event2": {RequestsPerSecond: 100, BurstSize: 50},
	}

	ml := NewMulti(configs, nil)

	var wg sync.WaitGroup
	successCount1 := 0
	successCount2 := 0
	var mu sync.Mutex

	// 100 goroutines para cada evento
	numGoroutines := 100

	// Event1
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			if ml.Allow("event1") {
				mu.Lock()
				successCount1++
				mu.Unlock()
			}
		}()
	}

	// Event2
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			if ml.Allow("event2") {
				mu.Lock()
				successCount2++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Cada evento debe haber permitido exactamente 50
	assert.Equal(t, 50, successCount1)
	assert.Equal(t, 50, successCount2)
}

func TestConcurrency_DynamicLimiterCreation(t *testing.T) {
	defaultCfg := &Config{RequestsPerSecond: 100, BurstSize: 20}
	ml := NewMulti(map[string]Config{}, defaultCfg)

	var wg sync.WaitGroup
	numGoroutines := 50

	// Múltiples goroutines intentando crear el mismo limiter
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			ml.Allow("dynamic_event")
		}()
	}

	wg.Wait()

	// Solo debe haberse creado un limiter
	assert.True(t, ml.HasLimiter("dynamic_event"))

	// Verificar que los limiters internos son correctos
	types := ml.EventTypes()
	assert.Len(t, types, 1)
	assert.Contains(t, types, "dynamic_event")
}

func TestMultiConcurrency_Wait(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 200, BurstSize: 10},
	}

	ml := NewMulti(configs, nil)

	var wg sync.WaitGroup
	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	numGoroutines := 50
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			if err := ml.Wait(ctx, "event1"); err == nil {
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

	// Todos deben completar
	assert.Equal(t, numGoroutines, successCount+errorCount)

	// La mayoría debe completar exitosamente
	assert.Greater(t, successCount, numGoroutines/2)
}

func TestConcurrency_MixedOperations(t *testing.T) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 100, BurstSize: 50},
		"event2": {RequestsPerSecond: 100, BurstSize: 50},
	}

	ml := NewMulti(configs, nil)

	var wg sync.WaitGroup
	numGoroutines := 20

	// Allow en event1
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			ml.Allow("event1")
		}()
	}

	// Tokens en event1
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			ml.Tokens("event1")
		}()
	}

	// Reset event2
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			ml.Reset("event2")
		}()
	}

	// EventTypes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			ml.EventTypes()
		}()
	}

	wg.Wait()

	// No debe haber panic ni race conditions
	assert.True(t, ml.HasLimiter("event1"))
	assert.True(t, ml.HasLimiter("event2"))
}

// Benchmark para operaciones básicas
func BenchmarkMultiRateLimiter_Allow(b *testing.B) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 1000000, BurstSize: 1000000},
	}
	ml := NewMulti(configs, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ml.Allow("event1")
	}
}

func BenchmarkMultiRateLimiter_Allow_Parallel(b *testing.B) {
	configs := map[string]Config{
		"event1": {RequestsPerSecond: 1000000, BurstSize: 1000000},
	}
	ml := NewMulti(configs, nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ml.Allow("event1")
		}
	})
}

func BenchmarkMultiRateLimiter_DynamicCreation(b *testing.B) {
	defaultCfg := &Config{RequestsPerSecond: 1000000, BurstSize: 1000000}
	ml := NewMulti(map[string]Config{}, defaultCfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ml.Allow("dynamic_event")
	}
}
