package swiggy

import "context"

// Client is the provider adapter contract for Swiggy APIs.
type Client interface {
	Ping(ctx context.Context) error
}
