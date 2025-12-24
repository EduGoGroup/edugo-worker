package s3

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	maxRetries  = 3
	baseBackoff = 100 * time.Millisecond

	// Límites de tamaño de archivo
	maxFileSize = 100 * 1024 * 1024 // 100MB
	minFileSize = 1024              // 1KB

	// Tipos de contenido permitidos
	contentTypePDF = "application/pdf"
)

var (
	// Extensiones de archivo permitidas
	allowedExtensions = []string{".pdf"}
)

type Client struct {
	s3Client        *s3.Client
	bucket          string
	logger          logger.Logger
	maxFileSize     int64
	minFileSize     int64
	allowedTypes    []string
	downloadTimeout time.Duration
}

func NewClient(ctx context.Context, region, bucket, endpoint, accessKey, secretKey string, usePathStyle bool, log logger.Logger) (*Client, error) {
	log.Info("creando cliente S3", "region", region, "bucket", bucket, "endpoint", endpoint)

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("error cargando config AWS: %w", err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = usePathStyle
	})

	return &Client{
		s3Client:        s3Client,
		bucket:          bucket,
		logger:          log,
		maxFileSize:     maxFileSize,
		minFileSize:     minFileSize,
		allowedTypes:    []string{contentTypePDF},
		downloadTimeout: 30 * time.Second,
	}, nil
}

func (c *Client) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	c.logger.Debug("descargando archivo", "bucket", c.bucket, "key", key)

	// Validar extensión del archivo
	if err := c.validateFileExtension(key); err != nil {
		return nil, err
	}

	// Validar metadata antes de descargar (tamaño y tipo)
	metadata, err := c.GetMetadata(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo metadata antes de descarga: %w", err)
	}

	if err := c.validateFileSize(metadata.Size, key); err != nil {
		return nil, err
	}

	if err := c.validateContentType(metadata.ContentType, key); err != nil {
		return nil, err
	}

	// Crear contexto con timeout para descarga
	downloadCtx, cancel := context.WithTimeout(ctx, c.downloadTimeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := baseBackoff * time.Duration(1<<attempt)
			c.logger.Debug("esperando antes de reintentar", "backoff", backoff.String(), "attempt", attempt+1)
			time.Sleep(backoff)
		}

		output, err := c.s3Client.GetObject(downloadCtx, &s3.GetObjectInput{
			Bucket: aws.String(c.bucket),
			Key:    aws.String(key),
		})
		if err == nil {
			c.logger.Info("archivo descargado exitosamente",
				"key", key,
				"size_mb", float64(metadata.Size)/(1024*1024),
				"content_type", metadata.ContentType,
			)
			return output.Body, nil
		}
		lastErr = err
		c.logger.Warn("reintentando descarga", "attempt", attempt+1, "max_retries", maxRetries, "error", err.Error())
	}

	return nil, fmt.Errorf("error descargando %s después de %d intentos: %w", key, maxRetries, lastErr)
}

// validateFileExtension valida que el archivo tenga una extensión permitida
func (c *Client) validateFileExtension(key string) error {
	keyLower := strings.ToLower(key)
	for _, ext := range allowedExtensions {
		if strings.HasSuffix(keyLower, ext) {
			return nil
		}
	}
	return fmt.Errorf("extensión de archivo no permitida: %s (permitidas: %v)", key, allowedExtensions)
}

// validateFileSize valida que el tamaño del archivo esté dentro de los límites
func (c *Client) validateFileSize(size int64, key string) error {
	if size < c.minFileSize {
		return fmt.Errorf("archivo demasiado pequeño: %s (%d bytes, mínimo: %d bytes)", key, size, c.minFileSize)
	}
	if size > c.maxFileSize {
		sizeMB := float64(size) / (1024 * 1024)
		maxMB := float64(c.maxFileSize) / (1024 * 1024)
		return fmt.Errorf("archivo demasiado grande: %s (%.2f MB, máximo: %.2f MB)", key, sizeMB, maxMB)
	}
	return nil
}

// validateContentType valida que el tipo de contenido sea permitido
func (c *Client) validateContentType(contentType, key string) error {
	// Normalizar content type (remover charset y otros parámetros)
	normalizedType := strings.ToLower(strings.Split(contentType, ";")[0])
	normalizedType = strings.TrimSpace(normalizedType)

	for _, allowedType := range c.allowedTypes {
		if normalizedType == allowedType {
			return nil
		}
	}

	// Permitir también si el content type está vacío pero la extensión es correcta
	if normalizedType == "" || normalizedType == "application/octet-stream" {
		c.logger.Warn("content type genérico o vacío, validando solo por extensión", "key", key, "content_type", contentType)
		return nil
	}

	return fmt.Errorf("tipo de contenido no permitido: %s (tipo: %s, permitidos: %v)", key, contentType, c.allowedTypes)
}

func (c *Client) Upload(ctx context.Context, key string, content io.Reader) error {
	c.logger.Debug("subiendo archivo", "bucket", c.bucket, "key", key)

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(baseBackoff * time.Duration(1<<attempt))
		}

		_, err := c.s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(c.bucket),
			Key:    aws.String(key),
			Body:   content,
		})
		if err == nil {
			c.logger.Info("archivo subido exitosamente", "key", key)
			return nil
		}
		lastErr = err
		c.logger.Warn("reintentando subida", "attempt", attempt+1, "error", err.Error())
	}

	return fmt.Errorf("error subiendo %s: %w", key, lastErr)
}

func (c *Client) Delete(ctx context.Context, key string) error {
	c.logger.Debug("eliminando archivo", "bucket", c.bucket, "key", key)

	_, err := c.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("error eliminando %s: %w", key, err)
	}

	c.logger.Info("archivo eliminado", "key", key)
	return nil
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	_, err := c.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (c *Client) GetMetadata(ctx context.Context, key string) (*storage.FileMetadata, error) {
	output, err := c.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("error obteniendo metadata de %s: %w", key, err)
	}

	return &storage.FileMetadata{
		Key:          key,
		Size:         aws.ToInt64(output.ContentLength),
		ContentType:  aws.ToString(output.ContentType),
		LastModified: output.LastModified.String(),
		ETag:         aws.ToString(output.ETag),
	}, nil
}

var _ storage.Client = (*Client)(nil)
