package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"swiggy-ssh/internal/auth"
)

// --- Mock repo ---

type mockAuthRepo struct {
	account   auth.OAuthAccount
	findErr   error
	upserted  auth.OAuthAccount
	upsertErr error
}

func (r *mockAuthRepo) FindOAuthAccountByUserAndProvider(_ context.Context, _, _ string) (auth.OAuthAccount, error) {
	return r.account, r.findErr
}

func (r *mockAuthRepo) UpsertOAuthAccount(_ context.Context, a auth.OAuthAccount) (auth.OAuthAccount, error) {
	r.upserted = a
	return a, r.upsertErr
}

// --- Mock login code service (minimal, just records calls) ---

type mockLoginSvc struct {
	called bool
}

func (m *mockLoginSvc) IssueLoginCode(_ context.Context, _, _ string) (string, auth.LoginCode, error) {
	m.called = true
	return "TEST-CODE", auth.LoginCode{}, nil
}
func (m *mockLoginSvc) GetLoginCode(_ context.Context, _ string) (auth.LoginCode, error) {
	return auth.LoginCode{Status: auth.LoginCodeStatusPending}, nil
}
func (m *mockLoginSvc) CompleteLoginCode(_ context.Context, _ string) error { return nil }
func (m *mockLoginSvc) CancelLoginCode(_ context.Context, _ string) error   { return nil }

// --- Tests ---

func TestEnsureValidAccountFirstAuth(t *testing.T) {
	repo := &mockAuthRepo{findErr: auth.ErrOAuthAccountNotFound}
	svc := auth.NewAuthService(repo, &mockLoginSvc{})

	result, err := svc.EnsureValidAccount(context.Background(), "user-1", "sess-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsFirstAuth {
		t.Fatal("expected IsFirstAuth=true")
	}
	if result.Account.Status != auth.OAuthAccountStatusActive {
		t.Fatalf("expected active status, got %s", result.Account.Status)
	}
	if repo.upserted.UserID != "user-1" {
		t.Fatalf("expected upserted user-1, got %s", repo.upserted.UserID)
	}
}

func TestEnsureValidAccountReturningValid(t *testing.T) {
	future := time.Now().UTC().Add(1 * time.Hour)
	repo := &mockAuthRepo{
		account: auth.OAuthAccount{
			Status:         auth.OAuthAccountStatusActive,
			TokenExpiresAt: &future,
			AccessToken:    "mock-token-user-1",
		},
	}
	reauthCalled := false
	svc := auth.NewAuthService(repo, &mockLoginSvc{})

	result, err := svc.EnsureValidAccount(context.Background(), "user-1", "sess-1", func(_ context.Context) error {
		reauthCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsFirstAuth || result.WasReauth {
		t.Fatal("expected no first-auth or reauth for valid returning user")
	}
	if reauthCalled {
		t.Fatal("reauth callback must not be called for valid token")
	}
}

func TestEnsureValidAccountExpiredTriggersReauth(t *testing.T) {
	past := time.Now().UTC().Add(-1 * time.Hour)
	repo := &mockAuthRepo{
		account: auth.OAuthAccount{
			Status:      auth.OAuthAccountStatusActive,
			TokenExpiresAt: &past,
			AccessToken: "old-token",
		},
	}
	reauthCalled := false
	svc := auth.NewAuthService(repo, &mockLoginSvc{})

	result, err := svc.EnsureValidAccount(context.Background(), "user-1", "sess-1", func(_ context.Context) error {
		reauthCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reauthCalled {
		t.Fatal("expected reauth callback to be called")
	}
	if !result.WasReauth {
		t.Fatal("expected WasReauth=true")
	}
	if result.Account.Status != auth.OAuthAccountStatusActive {
		t.Fatalf("expected refreshed account to be active, got %s", result.Account.Status)
	}
}

func TestEnsureValidAccountReconnectRequiredTriggersReauth(t *testing.T) {
	repo := &mockAuthRepo{
		account: auth.OAuthAccount{
			Status:      auth.OAuthAccountStatusReconnectRequired,
			AccessToken: "stale-token",
		},
	}
	reauthCalled := false
	svc := auth.NewAuthService(repo, &mockLoginSvc{})

	result, err := svc.EnsureValidAccount(context.Background(), "user-1", "sess-1", func(_ context.Context) error {
		reauthCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reauthCalled {
		t.Fatal("expected reauth for reconnect_required")
	}
	if !result.WasReauth {
		t.Fatal("expected WasReauth=true")
	}
}

func TestEnsureValidAccountRevokedReturnsError(t *testing.T) {
	repo := &mockAuthRepo{
		account: auth.OAuthAccount{
			Status:      auth.OAuthAccountStatusRevoked,
			AccessToken: "revoked-token",
		},
	}
	reauthCalled := false
	svc := auth.NewAuthService(repo, &mockLoginSvc{})

	_, err := svc.EnsureValidAccount(context.Background(), "user-1", "sess-1", func(_ context.Context) error {
		reauthCalled = true
		return nil
	})
	if !errors.Is(err, auth.ErrAccountRevoked) {
		t.Fatalf("expected ErrAccountRevoked, got %v", err)
	}
	if reauthCalled {
		t.Fatal("reauth callback must not be called for revoked account")
	}
}
