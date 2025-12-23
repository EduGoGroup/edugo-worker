package storage

import (
	"context"
	"io"
)

// Client define la interfaz para clientes de almacenamiento
// Permite abstraer S3, MinIO, filesystem local, etc.
type Client interface {
	// Download descarga un archivo desde el storage
	// Returns: io.ReadCloser que debe cerrarse después de usar
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Upload sube un archivo al storage
	Upload(ctx context.Context, key string, content io.Reader) error

	// Delete elimina un archivo del storage
	Delete(ctx context.Context, key string) error

	// Exists verifica si un archivo existe
	Exists(ctx context.Context, key string) (bool, error)

	// GetMetadata obtiene metadatos de un archivo
	GetMetadata(ctx context.Context, key string) (*FileMetadata, error)
}

// FileMetadata contiene información sobre un archivo en storage
type FileMetadata struct {
	Key          string
	Size         int64
	ContentType  string
	LastModified string
	ETag         string
}
