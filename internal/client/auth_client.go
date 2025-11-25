// Package client proporciona clientes HTTP para comunicación con otros servicios.
// AuthClient permite validar tokens JWT contra api-admin como autoridad central.
// Optimizado para workers con soporte de validación en bulk.
package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

// TokenInfo contiene la información de un token validado por api-admin
type TokenInfo struct {
	Valid     bool      `json:"valid"`
	UserID    string    `json:"user_id,omitempty"`
	Email     string    `json:"email,omitempty"`
	Role      string    `json:"role,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// BulkTokenResult resultado de validación bulk
type BulkTokenResult struct {
	Token string     `json:"token"`
	Info  *TokenInfo `json:"info"`
}

// AuthClientConfig configuración del cliente de autenticación
type AuthClientConfig struct {
	BaseURL        string               // URL base de api-admin (ej: http://localhost:8081)
	Timeout        time.Duration        // Timeout para requests HTTP (default: 5s)
	CacheTTL       time.Duration        // TTL del cache de validaciones (default: 60s)
	CacheEnabled   bool                 // Habilitar cache de validaciones
	CircuitBreaker CircuitBreakerConfig // Configuración del circuit breaker
	MaxBulkSize    int                  // Máximo de tokens por request bulk (default: 50)
}

// CircuitBreakerConfig configuración del circuit breaker
type CircuitBreakerConfig struct {
	MaxRequests uint32        // Máximo de requests en estado half-open
	Interval    time.Duration // Intervalo para resetear contadores
	Timeout     time.Duration // Tiempo que permanece abierto antes de half-open
}

// AuthClient cliente para validar tokens con api-admin
// Optimizado para workers con validación bulk y cache
type AuthClient struct {
	baseURL        string
	httpClient     *http.Client
	cache          *tokenCache
	circuitBreaker *gobreaker.CircuitBreaker
	config         AuthClientConfig
}

// NewAuthClient crea una nueva instancia del cliente de autenticación
func NewAuthClient(config AuthClientConfig) *AuthClient {
	// Valores por defecto
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 60 * time.Second
	}
	if config.MaxBulkSize == 0 {
		config.MaxBulkSize = 50
	}

	// Configurar circuit breaker
	cbSettings := gobreaker.Settings{
		Name:        "worker-auth-service",
		MaxRequests: config.CircuitBreaker.MaxRequests,
		Interval:    config.CircuitBreaker.Interval,
		Timeout:     config.CircuitBreaker.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Abrir circuito si hay 60% de fallos con al menos 3 requests
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			fmt.Printf("[WorkerAuthClient] Circuit breaker '%s': %s -> %s\n", name, from, to)
		},
	}

	// Valores por defecto del circuit breaker
	if cbSettings.MaxRequests == 0 {
		cbSettings.MaxRequests = 3
	}
	if cbSettings.Interval == 0 {
		cbSettings.Interval = 10 * time.Second
	}
	if cbSettings.Timeout == 0 {
		cbSettings.Timeout = 30 * time.Second
	}

	return &AuthClient{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		cache:          newTokenCache(config.CacheTTL),
		circuitBreaker: gobreaker.NewCircuitBreaker(cbSettings),
		config:         config,
	}
}

// ValidateToken valida un token JWT con api-admin
// Utiliza cache y circuit breaker para resiliencia
func (c *AuthClient) ValidateToken(ctx context.Context, token string) (*TokenInfo, error) {
	// 1. Verificar cache primero
	cacheKey := c.hashToken(token)
	if c.config.CacheEnabled {
		if cached, found := c.cache.Get(cacheKey); found {
			return cached, nil
		}
	}

	// 2. Llamar a api-admin con circuit breaker
	result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
		return c.doValidateToken(ctx, token)
	})

	if err != nil {
		return &TokenInfo{Valid: false, Error: fmt.Sprintf("auth service error: %v", err)}, nil
	}

	info := result.(*TokenInfo)

	// 3. Guardar en cache si el token es válido
	if c.config.CacheEnabled && info.Valid {
		c.cache.Set(cacheKey, info)
	}

	return info, nil
}

// ValidateTokensBulk valida múltiples tokens en una sola llamada
// Optimizado para workers que procesan batches de eventos
func (c *AuthClient) ValidateTokensBulk(ctx context.Context, tokens []string) ([]BulkTokenResult, error) {
	if len(tokens) == 0 {
		return []BulkTokenResult{}, nil
	}

	results := make([]BulkTokenResult, len(tokens))
	tokensToValidate := make([]string, 0, len(tokens))
	tokenIndices := make(map[string][]int) // Map token -> indices en results

	// 1. Verificar cache primero para cada token
	for i, token := range tokens {
		cacheKey := c.hashToken(token)
		if c.config.CacheEnabled {
			if cached, found := c.cache.Get(cacheKey); found {
				results[i] = BulkTokenResult{Token: token, Info: cached}
				continue
			}
		}

		// Token necesita validación remota
		if _, exists := tokenIndices[token]; !exists {
			tokensToValidate = append(tokensToValidate, token)
		}
		tokenIndices[token] = append(tokenIndices[token], i)
	}

	// 2. Si todos estaban en cache, retornar
	if len(tokensToValidate) == 0 {
		return results, nil
	}

	// 3. Dividir en chunks si excede MaxBulkSize
	chunks := c.chunkTokens(tokensToValidate)

	// 4. Validar cada chunk
	for _, chunk := range chunks {
		chunkResults, err := c.doValidateTokensBulk(ctx, chunk)
		if err != nil {
			// En caso de error, marcar todos los tokens del chunk como inválidos
			for _, token := range chunk {
				errorInfo := &TokenInfo{Valid: false, Error: fmt.Sprintf("bulk validation error: %v", err)}
				for _, idx := range tokenIndices[token] {
					results[idx] = BulkTokenResult{Token: token, Info: errorInfo}
				}
			}
			continue
		}

		// 5. Asignar resultados y guardar en cache
		for _, result := range chunkResults {
			if c.config.CacheEnabled && result.Info.Valid {
				cacheKey := c.hashToken(result.Token)
				c.cache.Set(cacheKey, result.Info)
			}
			for _, idx := range tokenIndices[result.Token] {
				results[idx] = result
			}
		}
	}

	return results, nil
}

// doValidateToken realiza la llamada HTTP a api-admin /v1/auth/verify
func (c *AuthClient) doValidateToken(ctx context.Context, token string) (*TokenInfo, error) {
	url := c.baseURL + "/v1/auth/verify"

	// Preparar request body
	reqBody := map[string]string{"token": token}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Crear request con contexto
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Ejecutar request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling auth service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Leer response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// Verificar status code
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("auth service error: status %d", resp.StatusCode)
	}

	// Parsear response
	var info TokenInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return &info, nil
}

// doValidateTokensBulk realiza validación bulk con api-admin /v1/auth/verify-bulk
func (c *AuthClient) doValidateTokensBulk(ctx context.Context, tokens []string) ([]BulkTokenResult, error) {
	// Usar circuit breaker para la llamada bulk
	result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
		return c.doBulkRequest(ctx, tokens)
	})

	if err != nil {
		return nil, err
	}

	return result.([]BulkTokenResult), nil
}

// doBulkRequest ejecuta el request HTTP bulk
func (c *AuthClient) doBulkRequest(ctx context.Context, tokens []string) ([]BulkTokenResult, error) {
	url := c.baseURL + "/v1/auth/verify-bulk"

	// Preparar request body
	reqBody := map[string][]string{"tokens": tokens}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling bulk request: %w", err)
	}

	// Crear request con contexto
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating bulk request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Ejecutar request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling auth service bulk: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Leer response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading bulk response: %w", err)
	}

	// Verificar status code
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("auth service bulk error: status %d", resp.StatusCode)
	}

	// Si el endpoint bulk no existe, fallback a validación individual
	if resp.StatusCode == 404 {
		return c.fallbackIndividualValidation(ctx, tokens)
	}

	// Parsear response bulk
	var bulkResponse struct {
		Results []BulkTokenResult `json:"results"`
	}
	if err := json.Unmarshal(body, &bulkResponse); err != nil {
		return nil, fmt.Errorf("error parsing bulk response: %w", err)
	}

	return bulkResponse.Results, nil
}

// fallbackIndividualValidation valida tokens uno por uno si el endpoint bulk no existe
func (c *AuthClient) fallbackIndividualValidation(ctx context.Context, tokens []string) ([]BulkTokenResult, error) {
	results := make([]BulkTokenResult, len(tokens))

	for i, token := range tokens {
		info, err := c.doValidateToken(ctx, token)
		if err != nil {
			results[i] = BulkTokenResult{
				Token: token,
				Info:  &TokenInfo{Valid: false, Error: err.Error()},
			}
			continue
		}
		results[i] = BulkTokenResult{Token: token, Info: info}
	}

	return results, nil
}

// chunkTokens divide tokens en chunks del tamaño MaxBulkSize
func (c *AuthClient) chunkTokens(tokens []string) [][]string {
	var chunks [][]string
	for i := 0; i < len(tokens); i += c.config.MaxBulkSize {
		end := i + c.config.MaxBulkSize
		if end > len(tokens) {
			end = len(tokens)
		}
		chunks = append(chunks, tokens[i:end])
	}
	return chunks
}

// hashToken genera un hash SHA256 del token para usar como cache key
func (c *AuthClient) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// GetCacheStats retorna estadísticas del cache (para métricas)
func (c *AuthClient) GetCacheStats() (total int, expired int) {
	return c.cache.Stats()
}

// ============================================
// Token Cache Implementation
// ============================================

type tokenCache struct {
	entries map[string]*cacheEntry
	ttl     time.Duration
	mutex   sync.RWMutex
}

type cacheEntry struct {
	info      *TokenInfo
	expiresAt time.Time
}

func newTokenCache(ttl time.Duration) *tokenCache {
	cache := &tokenCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}

	// Iniciar limpieza periódica de entries expirados
	go cache.cleanupLoop()

	return cache
}

// Get obtiene un entry del cache si existe y no ha expirado
func (c *tokenCache) Get(key string) (*TokenInfo, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.info, true
}

// Set guarda un entry en el cache con el TTL configurado
func (c *tokenCache) Set(key string, info *TokenInfo) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.entries[key] = &cacheEntry{
		info:      info,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// cleanupLoop limpia entries expirados cada minuto
func (c *tokenCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup elimina todos los entries expirados
func (c *tokenCache) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}

// Stats retorna estadísticas del cache (para métricas)
func (c *tokenCache) Stats() (total int, expired int) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	now := time.Now()
	total = len(c.entries)
	for _, entry := range c.entries {
		if now.After(entry.expiresAt) {
			expired++
		}
	}
	return total, expired
}
