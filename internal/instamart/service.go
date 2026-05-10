package instamart

import "context"

// Service defines business operations exposed to the TUI.
type Service interface {
	Health(ctx context.Context) error
}
