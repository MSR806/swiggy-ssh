package mock

import "context"

// Client is a local development provider implementation boundary.
type Client interface {
	Ping(ctx context.Context) error
}
