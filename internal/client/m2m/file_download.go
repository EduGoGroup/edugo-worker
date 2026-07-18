package m2m

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ErrFileTooLarge marca que el archivo descargado excede el límite de bytes
// permitido. Un PDF gigante NO se cura reintentando, así que envuelve
// ErrLearningPermanent: el clasificador de retry (retry.go) lo trata como permanente
// (a DLQ, sin reintento). El caller puede distinguirlo con errors.Is(err, ErrFileTooLarge).
var ErrFileTooLarge = fmt.Errorf("%w: el archivo excede el límite de tamaño", ErrLearningPermanent)

// downloadClient es un http.Client sin timeout global: la cancelación/duración la
// gobierna el context del caller (una descarga grande no debe morir por un timeout
// fijo pensado para requests de control M2M).
var downloadClient = &http.Client{}

// DownloadFile baja el contenido de una URL ya firmada (presignada) con un GET
// simple SIN Authorization —la firma va en la propia URL—. Respeta el context y
// corta por bytes REALES leídos (no confía en Content-Length, que puede mentir):
// si el cuerpo supera maxBytes, aborta con ErrFileTooLarge. Clasificación de errores:
//   - 2xx → bytes; nil (salvo exceso de tamaño → ErrFileTooLarge, permanente).
//   - 4xx salvo 408/429 (incl. 403 = URL firmada expirada/inválida) → ErrLearningPermanent.
//   - 5xx / red / timeout / 408 / 429 → transitorio sin sentinel.
func DownloadFile(ctx context.Context, url string, maxBytes int64) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("url vacía")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request: %w", err)
	}

	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := fmt.Sprintf("download returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 &&
			resp.StatusCode != http.StatusRequestTimeout && resp.StatusCode != http.StatusTooManyRequests {
			return nil, fmt.Errorf("%w: %s", ErrLearningPermanent, msg)
		}
		return nil, fmt.Errorf("%s", msg)
	}

	// Leemos hasta maxBytes+1: si conseguimos ese byte extra, el archivo excede el
	// límite (el Content-Length no es de fiar, cortamos por bytes reales).
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading download body: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w (límite %d bytes)", ErrFileTooLarge, maxBytes)
	}
	return data, nil
}
