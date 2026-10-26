package storage

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("object not found")

type Store interface {
	Put(ctx context.Context, key string, contentType string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}
