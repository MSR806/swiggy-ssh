package auth

import (
	"context"
	"errors"
	"time"
)

// Service defines authentication behavior for SSH sessions.
type Service interface {
	Authenticate(subject string) (Identity, error)
}

// Identity represents a successfully authenticated principal.
type Identity struct {
	UserID string
}

// OAuthAccount represents provider linkage and token state.
type OAuthAccount struct {
	ID             string
	UserID         string
	Provider       string
	ProviderUserID *string
	// AccessToken holds the plaintext access token value.
	// The store layer encrypts this at rest (AES-256-GCM) and decrypts on read.
	// Never log, render, or expose this value to clients.
	AccessToken    string
	TokenExpiresAt *time.Time
	Scopes         []string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Repository is the auth persistence boundary.
type Repository interface {
	UpsertOAuthAccount(ctx context.Context, account OAuthAccount) (OAuthAccount, error)
	FindOAuthAccountByUserAndProvider(ctx context.Context, userID, provider string) (OAuthAccount, error)
}

// LoginCode status values.
const (
	LoginCodeStatusPending   = "pending"
	LoginCodeStatusCompleted = "completed"
	LoginCodeStatusCancelled = "cancelled"
	// "expired" is implicit: TTL eviction in Redis; GetLoginCode returns ErrLoginCodeNotFound when key is gone.
)

// LoginCode is the domain record for a short-lived terminal-to-browser auth bridge.
type LoginCode struct {
	CodeHash          string // SHA-256 hex of raw code; raw code is never stored
	UserID            string
	TerminalSessionID string
	Status            string // pending | completed | cancelled
	ExpiresAt         time.Time
	CreatedAt         time.Time
}

// OAuthAccountStatus values.
const (
	OAuthAccountStatusActive            = "active"
	OAuthAccountStatusExpired           = "expired"
	OAuthAccountStatusReconnectRequired = "reconnect_required"
	OAuthAccountStatusRevoked           = "revoked"
)

// ErrTokenExpired is returned when an OAuth token has passed its expiry time.
var ErrTokenExpired = errors.New("oauth token expired")

// ErrTokenReconnectRequired is returned when the account needs re-authentication.
var ErrTokenReconnectRequired = errors.New("oauth account requires reconnect")

// ErrTokenRevoked is returned when the account has been revoked.
var ErrTokenRevoked = errors.New("oauth account revoked")

// TokenEncryptor is the port for symmetric token encryption at rest.
type TokenEncryptor interface {
	// Encrypt returns an opaque ciphertext string safe to store in the database.
	// The plaintext token must never be stored or logged after this call.
	Encrypt(ctx context.Context, plaintext string) (ciphertext string, err error)

	// Decrypt recovers the plaintext token from a stored ciphertext string.
	// Returns an error if the ciphertext is invalid or tampered.
	Decrypt(ctx context.Context, ciphertext string) (plaintext string, err error)
}

// ValidateTokenForUse checks that the account is active and the token is not expired.
// Returns a typed domain error (ErrTokenExpired, ErrTokenReconnectRequired, ErrTokenRevoked)
// or nil when the token is valid for provider calls.
// now is the caller's current time (use s.now() in services, time.Now().UTC() in tests).
func ValidateTokenForUse(account OAuthAccount, now time.Time) error {
	switch account.Status {
	case OAuthAccountStatusRevoked:
		return ErrTokenRevoked
	case OAuthAccountStatusReconnectRequired:
		return ErrTokenReconnectRequired
	case OAuthAccountStatusExpired:
		return ErrTokenExpired
	}
	if account.Status == OAuthAccountStatusActive && account.TokenExpiresAt == nil {
		return ErrTokenReconnectRequired
	}
	// Also check wall-clock expiry regardless of stored status.
	if account.TokenExpiresAt != nil && now.After(*account.TokenExpiresAt) {
		return ErrTokenExpired
	}
	return nil
}

// ErrOAuthAccountNotFound is returned by Repository when no OAuth account exists for the given user/provider pair.
var ErrOAuthAccountNotFound = errors.New("oauth account not found")

// ErrAccountRevoked is surfaced to callers when the OAuth account has been revoked.
var ErrAccountRevoked = errors.New("oauth account has been revoked")

// ErrLoginCodeNotFound is returned when the code key does not exist (expired or never created).
var ErrLoginCodeNotFound = errors.New("login code not found or expired")

// ErrLoginCodeAlreadyUsed is returned when attempting to complete a code that is not pending.
var ErrLoginCodeAlreadyUsed = errors.New("login code already used or cancelled")

// LoginCodeService is the application-layer port for the login-code lifecycle.
type LoginCodeService interface {
	// IssueLoginCode generates a human-readable raw code, hashes it, persists the
	// LoginCode record, and returns the raw code (shown once to the user).
	IssueLoginCode(ctx context.Context, userID, terminalSessionID string) (rawCode string, record LoginCode, err error)

	// GetLoginCode looks up the record by raw code. Returns ErrLoginCodeNotFound if
	// the key has expired or was never issued.
	GetLoginCode(ctx context.Context, rawCode string) (LoginCode, error)

	// CompleteLoginCode atomically marks the code completed. Returns
	// ErrLoginCodeNotFound if expired, ErrLoginCodeAlreadyUsed if not pending.
	CompleteLoginCode(ctx context.Context, rawCode string) error

	// CancelLoginCode marks the code cancelled (user dismissed the TUI prompt).
	// Returns ErrLoginCodeNotFound if expired.
	CancelLoginCode(ctx context.Context, rawCode string) error
}
