package tui_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"swiggy-ssh/internal/application/auth"
	"swiggy-ssh/internal/presentation/tui"
)

// renderCtx returns a context with a short timeout suitable for interactive views.
// Interactive Bubbletea models block until ctx.Done() fires; the timeout ensures
// the test program exits promptly.
func renderCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 200*time.Millisecond)
}

// --- HomeView ---

func TestHomeViewRendersMenuItems(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.HomeView{}
	var buf bytes.Buffer
	if err := v.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"swiggy.ssh",
		"Instamart",
		"Food",
		"Reorder usuals",
		"coming soon",
		"j/k move",
		"● Connected SSH",
		"███████╗", // first row of ASCII logo
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in HomeView output", want)
		}
	}
}

func TestHomeViewUsesSwiggyOrange(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	var buf bytes.Buffer
	if err := (tui.HomeView{}).Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(buf.String(), "\x1b[38;2;252;128;25m") {
		t.Fatal("expected home view to render Swiggy orange ANSI color")
	}
}

func TestHomeViewCentersInViewport(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	ctx = tui.WithViewport(ctx, tui.Viewport{Width: 100, Height: 30})

	var buf bytes.Buffer
	if err := (tui.HomeView{}).Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "\r\n          ┌") {
		t.Fatal("expected home view to be vertically and horizontally centered")
	}
}

func TestInstamartPlaceholderViewUsesInput(t *testing.T) {
	var buf bytes.Buffer
	err := tui.InstamartPlaceholderView{
		UserID: "user-1",
		In:     strings.NewReader("q"),
	}.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), "Instamart") {
		t.Fatal("expected Instamart screen output")
	}
}

// --- LoginWaitingView ---

func TestLoginWaitingViewRendersCodeBox(t *testing.T) {
	// LoginWaitingView is static — Init returns tea.Quit immediately.
	v := tui.LoginWaitingView{LoginURL: "http://localhost:8080/login", RawCode: "ABCD-1234"}
	var buf bytes.Buffer
	if err := v.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"http://localhost:8080/login",
		"ABCD-1234",
		"┌────────────────────┐", // code box top border
		"Waiting for login",
		"not connected",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in LoginWaitingView output", want)
		}
	}
}

func TestLoginWaitingViewCodeAppearsOnce(t *testing.T) {
	v := tui.LoginWaitingView{LoginURL: "http://localhost:8080/login", RawCode: "XXXX-9999"}
	var buf bytes.Buffer
	_ = v.Render(context.Background(), &buf)
	out := buf.String()
	count := strings.Count(out, "XXXX-9999")
	if count != 1 {
		t.Fatalf("expected code to appear exactly once, got %d times", count)
	}
}

// --- LoginSuccessView ---

func TestLoginSuccessViewShowsSignedInAs(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.LoginSuccessView{
		IsFirstAuth: true,
		DisplayName: "Alice",
		Email:       "alice@example.com",
	}
	var buf bytes.Buffer
	if err := v.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"✓ Swiggy account connected",
		"Alice",
		"alice@example.com",
		"Instamart",
		"j/k move",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in LoginSuccessView output", want)
		}
	}
	// AccessToken must never appear
	if strings.Contains(out, v.Account.AccessToken) && v.Account.AccessToken != "" {
		t.Fatal("access token must not appear in TUI output")
	}
}

func TestLoginSuccessViewEmptyDisplayName(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.LoginSuccessView{DisplayName: "", Email: ""}
	var buf bytes.Buffer
	_ = v.Render(ctx, &buf)
	out := buf.String()

	if !strings.Contains(out, "Unknown") {
		t.Fatal("expected 'Unknown' when DisplayName is empty")
	}
	if !strings.Contains(out, "(no email)") {
		t.Fatal("expected '(no email)' when Email is empty")
	}
}

func TestLoginSuccessViewFirstAuth(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.LoginSuccessView{IsFirstAuth: true, DisplayName: "Sujith", Email: "sujith@example.com"}
	var buf bytes.Buffer
	_ = v.Render(ctx, &buf)
	if !strings.Contains(buf.String(), "✓ Swiggy account connected") {
		t.Fatal("expected connected message in first-auth LoginSuccessView")
	}
}

func TestLoginSuccessViewWasReauth(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.LoginSuccessView{WasReauth: true, DisplayName: "Sujith", Email: "sujith@example.com"}
	var buf bytes.Buffer
	_ = v.Render(ctx, &buf)
	if !strings.Contains(buf.String(), "✓ Swiggy account connected") {
		t.Fatal("expected connected message in reauth LoginSuccessView")
	}
}

func TestLoginSuccessViewWelcomeBack(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.LoginSuccessView{DisplayName: "Sujith", Email: "sujith@example.com"}
	var buf bytes.Buffer
	_ = v.Render(ctx, &buf)
	if !strings.Contains(buf.String(), "✓ Swiggy account connected") {
		t.Fatal("expected connected message in LoginSuccessView")
	}
}

// --- InstamartView ---

func TestInstamartViewRendersAddress(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.InstamartView{
		AddressLabel:  "Work",
		AddressLine:   "Koramangala, Bengaluru",
		CartItemCount: 3,
	}
	var buf bytes.Buffer
	if err := v.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"Work",
		"Koramangala",
		"3 items",
		"Search products",
		"/ search",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in InstamartView output", want)
		}
	}
}

// --- InstamartPlaceholderView ---

func TestInstamartPlaceholderViewDelegates(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.InstamartPlaceholderView{UserID: "user-1"}
	var buf bytes.Buffer
	_ = v.Render(ctx, &buf)
	out := buf.String()

	for _, want := range []string{
		"Instamart",
		"Search products",
		"Home", // default address label
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in InstamartPlaceholderView output", want)
		}
	}
}

// --- AccountHomeView ---

func TestAccountHomeViewShowsFingerprint(t *testing.T) {
	// AccountHomeView is static (plain fmt.Fprint, no tea.Program).
	expiresAt := time.Now().Add(time.Hour)
	v := tui.AccountHomeView{
		SSHFingerprint: "SHA256:abcdef",
		Account: auth.OAuthAccount{
			AccessToken:    "super-secret-token-abc123",
			Status:         auth.OAuthAccountStatusActive,
			TokenExpiresAt: &expiresAt,
		},
	}
	var buf bytes.Buffer
	_ = v.Render(context.Background(), &buf)
	out := buf.String()
	if !strings.Contains(out, "SHA256:abcdef") {
		t.Fatal("expected fingerprint in output")
	}
	if !strings.Contains(out, "active") {
		t.Fatal("expected active status in output")
	}
	// AccessToken must not appear in TUI output
	if strings.Contains(out, v.Account.AccessToken) {
		t.Fatal("access token must not appear in TUI output")
	}
}

// --- RevokedView ---

func TestRevokedViewRenders(t *testing.T) {
	v := tui.RevokedView{}
	var buf bytes.Buffer
	_ = v.Render(context.Background(), &buf)
	if !strings.Contains(buf.String(), "revoked") {
		t.Fatal("expected revoked message")
	}
}

// --- ReconnectView ---

func TestReconnectViewRendersCode(t *testing.T) {
	// ReconnectView is inline plain text — no tea.Program.
	v := tui.ReconnectView{RawCode: "RECO-AUTH"}
	var buf bytes.Buffer
	if err := v.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "RECO-AUTH") {
		t.Fatal("expected code in reconnect view output")
	}
	// code should appear exactly once
	if strings.Count(out, "RECO-AUTH") != 1 {
		t.Fatalf("expected code to appear exactly once, got %d", strings.Count(out, "RECO-AUTH"))
	}
}
