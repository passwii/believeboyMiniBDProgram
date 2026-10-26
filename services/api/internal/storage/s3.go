package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/believeboy/mini-bd-program/services/api/internal/config"
)

type S3Store struct {
	client *s3.Client
	bucket string
}

func NewS3Store(cfg config.Config) (*S3Store, error) {
	if cfg.S3Bucket == "" {
		return nil, errors.New("S3 bucket is required")
	}
	awsConfig := aws.Config{
		Region: cfg.S3Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.S3AccessKey,
			cfg.S3SecretKey,
			"",
		),
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = cfg.S3UsePathStyle
		if cfg.S3Endpoint != "" {
			options.BaseEndpoint = aws.String(cfg.S3Endpoint)
		}
	})
	return &S3Store{client: client, bucket: cfg.S3Bucket}, nil
}

func (s *S3Store) Put(ctx context.Context, key string, contentType string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(int64(len(data))),
	})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

func (s *S3Store) Get(ctx context.Context, key string) ([]byte, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isS3NotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get object: %w", err)
	}
	defer result.Body.Close()
	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("read object: %w", err)
	}
	return data, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil && !isS3NotFound(err) {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

func isS3NotFound(err error) bool {
	var apiError smithy.APIError
	if !errors.As(err, &apiError) {
		return false
	}
	code := apiError.ErrorCode()
	return code == "NoSuchKey" || code == "NotFound"
}
