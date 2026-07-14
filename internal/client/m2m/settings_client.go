package m2m

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TokenProvider entrega un service JWT para autenticar la llamada M2M.
type TokenProvider interface {
	Token() (string, error)
}

// ResolvedSetting es una clave de configuración de escuela YA RESUELTA por
// academic (fila propia o default de plataforma), con su procedencia.
type ResolvedSetting struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"` // "school" | "default"
}

// SchoolSettings es la respuesta del endpoint M2M de academic
// (GET /api/v1/internal/schools/{school_id}/settings). El worker recibe valores
// ya resueltos: NO importa el catálogo (design 039 §3).
type SchoolSettings struct {
	SchoolID string            `json:"school_id"`
	Settings []ResolvedSetting `json:"settings"`
}

// Get devuelve el valor resuelto de una clave, o ("", false) si no está.
func (s SchoolSettings) Get(key string) (string, bool) {
	for _, r := range s.Settings {
		if r.Key == key {
			return r.Value, true
		}
	}
	return "", false
}

// SettingsClientConfig configura el SettingsClient.
type SettingsClientConfig struct {
	// BaseURL de academic (ej. http://localhost:8060). El path interno se anexa.
	BaseURL string
	// Timeout de la request HTTP (default 5s).
	Timeout time.Duration
	// CacheTTL del valor resuelto por school_id (default 60s, TTL corto: el
	// riesgo de config leída por M2M en runtime se mitiga con caché corta,
	// design 039 §7).
	CacheTTL time.Duration
	// TokenProvider firma/obtiene el service JWT (scope schools.settings.read).
	TokenProvider TokenProvider
}

// settingsPath es la ruta del endpoint M2M en academic (%s = school_id).
const settingsPathFmt = "/api/v1/internal/schools/%s/settings"

// SettingsClient lee la configuración resuelta de una escuela desde academic,
// con caché TTL corta por school_id. Seguro para uso concurrente.
type SettingsClient struct {
	baseURL       string
	httpClient    *http.Client
	tokenProvider TokenProvider
	cacheTTL      time.Duration

	mu    sync.RWMutex
	cache map[string]settingsCacheEntry
}

type settingsCacheEntry struct {
	value     SchoolSettings
	expiresAt time.Time
}

// NewSettingsClient crea el cliente.
func NewSettingsClient(cfg SettingsClientConfig) *SettingsClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ttl := cfg.CacheTTL
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &SettingsClient{
		baseURL:       strings.TrimRight(cfg.BaseURL, "/"),
		httpClient:    &http.Client{Timeout: timeout},
		tokenProvider: cfg.TokenProvider,
		cacheTTL:      ttl,
		cache:         make(map[string]settingsCacheEntry),
	}
}

// GetSettings devuelve la configuración resuelta de una escuela, sirviéndola de
// caché si está vigente. NUNCA loguea el token.
func (c *SettingsClient) GetSettings(ctx context.Context, schoolID string) (SchoolSettings, error) {
	if schoolID == "" {
		return SchoolSettings{}, fmt.Errorf("school_id vacío")
	}

	if cached, ok := c.getCached(schoolID); ok {
		return cached, nil
	}

	settings, err := c.fetch(ctx, schoolID)
	if err != nil {
		return SchoolSettings{}, err
	}

	c.setCached(schoolID, settings)
	return settings, nil
}

func (c *SettingsClient) getCached(schoolID string) (SchoolSettings, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.cache[schoolID]
	if !ok || time.Now().After(entry.expiresAt) {
		return SchoolSettings{}, false
	}
	return entry.value, true
}

func (c *SettingsClient) setCached(schoolID string, value SchoolSettings) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[schoolID] = settingsCacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.cacheTTL),
	}
}

func (c *SettingsClient) fetch(ctx context.Context, schoolID string) (SchoolSettings, error) {
	token, err := c.tokenProvider.Token()
	if err != nil {
		return SchoolSettings{}, fmt.Errorf("obtaining service token: %w", err)
	}

	url := c.baseURL + fmt.Sprintf(settingsPathFmt, schoolID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return SchoolSettings{}, fmt.Errorf("creating settings request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return SchoolSettings{}, fmt.Errorf("settings request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SchoolSettings{}, fmt.Errorf("reading settings response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SchoolSettings{}, fmt.Errorf("settings returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var settings SchoolSettings
	if err := json.Unmarshal(body, &settings); err != nil {
		return SchoolSettings{}, fmt.Errorf("parsing settings response: %w", err)
	}
	return settings, nil
}
