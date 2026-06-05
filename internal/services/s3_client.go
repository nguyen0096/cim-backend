package services

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"cim-backend/internal/config"
)

//go:generate mockery --name=S3Client --structname=S3Client --output=../mocks/servicemocks --outpkg=servicemocks
type S3Client interface {
	UploadFile(ctx context.Context, key string, content []byte, contentType string) error
	// GeneratePresignedURL returns a presigned GET URL for key. An optional
	// downloadFilename sets Content-Disposition so the browser saves the file
	// under a meaningful name instead of the storage key (e.g. a UUID).
	GeneratePresignedURL(ctx context.Context, key string, expiration time.Duration, downloadFilename ...string) (string, error)
}

type r2Client struct {
	s3Client      *s3.Client
	presignClient *s3.PresignClient
	bucketName    string
}

// NewS3Client creates a new S3-compatible client for Cloudflare R2
func NewS3Client(cfg *config.Config) (S3Client, error) {
	if !cfg.R2.Enabled {
		return &noopS3Client{}, nil
	}

	if cfg.R2.AccountID == "" || cfg.R2.AccessKeyID == "" ||
		cfg.R2.SecretAccessKey == "" || cfg.R2.BucketName == "" {
		return nil, fmt.Errorf("R2 configuration incomplete")
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.R2.AccountID)

	r2Config := aws.Config{
		Region: "auto",
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.R2.AccessKeyID,
			cfg.R2.SecretAccessKey,
			"",
		),
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:           endpoint,
					SigningRegion: "auto",
				}, nil
			},
		),
	}

	s3Client := s3.NewFromConfig(r2Config)
	presignClient := s3.NewPresignClient(s3Client)

	return &r2Client{
		s3Client:      s3Client,
		presignClient: presignClient,
		bucketName:    cfg.R2.BucketName,
	}, nil
}

func (c *r2Client) UploadFile(ctx context.Context, key string, content []byte, contentType string) error {
	_, err := c.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(content),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to R2: %w", err)
	}
	return nil
}

func (c *r2Client) GeneratePresignedURL(ctx context.Context, key string, expiration time.Duration, downloadFilename ...string) (string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(key),
	}
	if len(downloadFilename) > 0 && downloadFilename[0] != "" {
		input.ResponseContentDisposition = aws.String(
			fmt.Sprintf(`attachment; filename="%s"`, downloadFilename[0]),
		)
	}
	request, err := c.presignClient.PresignGetObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = expiration
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	return request.URL, nil
}

// noopS3Client is used when R2 is disabled
type noopS3Client struct{}

func (c *noopS3Client) UploadFile(ctx context.Context, key string, content []byte, contentType string) error {
	return nil
}

func (c *noopS3Client) GeneratePresignedURL(ctx context.Context, key string, expiration time.Duration, downloadFilename ...string) (string, error) {
	return "", nil
}
