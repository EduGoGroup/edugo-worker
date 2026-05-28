package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// ============================================
// Test: NewAuthClient
// ============================================

func TestNewAuthClient_DefaultValues(t *testing.T) {
	client := NewAuthClient(AuthClientConfig{
		BaseURL: "http://localhost:8081",
	})

	if client.config.Timeout != 5*time.Second {
		t.Errorf("Expected default timeout 5s, got %v", client.config.Timeout)
	}
	if client.config.CacheTTL != 60*time.Second {
		t.Errorf("Expected default cache TTL 60s, got %v", client.config.CacheTTL)
	}
	if client.config.MaxBulkSize != 50 {
		t.Errorf("Expected default MaxBulkSize 50, got %d", client.config.MaxBulkSize)
	}
}

func TestNewAuthClient_CustomValues(t *testing.T) {
	client := NewAuthClient(AuthClientConfig{
		BaseURL:      "http://localhost:8081",
		Timeout:      10 * time.Second,
		CacheTTL:     120 * time.Second,
		CacheEnabled: true,
		MaxBulkSize:  100,
	})

	if client.config.Timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", client.config.Timeout)
	}
	if client.config.CacheTTL != 120*time.Second {
		t.Errorf("Expected cache TTL 120s, got %v", client.config.CacheTTL)
	}
	if client.config.MaxBulkSize != 100 {
		t.Errorf("Expected MaxBulkSize 100, got %d", client.config.MaxBulkSize)
	}
}

// ============================================
// Test: ValidateToken
// ============================================

func TestValidateToken_Success(t *testing.T) {
	// Mock server que retorna token válido
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/verify" {
			t.Errorf("Expected path /v1/auth/verify, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		response := TokenInfo{
			Valid:  true,
			UserID: "user-123",
			Email:  "test@example.com",
			Role:   "student",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(AuthClientConfig{
		BaseURL:      server.URL,
		CacheEnabled: false,
	})

	ctx := context.Background()
	info, err := client.ValidateToken(ctx, "valid-token")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !info.Valid {
		t.Error("Expected token to be valid")
	}
	if info.UserID != "user-123" {
		t.Errorf("Expected UserID user-123, got %s", info.UserID)
	}
	if info.Role != "student" {
		t.Errorf("Expected Role student, got %s", info.Role)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := TokenInfo{
			Valid: false,
			Error: "token expired",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(AuthClientConfig{
		BaseURL:      server.URL,
		CacheEnabled: false,
	})

	ctx := context.Background()
	info, err := client.ValidateToken(ctx, "invalid-token")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if info.Valid {
		t.Error("Expected token to be invalid")
	}
	if info.Error != "token expired" {
		t.Errorf("Expected error 'token expired', got %s", info.Error)
	}
}

func TestValidateToken_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewAuthClient(AuthClientConfig{
		BaseURL:      server.URL,
		CacheEnabled: false,
	})

	ctx := context.Background()
	info, err := client.ValidateToken(ctx, "some-token")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// Con circuit breaker, debe retornar info con error
	if info.Valid {
		t.Error("Expected token to be invalid on server error")
	}
}

func TestValidateToken_WithCache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		response := TokenInfo{
			Valid:  true,
			UserID: "user-123",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(AuthClientConfig{
		BaseURL:      server.URL,
		CacheEnabled: true,
		CacheTTL:     1 * time.Minute,
	})

	ctx := context.Background()

	// Primera llamada - debe ir al servidor
	_, _ = client.ValidateToken(ctx, "cached-token")
	if callCount != 1 {
		t.Errorf("Expected 1 server call, got %d", callCount)
	}

	// Segunda llamada - debe usar cache
	_, _ = client.ValidateToken(ctx, "cached-token")
	if callCount != 1 {
		t.Errorf("Expected still 1 server call (cached), got %d", callCount)
	}

	// Diferente token - debe ir al servidor
	_, _ = client.ValidateToken(ctx, "different-token")
	if callCount != 2 {
		t.Errorf("Expected 2 server calls (different token), got %d", callCount)
	}
}

// ============================================
// Test: ValidateTokensBulk
// ============================================

func TestValidateTokensBulk_Empty(t *testing.T) {
	client := NewAuthClient(AuthClientConfig{
		BaseURL: "http://localhost:8081",
	})

	ctx := context.Background()
	results, err := client.ValidateTokensBulk(ctx, []string{})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected empty results, got %d", len(results))
	}
}

func TestValidateTokensBulk_AllCached(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		response := TokenInfo{Valid: true, UserID: "user-123"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(AuthClientConfig{
		BaseURL:      server.URL,
		CacheEnabled: true,
		CacheTTL:     1 * time.Minute,
	})

	ctx := context.Background()

	// Pre-cachear tokens
	_, _ = client.ValidateToken(ctx, "token1")
	_, _ = client.ValidateToken(ctx, "token2")
	initialCalls := callCount

	// Bulk con tokens ya cacheados
	results, err := client.ValidateTokensBulk(ctx, []string{"token1", "token2"})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	if callCount != initialCalls {
		t.Errorf("Expected no additional server calls (all cached), got %d extra", callCount-initialCalls)
	}
}

func TestValidateTokensBulk_FallbackToIndividual(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/verify-bulk" {
			// Simular endpoint no existe
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/v1/auth/verify" {
			response := TokenInfo{Valid: true, UserID: "user-123"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
			return
		}
	}))
	defer server.Close()

	client := NewAuthClient(AuthClientConfig{
		BaseURL:      server.URL,
		CacheEnabled: false,
	})

	ctx := context.Background()
	results, err := client.ValidateTokensBulk(ctx, []string{"token1", "token2"})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results from fallback, got %d", len(results))
	}
	for _, r := range results {
		if !r.Info.Valid {
			t.Errorf("Expected valid token from fallback, got invalid")
		}
	}
}

