package tui

import (
	"context"
	"io"

	"swiggy-ssh/internal/presentation/tui/instamartflow"
)

type InstamartService = instamartflow.InstamartService

// InstamartPlaceholderView remains for unauthenticated/legacy fallback paths.
// Successful authenticated sessions use InstamartAppView instead.
type InstamartPlaceholderView struct {
	UserID        string
	StatusMessage string
	In            io.Reader
}

func (v InstamartPlaceholderView) Render(ctx context.Context, w io.Writer) error {
	return instamartflow.InstamartPlaceholderView{
		UserID:        v.UserID,
		StatusMessage: v.StatusMessage,
		Viewport:      instamartViewport(ctx),
		In:            v.In,
	}.Render(ctx, w)
}

// InstamartView renders the static Instamart landing screen used by older tests
// and fallback paths that do not have an application service.
type InstamartView struct {
	AddressLabel  string
	AddressLine   string
	CartItemCount int
	StatusMessage string
	In            io.Reader
}

func (v InstamartView) Render(ctx context.Context, w io.Writer) error {
	return instamartflow.InstamartView{
		AddressLabel:  v.AddressLabel,
		AddressLine:   v.AddressLine,
		CartItemCount: v.CartItemCount,
		StatusMessage: v.StatusMessage,
		Viewport:      instamartViewport(ctx),
		In:            v.In,
	}.Render(ctx, w)
}

// InstamartAppView renders the service-backed Instamart flow.
type InstamartAppView struct {
	Service       InstamartService
	UserID        string
	StatusMessage string
	In            io.Reader
}

func (v InstamartAppView) Render(ctx context.Context, w io.Writer) error {
	return instamartflow.InstamartAppView{
		Service:       v.Service,
		UserID:        v.UserID,
		StatusMessage: v.StatusMessage,
		Viewport:      instamartViewport(ctx),
		In:            v.In,
	}.Render(ctx, w)
}

func instamartViewport(ctx context.Context) instamartflow.Viewport {
	viewport := viewportFromContext(ctx)
	return instamartflow.Viewport{Width: viewport.Width, Height: viewport.Height}
}
