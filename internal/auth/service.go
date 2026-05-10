package auth

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	MockProvider = "swiggy"
	mockTokenTTL = 24 * time.Hour
)

// AccountResult is returned by EnsureValidAccount to tell the caller
// what happened so it can render the right terminal message.
type AccountResult struct {
	Account     OAuthAccount
	IsFirstAuth bool // true when a new account record was created
	WasReauth   bool // true when the user went through a re-auth loop
}

// AuthService orchestrates the OAuth account lifecycle for a terminal session.
// It is client-agnostic: SSH, web, and future clients all call the same service.
type AuthService struct {
	repo     Repository
	loginSvc LoginCodeService
	now      func() time.Time
}

// NewAuthService constructs an AuthService with its dependencies.
func NewAuthService(repo Repository, loginSvc LoginCodeService) *AuthService {
	return &AuthService{
		repo:     repo,
		loginSvc: loginSvc,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// EnsureValidAccount checks or establishes a valid OAuth account for userID.
//
// Behaviour:
//   - If no account exists (first login) → creates a mock active account and returns IsFirstAuth=true.
//   - If account exists and is valid → returns the account as-is.
//   - If account is expired or reconnect_required → calls reauth to issue a new login code,
//     waits for completion (via reauth callback), then refreshes the account record.
//   - If account is revoked → returns ErrAccountRevoked.
//
// reauth is a callback the caller provides to run the re-auth login-code flow.
// It should issue a new login code, show it to the user, poll for completion,
// and return nil on success or an error (including context cancellation) on failure.
func (s *AuthService) EnsureValidAccount(
	ctx context.Context,
	userID string,
	terminalSessionID string,
	reauth func(ctx context.Context) error,
) (AccountResult, error) {
	account, err := s.repo.FindOAuthAccountByUserAndProvider(ctx, userID, MockProvider)
	if err != nil {
		if errors.Is(err, ErrOAuthAccountNotFound) {
			newAccount, createErr := s.createMockAccount(ctx, userID)
			if createErr != nil {
				return AccountResult{}, fmt.Errorf("create mock oauth account: %w", createErr)
			}
			return AccountResult{Account: newAccount, IsFirstAuth: true}, nil
		}
		return AccountResult{}, fmt.Errorf("find oauth account: %w", err)
	}

	if err := ValidateTokenForUse(account, s.now()); err != nil {
		switch {
		case errors.Is(err, ErrTokenRevoked):
			return AccountResult{}, ErrAccountRevoked
		case errors.Is(err, ErrTokenExpired), errors.Is(err, ErrTokenReconnectRequired):
			if reauth == nil {
				return AccountResult{}, err
			}
			if reauthErr := reauth(ctx); reauthErr != nil {
				return AccountResult{}, fmt.Errorf("reauth: %w", reauthErr)
			}
			refreshed, refreshErr := s.refreshMockAccount(ctx, userID, account)
			if refreshErr != nil {
				return AccountResult{}, fmt.Errorf("refresh mock account after reauth: %w", refreshErr)
			}
			return AccountResult{Account: refreshed, WasReauth: true}, nil
		default:
			return AccountResult{}, err
		}
	}

	return AccountResult{Account: account}, nil
}

// createMockAccount creates a new mock OAuth account for a first-time user.
// The mock token is a placeholder — no real Swiggy credentials.
func (s *AuthService) createMockAccount(ctx context.Context, userID string) (OAuthAccount, error) {
	now := s.now()
	expiresAt := now.Add(mockTokenTTL)

	return s.repo.UpsertOAuthAccount(ctx, OAuthAccount{
		UserID:         userID,
		Provider:       MockProvider,
		AccessToken:    mockAccessToken(userID),
		TokenExpiresAt: &expiresAt,
		Scopes:         []string{"profile:read"},
		Status:         OAuthAccountStatusActive,
	})
}

// refreshMockAccount updates an existing account with a new mock token after re-auth.
func (s *AuthService) refreshMockAccount(ctx context.Context, userID string, existing OAuthAccount) (OAuthAccount, error) {
	now := s.now()
	expiresAt := now.Add(mockTokenTTL)

	existing.AccessToken = mockAccessToken(userID)
	existing.TokenExpiresAt = &expiresAt
	existing.Status = OAuthAccountStatusActive
	return s.repo.UpsertOAuthAccount(ctx, existing)
}

// mockAccessToken returns the placeholder token value for a given userID.
// Mock tokens are never real Swiggy credentials.
func mockAccessToken(userID string) string {
	return fmt.Sprintf("mock-token-%s", userID)
}