func TestValidateTokensBulk_BulkEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/verify-bulk" {
			response := struct {
				Results []BulkTokenResult `json:"results"`
			}{
				Results: []BulkTokenResult{
					{Token: "token1", Info: &TokenInfo{Valid: true, UserID: "user-1"}},
					{Token: "token2", Info: &TokenInfo{Valid: true, UserID: "user-2"}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
			return
		}
	}))
	defer server.Close()

	client := NewAuthClient(AuthClientConfig{
		BaseURL:      server.URL,
		CacheEnabled: false,
	})

	ctx := context.Background()
	results, err := client.ValidateTokensBulk(ctx, []string{"token1", "token2"})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	if results[0].Info.UserID != "user-1" {
		t.Errorf("Expected user-1, got %s", results[0].Info.UserID)
	}
	if results[1].Info.UserID != "user-2" {
		t.Errorf("Expected user-2, got %s", results[1].Info.UserID)
	}
}

func TestValidateTokensBulk_Chunking(t *testing.T) {
	chunkCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/verify-bulk" {
			mu.Lock()
			chunkCount++
			mu.Unlock()

			var reqBody struct {
				Tokens []string `json:"tokens"`
			}
			_ = json.NewDecoder(r.Body).Decode(&reqBody)

			results := make([]BulkTokenResult, len(reqBody.Tokens))
			for i, token := range reqBody.Tokens {
				results[i] = BulkTokenResult{
					Token: token,
					Info:  &TokenInfo{Valid: true, UserID: "user-" + token},
				}
			}

			response := struct {
				Results []BulkTokenResult `json:"results"`
			}{Results: results}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
			return
		}
	}))
	defer server.Close()

	client := NewAuthClient(AuthClientConfig{
		BaseURL:      server.URL,
		CacheEnabled: false,
		MaxBulkSize:  3, // Pequeño para forzar chunking
	})

	ctx := context.Background()
	tokens := []string{"t1", "t2", "t3", "t4", "t5", "t6", "t7"}
	results, err := client.ValidateTokensBulk(ctx, tokens)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(results) != 7 {
		t.Errorf("Expected 7 results, got %d", len(results))
	}
	// Con MaxBulkSize=3, 7 tokens = 3 chunks (3+3+1)
	if chunkCount != 3 {
		t.Errorf("Expected 3 chunks, got %d", chunkCount)
	}
}

// ============================================
// Test: Token Cache
// ============================================

