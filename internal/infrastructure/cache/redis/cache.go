package redis

import "context"

// Cache is a key-value boundary for short-lived app data.
type Cache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string) error
}
