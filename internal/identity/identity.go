package identity

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/ssh"
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

// Resolver owns identity resolution rules.
type Resolver struct {
	repo Repository
	now  func() time.Time
}

func NewResolver(repo Repository) *Resolver {
	return &Resolver{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

// ResolveSSHKey resolves or creates an identity from an SSH public key.
func (r *Resolver) ResolveSSHKey(ctx context.Context, client string, key ssh.PublicKey) (SessionIdentity, error) {
	if key == nil {
		return SessionIdentity{}, ErrMissingSSHPublicKey
	}

	fingerprint := ssh.FingerprintSHA256(key)
	resolvedAt := r.now()

	sshIdentity, err := r.repo.FindSSHIdentityByFingerprint(ctx, fingerprint)
	if err == nil {
		return r.resolveExistingSSHIdentity(ctx, client, fingerprint, sshIdentity, resolvedAt)
	}
	if !errors.Is(err, ErrNotFound) {
		return SessionIdentity{}, err
	}

	user, sshIdentity, err := r.repo.CreateUserWithSSHIdentity(
		ctx,
		User{DisplayName: "ssh-user", LastSeenAt: &resolvedAt},
		SSHIdentity{
			PublicKeyFingerprint: fingerprint,
			PublicKey:            string(ssh.MarshalAuthorizedKey(key)),
			LastSeenAt:           &resolvedAt,
		},
	)
	if err != nil {
		if errors.Is(err, ErrSSHIdentityAlreadyExists) {
			existingIdentity, findErr := r.repo.FindSSHIdentityByFingerprint(ctx, fingerprint)
			if findErr != nil {
				return SessionIdentity{}, findErr
			}

			return r.resolveExistingSSHIdentity(ctx, client, fingerprint, existingIdentity, resolvedAt)
		}

		return SessionIdentity{}, err
	}

	return SessionIdentity{Client: client, User: user, SSHIdentity: sshIdentity}, nil
}

func (r *Resolver) resolveExistingSSHIdentity(ctx context.Context, client, fingerprint string, sshIdentity SSHIdentity, resolvedAt time.Time) (SessionIdentity, error) {
	if sshIdentity.RevokedAt != nil {
		return SessionIdentity{}, ErrSSHIdentityRevoked
	}

	if err := r.repo.UpdateSSHIdentityLastSeen(ctx, fingerprint, resolvedAt); err != nil {
		return SessionIdentity{}, err
	}

	if err := r.repo.UpdateUserLastSeen(ctx, sshIdentity.UserID, resolvedAt); err != nil {
		return SessionIdentity{}, err
	}

	user, err := r.repo.FindUserByID(ctx, sshIdentity.UserID)
	if err != nil {
		return SessionIdentity{}, err
	}

	sshIdentity.LastSeenAt = &resolvedAt
	user.LastSeenAt = &resolvedAt

	return SessionIdentity{Client: client, User: user, SSHIdentity: sshIdentity}, nil
}
