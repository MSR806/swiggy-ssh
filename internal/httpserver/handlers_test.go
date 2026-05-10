package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"swiggy-ssh/internal/auth"
)

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testLoginCodeService is a minimal stub implementing auth.LoginCodeService.
// Set completeErr to control what CompleteLoginCode returns.
type testLoginCodeService struct {
	completeErr error
}

func (m *testLoginCodeService) IssueLoginCode(_ context.Context, _, _ string) (string, auth.LoginCode, error) {
	return "", auth.LoginCode{}, nil
}

func (m *testLoginCodeService) GetLoginCode(_ context.Context, _ string) (auth.LoginCode, error) {
	return auth.LoginCode{}, nil
}

func (m *testLoginCodeService) CompleteLoginCode(_ context.Context, _ string) error {
	return m.completeErr
}

func (m *testLoginCodeService) CancelLoginCode(_ context.Context, _ string) error {
	return nil
}

func newTestServer(svc auth.LoginCodeService) *Server {
	return New(":0", newDiscardLogger(), svc)
}

func TestHandleHealthOK(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&testLoginCodeService{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"ok"`) {
		t.Fatalf("expected body to contain ok, got: %s", body)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got: %s", ct)
	}
}

func TestHandleLoginGetRendersForm(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&testLoginCodeService{})
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()

	srv.handleLoginGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Complete Login") {
		t.Fatalf("expected body to contain 'Complete Login', got: %s", body)
	}
	if !strings.Contains(body, "<form") {
		t.Fatalf("expected body to contain '<form', got: %s", body)
	}
}

func TestHandleLoginPostSuccess(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&testLoginCodeService{completeErr: nil})
	form := url.Values{"code": {"ABCD-1234"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.handleLoginPost(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Login complete") {
		t.Fatalf("expected body to contain 'Login complete', got: %s", body)
	}
}

func TestHandleLoginPostExpiredCode(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&testLoginCodeService{completeErr: auth.ErrLoginCodeNotFound})
	form := url.Values{"code": {"ZZZZ-9999"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.handleLoginPost(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "not found or expired") {
		t.Fatalf("expected body to contain 'not found or expired', got: %s", body)
	}
}

func TestHandleLoginPostAlreadyUsed(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&testLoginCodeService{completeErr: auth.ErrLoginCodeAlreadyUsed})
	form := url.Values{"code": {"ABCD-5678"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.handleLoginPost(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "already been used") {
		t.Fatalf("expected body to contain 'already been used', got: %s", body)
	}
}

func TestHandleLoginPostEmptyCode(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&testLoginCodeService{})
	form := url.Values{"code": {""}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.handleLoginPost(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Please enter") {
		t.Fatalf("expected body to contain 'Please enter', got: %s", body)
	}
}
