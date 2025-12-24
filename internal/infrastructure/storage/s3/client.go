package s3

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/storage"
)

const (
	maxRetries    = 3
	baseBackoff   = 100 * time.Millisecond
)

type Client struct {
	s3Client *s3.Client
	bucket   string
	logger   logger.Logger
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
		s3Client: s3Client,
		bucket:   bucket,
		logger:   log,
	}, nil
}

func (c *Client) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	c.logger.Debug("descargando archivo", "bucket", c.bucket, "key", key)

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(baseBackoff * time.Duration(1<<attempt))
		}

		output, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(c.bucket),
			Key:    aws.String(key),
		})
		if err == nil {
			c.logger.Info("archivo descargado exitosamente", "key", key)
			return output.Body, nil
		}
		lastErr = err
		c.logger.Warn("reintentando descarga", "attempt", attempt+1, "error", err.Error())
	}

	return nil, fmt.Errorf("error descargando %s después de %d intentos: %w", key, maxRetries, lastErr)
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
