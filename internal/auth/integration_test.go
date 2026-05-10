package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"swiggy-ssh/internal/auth"
	"swiggy-ssh/internal/provider/mock"
)

// repoKey is the composite key for the in-memory auth repo.
type repoKey struct{ userID, provider string }

// intAuthRepo is a simple in-memory repository for integration testing.
type intAuthRepo struct {
	accounts map[repoKey]auth.OAuthAccount
}

func newIntAuthRepo() *intAuthRepo {
	return &intAuthRepo{accounts: make(map[repoKey]auth.OAuthAccount)}
}

func (r *intAuthRepo) UpsertOAuthAccount(_ context.Context, a auth.OAuthAccount) (auth.OAuthAccount, error) {
	if a.UserID == "" {
		return auth.OAuthAccount{}, errors.New("upsert: userID required")
	}
	r.accounts[repoKey{a.UserID, a.Provider}] = a
	return a, nil
}

func (r *intAuthRepo) FindOAuthAccountByUserAndProvider(_ context.Context, userID, provider string) (auth.OAuthAccount, error) {
	a, ok := r.accounts[repoKey{userID, provider}]
	if !ok {
		return auth.OAuthAccount{}, auth.ErrOAuthAccountNotFound
	}
	return a, nil
}

func TestAuthIntegrationFirstTimeUser(t *testing.T) {
	repo := newIntAuthRepo()
	loginSvc := mock.NewMockLoginCodeService(10 * time.Minute)
	svc := auth.NewAuthService(repo, loginSvc)

	result, err := svc.EnsureValidAccount(context.Background(), "user-new", "sess-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsFirstAuth {
		t.Fatal("expected IsFirstAuth for new user")
	}
	if result.Account.Status != auth.OAuthAccountStatusActive {
		t.Fatalf("expected active, got %s", result.Account.Status)
	}
	// Account persisted
	found, err := repo.FindOAuthAccountByUserAndProvider(context.Background(), "user-new", auth.MockProvider)
	if err != nil {
		t.Fatalf("find after first auth: %v", err)
	}
	if found.Status != auth.OAuthAccountStatusActive {
		t.Fatalf("expected active in repo, got %s", found.Status)
	}
}

func TestAuthIntegrationReturningValidUser(t *testing.T) {
	repo := newIntAuthRepo()
	loginSvc := mock.NewMockLoginCodeService(10 * time.Minute)
	svc := auth.NewAuthService(repo, loginSvc)

	// Seed a valid account
	future := time.Now().UTC().Add(2 * time.Hour)
	repo.accounts[repoKey{"user-returning", auth.MockProvider}] = auth.OAuthAccount{
		UserID:         "user-returning",
		Provider:       auth.MockProvider,
		Status:         auth.OAuthAccountStatusActive,
		AccessToken:    "valid-token",
		TokenExpiresAt: &future,
	}

	reauthCalled := false
	result, err := svc.EnsureValidAccount(context.Background(), "user-returning", "sess-2", func(_ context.Context) error {
		reauthCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reauthCalled {
		t.Fatal("reauth must not be called for valid returning user")
	}
	if result.IsFirstAuth || result.WasReauth {
		t.Fatal("expected plain returning user result")
	}
}

func TestAuthIntegrationExpiredUserReauth(t *testing.T) {
	repo := newIntAuthRepo()
	loginSvc := mock.NewMockLoginCodeService(10 * time.Minute)
	svc := auth.NewAuthService(repo, loginSvc)

	// Seed expired account
	past := time.Now().UTC().Add(-1 * time.Hour)
	repo.accounts[repoKey{"user-expired", auth.MockProvider}] = auth.OAuthAccount{
		UserID:         "user-expired",
		Provider:       auth.MockProvider,
		Status:         auth.OAuthAccountStatusActive,
		AccessToken:    "expired-token",
		TokenExpiresAt: &past,
	}

	reauthCount := 0
	result, err := svc.EnsureValidAccount(context.Background(), "user-expired", "sess-3", func(_ context.Context) error {
		reauthCount++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reauthCount != 1 {
		t.Fatalf("expected reauth called once, got %d", reauthCount)
	}
	if !result.WasReauth {
		t.Fatal("expected WasReauth=true")
	}
	if result.Account.Status != auth.OAuthAccountStatusActive {
		t.Fatalf("expected active after reauth, got %s", result.Account.Status)
	}
	// Token refreshed in repo
	found, _ := repo.FindOAuthAccountByUserAndProvider(context.Background(), "user-expired", auth.MockProvider)
	if found.TokenExpiresAt == nil || found.TokenExpiresAt.Before(time.Now().UTC()) {
		t.Fatal("expected refreshed future expiry in repo")
	}
}

func TestAuthIntegrationReconnectRequiredReauth(t *testing.T) {
	repo := newIntAuthRepo()
	loginSvc := mock.NewMockLoginCodeService(10 * time.Minute)
	svc := auth.NewAuthService(repo, loginSvc)

	// Seed account with reconnect_required status (token not wall-clock expired)
	future := time.Now().UTC().Add(2 * time.Hour)
	repo.accounts[repoKey{"user-reconnect", auth.MockProvider}] = auth.OAuthAccount{
		UserID:         "user-reconnect",
		Provider:       auth.MockProvider,
		Status:         auth.OAuthAccountStatusReconnectRequired,
		AccessToken:    "stale-token",
		TokenExpiresAt: &future, // not expired by time, but status forces reauth
	}

	reauthCount := 0
	result, err := svc.EnsureValidAccount(context.Background(), "user-reconnect", "sess-5", func(_ context.Context) error {
		reauthCount++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reauthCount != 1 {
		t.Fatalf("expected reauth called once, got %d", reauthCount)
	}
	if !result.WasReauth {
		t.Fatal("expected WasReauth=true")
	}
	if result.Account.Status != auth.OAuthAccountStatusActive {
		t.Fatalf("expected active after reauth, got %s", result.Account.Status)
	}
}

func TestAuthIntegrationRevokedUserBlocked(t *testing.T) {
	repo := newIntAuthRepo()
	loginSvc := mock.NewMockLoginCodeService(10 * time.Minute)
	svc := auth.NewAuthService(repo, loginSvc)

	repo.accounts[repoKey{"user-revoked", auth.MockProvider}] = auth.OAuthAccount{
		UserID:      "user-revoked",
		Provider:    auth.MockProvider,
		Status:      auth.OAuthAccountStatusRevoked,
		AccessToken: "revoked-token",
	}

	_, err := svc.EnsureValidAccount(context.Background(), "user-revoked", "sess-4", func(_ context.Context) error {
		t.Fatal("reauth must not be called for revoked account")
		return nil
	})
	if !errors.Is(err, auth.ErrAccountRevoked) {
		t.Fatalf("expected ErrAccountRevoked, got %v", err)
	}
}
