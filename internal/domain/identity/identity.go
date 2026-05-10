package identity

import (
	"context"
	"errors"
	"time"
)

var ErrMissingSSHPublicKey = errors.New("identity: missing ssh public key")
var ErrSSHIdentityRevoked = errors.New("identity: ssh identity revoked")
var ErrNotFound = errors.New("identity: not found")
var ErrSSHIdentityAlreadyExists = errors.New("identity: ssh identity already exists")

// User models an authenticated user.
type User struct {
	ID          string
	DisplayName string
	Email       *string
	CreatedAt   time.Time
	LastSeenAt  *time.Time
}

// SSHIdentity maps a public key fingerprint to a user.
type SSHIdentity struct {
	ID                   string
	UserID               string
	PublicKeyFingerprint string
	PublicKey            string
	Label                *string
	FirstSeenAt          time.Time
	LastSeenAt           *time.Time
	RevokedAt            *time.Time
}

// Repository is the identity persistence boundary.
type Repository interface {
	CreateUser(ctx context.Context, user User) (User, error)
	FindUserByID(ctx context.Context, userID string) (User, error)
	UpdateUserLastSeen(ctx context.Context, userID string, lastSeenAt time.Time) error

	CreateSSHIdentity(ctx context.Context, sshIdentity SSHIdentity) (SSHIdentity, error)
	FindSSHIdentityByFingerprint(ctx context.Context, fingerprint string) (SSHIdentity, error)
	UpdateSSHIdentityLastSeen(ctx context.Context, fingerprint string, lastSeenAt time.Time) error

	CreateUserWithSSHIdentity(ctx context.Context, user User, sshIdentity SSHIdentity) (User, SSHIdentity, error)
}

// SessionIdentity is the resolved principal for an incoming client identity.
type SessionIdentity struct {
	Client      string
	User        User
	SSHIdentity SSHIdentity
}

const (
	ClientProtocolSSH                 = "ssh"
	ScreenSSHSessionPlaceholder       = "ssh_session_placeholder"
	SelectedAddressIDUnsetPlaceholder = "unselected"
)

type TerminalSession struct {
	ID                string
	Client            string
	ClientSessionID   string
	UserID            *string
	SSHIdentityID     *string
	SSHFingerprint    *string
	CurrentScreen     string
	SelectedAddressID *string
	CreatedAt         time.Time
	LastSeenAt        *time.Time
	EndedAt           *time.Time
}

type SessionRepository interface {
	CreateTerminalSession(ctx context.Context, session TerminalSession) (TerminalSession, error)
	MarkTerminalSessionEnded(ctx context.Context, sessionID string, endedAt time.Time) error
}
