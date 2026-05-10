package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	domainauth "swiggy-ssh/internal/domain/auth"
)

type OAuthAccount = domainauth.OAuthAccount
type Repository = domainauth.Repository
type LoginCode = domainauth.LoginCode
type LoginCodeService = domainauth.LoginCodeService
type TokenEncryptor = domainauth.TokenEncryptor

const (
	LoginCodeStatusPending   = domainauth.LoginCodeStatusPending
	LoginCodeStatusCompleted = domainauth.LoginCodeStatusCompleted
	LoginCodeStatusCancelled = domainauth.LoginCodeStatusCancelled

	OAuthAccountStatusActive            = domainauth.OAuthAccountStatusActive
	OAuthAccountStatusExpired           = domainauth.OAuthAccountStatusExpired
	OAuthAccountStatusReconnectRequired = domainauth.OAuthAccountStatusReconnectRequired
	OAuthAccountStatusRevoked           = domainauth.OAuthAccountStatusRevoked
)

var ErrTokenExpired = domainauth.ErrTokenExpired
var ErrTokenReconnectRequired = domainauth.ErrTokenReconnectRequired
var ErrTokenRevoked = domainauth.ErrTokenRevoked
var ErrOAuthAccountNotFound = domainauth.ErrOAuthAccountNotFound
var ErrAccountRevoked = domainauth.ErrAccountRevoked
var ErrLoginCodeNotFound = domainauth.ErrLoginCodeNotFound
var ErrLoginCodeAlreadyUsed = domainauth.ErrLoginCodeAlreadyUsed

var ValidateTokenForUse = domainauth.ValidateTokenForUse

const (
	MockProvider = "swiggy"
	mockTokenTTL = 24 * time.Hour
)

// EnsureValidAccountInput contains the account context needed to validate or establish an OAuth account.
type EnsureValidAccountInput struct {
	UserID         string
	AllowFirstAuth bool
	Reauth         func(ctx context.Context) error
}

// EnsureValidAccountOutput tells the caller what happened so it can render the right terminal message.
type EnsureValidAccountOutput struct {
	Account     OAuthAccount
	IsFirstAuth bool // true when a new account record was created
	WasReauth   bool // true when the user went through a re-auth loop
}

// EnsureValidAccountUseCase orchestrates the OAuth account lifecycle for a terminal session.
// It is client-agnostic: SSH, web, and future clients all call the same service.
type EnsureValidAccountUseCase struct {
	repo Repository
	now  func() time.Time
}

// NewEnsureValidAccountUseCase constructs an EnsureValidAccountUseCase with its dependencies.
func NewEnsureValidAccountUseCase(repo Repository) *EnsureValidAccountUseCase {
	return &EnsureValidAccountUseCase{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

// Execute checks or establishes a valid OAuth account for input.UserID.
//
// Behaviour:
//   - If no account exists and AllowFirstAuth is true → creates a mock active account and returns IsFirstAuth=true.
//   - If no account exists and AllowFirstAuth is false → returns ErrOAuthAccountNotFound.
//   - If account exists and is valid → returns the account as-is.
//   - If account is expired or reconnect_required → calls reauth to issue a new login code,
//     waits for completion (via reauth callback), then refreshes the account record.
//   - If account is revoked → returns ErrAccountRevoked.
//
// input.Reauth is a callback the caller provides to run the re-auth login-code flow.
// It should issue a new login code, show it to the user, poll for completion,
// and return nil on success or an error (including context cancellation) on failure.
func (s *EnsureValidAccountUseCase) Execute(ctx context.Context, input EnsureValidAccountInput) (EnsureValidAccountOutput, error) {
	account, err := s.repo.FindOAuthAccountByUserAndProvider(ctx, input.UserID, MockProvider)
	if err != nil {
		if errors.Is(err, ErrOAuthAccountNotFound) {
			if !input.AllowFirstAuth {
				return EnsureValidAccountOutput{}, ErrOAuthAccountNotFound
			}
			newAccount, createErr := s.createMockAccount(ctx, input.UserID)
			if createErr != nil {
				return EnsureValidAccountOutput{}, fmt.Errorf("create mock oauth account: %w", createErr)
			}
			return EnsureValidAccountOutput{Account: newAccount, IsFirstAuth: true}, nil
		}
		return EnsureValidAccountOutput{}, fmt.Errorf("find oauth account: %w", err)
	}

	if err := ValidateTokenForUse(account, s.now()); err != nil {
		switch {
		case errors.Is(err, ErrTokenRevoked):
			return EnsureValidAccountOutput{}, ErrAccountRevoked
		case errors.Is(err, ErrTokenExpired), errors.Is(err, ErrTokenReconnectRequired):
			if input.Reauth == nil {
				return EnsureValidAccountOutput{}, err
			}
			if reauthErr := input.Reauth(ctx); reauthErr != nil {
				return EnsureValidAccountOutput{}, fmt.Errorf("reauth: %w", reauthErr)
			}
			refreshed, refreshErr := s.refreshMockAccount(ctx, input.UserID, account)
			if refreshErr != nil {
				return EnsureValidAccountOutput{}, fmt.Errorf("refresh mock account after reauth: %w", refreshErr)
			}
			return EnsureValidAccountOutput{Account: refreshed, WasReauth: true}, nil
		default:
			return EnsureValidAccountOutput{}, err
		}
	}

	return EnsureValidAccountOutput{Account: account}, nil
}

// createMockAccount creates a new mock OAuth account for a first-time user.
// The mock token is a placeholder — no real Swiggy credentials.
func (s *EnsureValidAccountUseCase) createMockAccount(ctx context.Context, userID string) (OAuthAccount, error) {
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
func (s *EnsureValidAccountUseCase) refreshMockAccount(ctx context.Context, userID string, existing OAuthAccount) (OAuthAccount, error) {
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