func TestTokenCache_GetSet(t *testing.T) {
	cache := newTokenCache(1 * time.Minute)

	info := &TokenInfo{Valid: true, UserID: "user-123"}
	cache.Set("key1", info)

	retrieved, found := cache.Get("key1")
	if !found {
		t.Error("Expected to find cached entry")
	}
	if retrieved.UserID != "user-123" {
		t.Errorf("Expected UserID user-123, got %s", retrieved.UserID)
	}
}

func TestTokenCache_Expiration(t *testing.T) {
	cache := newTokenCache(50 * time.Millisecond)

	info := &TokenInfo{Valid: true, UserID: "user-123"}
	cache.Set("key1", info)

	// Debe estar en cache inmediatamente
	_, found := cache.Get("key1")
	if !found {
		t.Error("Expected to find cached entry immediately")
	}

	// Esperar a que expire
	time.Sleep(100 * time.Millisecond)

	_, found = cache.Get("key1")
	if found {
		t.Error("Expected entry to be expired")
	}
}

func TestTokenCache_Stats(t *testing.T) {
	cache := newTokenCache(50 * time.Millisecond)

	cache.Set("key1", &TokenInfo{Valid: true})
	cache.Set("key2", &TokenInfo{Valid: true})

	total, expired := cache.Stats()
	if total != 2 {
		t.Errorf("Expected 2 total entries, got %d", total)
	}
	if expired != 0 {
		t.Errorf("Expected 0 expired entries, got %d", expired)
	}

	// Esperar a que expiren
	time.Sleep(100 * time.Millisecond)

	total, expired = cache.Stats()
	if total != 2 {
		t.Errorf("Expected 2 total entries (still in map), got %d", total)
	}
	if expired != 2 {
		t.Errorf("Expected 2 expired entries, got %d", expired)
	}
}

// ============================================
// Test: Hash Token
// ============================================

func TestHashToken_Consistency(t *testing.T) {
	client := NewAuthClient(AuthClientConfig{BaseURL: "http://localhost"})

	hash1 := client.hashToken("same-token")
	hash2 := client.hashToken("same-token")

	if hash1 != hash2 {
		t.Error("Expected same hash for same token")
	}
}

func TestHashToken_Different(t *testing.T) {
	client := NewAuthClient(AuthClientConfig{BaseURL: "http://localhost"})

	hash1 := client.hashToken("token1")
	hash2 := client.hashToken("token2")

	if hash1 == hash2 {
		t.Error("Expected different hash for different tokens")
	}
}

// ============================================
// Test: Chunk Tokens
// ============================================

func TestChunkTokens(t *testing.T) {
	client := NewAuthClient(AuthClientConfig{
		BaseURL:     "http://localhost",
		MaxBulkSize: 3,
	})

	testCases := []struct {
		name     string
		tokens   []string
		expected int // número de chunks
	}{
		{"empty", []string{}, 0},
		{"less than max", []string{"a", "b"}, 1},
		{"exact max", []string{"a", "b", "c"}, 1},
		{"one over", []string{"a", "b", "c", "d"}, 2},
		{"multiple chunks", []string{"a", "b", "c", "d", "e", "f", "g"}, 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chunks := client.chunkTokens(tc.tokens)
			if len(chunks) != tc.expected {
				t.Errorf("Expected %d chunks, got %d", tc.expected, len(chunks))
			}
		})
	}
}

// ============================================
// Test: Concurrent Access
// ============================================

func TestValidateToken_ConcurrentAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := TokenInfo{Valid: true, UserID: "user-123"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(AuthClientConfig{
		BaseURL:      server.URL,
		CacheEnabled: true,
	})

	ctx := context.Background()
	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// 100 goroutines concurrentes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.ValidateToken(ctx, "concurrent-token")
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent error: %v", err)
	}
}

// ============================================
// Test: GetCacheStats
// ============================================

func TestGetCacheStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := TokenInfo{Valid: true, UserID: "user-123"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAuthClient(AuthClientConfig{
		BaseURL:      server.URL,
		CacheEnabled: true,
	})

	ctx := context.Background()
	_, _ = client.ValidateToken(ctx, "token1")
	_, _ = client.ValidateToken(ctx, "token2")

	total, expired := client.GetCacheStats()
	if total != 2 {
		t.Errorf("Expected 2 cached entries, got %d", total)
	}
	if expired != 0 {
		t.Errorf("Expected 0 expired entries, got %d", expired)
	}
}
