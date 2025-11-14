package digitalocean

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// SpacesClient handles DigitalOcean Spaces operations
type SpacesClient struct {
	s3Client *s3.S3
	bucket   string
	region   string
	endpoint string
	cdnURL   string
}

// SpacesConfig holds configuration for Spaces client
type SpacesConfig struct {
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	Endpoint  string
	CDNURL    string
}

// NewSpacesClient creates a new Spaces client
func NewSpacesClient(config SpacesConfig) (*SpacesClient, error) {
	// Create AWS session with DigitalOcean Spaces endpoint
	sess, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(
			config.AccessKey,
			config.SecretKey,
			"",
		),
		Endpoint:         aws.String(config.Endpoint),
		Region:           aws.String(config.Region),
		S3ForcePathStyle: aws.Bool(false),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Spaces session: %w", err)
	}

	return &SpacesClient{
		s3Client: s3.New(sess),
		bucket:   config.Bucket,
		region:   config.Region,
		endpoint: config.Endpoint,
		cdnURL:   config.CDNURL,
	}, nil
}

// UploadFile uploads a file to Spaces
func (s *SpacesClient) UploadFile(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	// Upload to S3-compatible Spaces
	_, err := s.s3Client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        aws.ReadSeekCloser(data),
		ACL:         aws.String("public-read"), // Make publicly accessible
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// Return CDN URL if available, otherwise regular URL
	if s.cdnURL != "" {
		return fmt.Sprintf("%s/%s", s.cdnURL, key), nil
	}
	return fmt.Sprintf("https://%s.%s/%s", s.bucket, s.endpoint, key), nil
}

// UploadBytes uploads bytes to Spaces
func (s *SpacesClient) UploadBytes(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	return s.UploadFile(ctx, key, bytes.NewReader(data), contentType)
}

// DownloadFile downloads a file from Spaces
func (s *SpacesClient) DownloadFile(ctx context.Context, key string) ([]byte, error) {
	result, err := s.s3Client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer result.Body.Close()

	return io.ReadAll(result.Body)
}

// DeleteFile deletes a file from Spaces
func (s *SpacesClient) DeleteFile(ctx context.Context, key string) error {
	_, err := s.s3Client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// ListFiles lists files in a directory (prefix)
func (s *SpacesClient) ListFiles(ctx context.Context, prefix string) ([]string, error) {
	result, err := s.s3Client.ListObjectsV2WithContext(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	var keys []string
	for _, obj := range result.Contents {
		keys = append(keys, *obj.Key)
	}
	return keys, nil
}

// FileExists checks if a file exists in Spaces
func (s *SpacesClient) FileExists(ctx context.Context, key string) (bool, error) {
	_, err := s.s3Client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Check if error is "not found"
		return false, nil
	}
	return true, nil
}

// GetFileURL returns the public URL for a file
func (s *SpacesClient) GetFileURL(key string) string {
	if s.cdnURL != "" {
		return fmt.Sprintf("%s/%s", s.cdnURL, key)
	}
	return fmt.Sprintf("https://%s.%s/%s", s.bucket, s.endpoint, key)
}

// GetPresignedURL generates a presigned URL for temporary access
func (s *SpacesClient) GetPresignedURL(key string, expiration time.Duration) (string, error) {
	req, _ := s.s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	url, err := req.Presign(expiration)
	if err != nil {
		return "", fmt.Errorf("failed to presign URL: %w", err)
	}
	return url, nil
}

// GenerateKey generates a unique key for file storage
func GenerateKey(prefix, filename string) string {
	timestamp := time.Now().Unix()
	ext := filepath.Ext(filename)
	base := filename[:len(filename)-len(ext)]

	return fmt.Sprintf("%s/%d_%s%s", prefix, timestamp, base, ext)
}

// GetContentType returns the content type for a filename
func GetContentType(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".doc":
		return "application/msword"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".csv":
		return "text/csv"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	default:
		return "application/octet-stream"
	}
}
