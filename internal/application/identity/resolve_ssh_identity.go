package identity

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/ssh"
	domainidentity "swiggy-ssh/internal/domain/identity"
)

type User = domainidentity.User
type SSHIdentity = domainidentity.SSHIdentity
type Repository = domainidentity.Repository
type SessionIdentity = domainidentity.SessionIdentity

var ErrMissingSSHPublicKey = domainidentity.ErrMissingSSHPublicKey
var ErrSSHIdentityRevoked = domainidentity.ErrSSHIdentityRevoked
var ErrNotFound = domainidentity.ErrNotFound
var ErrSSHIdentityAlreadyExists = domainidentity.ErrSSHIdentityAlreadyExists

// ResolveSSHIdentityInput contains the SSH client details needed to resolve identity.
type ResolveSSHIdentityInput struct {
	Client string
	Key    ssh.PublicKey
}

// ResolveSSHIdentityUseCase owns identity resolution rules.
type ResolveSSHIdentityUseCase struct {
	repo Repository
	now  func() time.Time
}

func NewResolveSSHIdentityUseCase(repo Repository) *ResolveSSHIdentityUseCase {
	return &ResolveSSHIdentityUseCase{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

// Execute resolves or creates an identity from an SSH public key.
func (r *ResolveSSHIdentityUseCase) Execute(ctx context.Context, input ResolveSSHIdentityInput) (SessionIdentity, error) {
	if input.Key == nil {
		return SessionIdentity{}, ErrMissingSSHPublicKey
	}

	fingerprint := ssh.FingerprintSHA256(input.Key)
	resolvedAt := r.now()

	sshIdentity, err := r.repo.FindSSHIdentityByFingerprint(ctx, fingerprint)
	if err == nil {
		return r.resolveExistingSSHIdentity(ctx, input.Client, fingerprint, sshIdentity, resolvedAt)
	}
	if !errors.Is(err, ErrNotFound) {
		return SessionIdentity{}, err
	}

	user, sshIdentity, err := r.repo.CreateUserWithSSHIdentity(
		ctx,
		User{DisplayName: "ssh-user", LastSeenAt: &resolvedAt},
		SSHIdentity{
			PublicKeyFingerprint: fingerprint,
			PublicKey:            string(ssh.MarshalAuthorizedKey(input.Key)),
			LastSeenAt:           &resolvedAt,
		},
	)
	if err != nil {
		if errors.Is(err, ErrSSHIdentityAlreadyExists) {
			existingIdentity, findErr := r.repo.FindSSHIdentityByFingerprint(ctx, fingerprint)
			if findErr != nil {
				return SessionIdentity{}, findErr
			}

			return r.resolveExistingSSHIdentity(ctx, input.Client, fingerprint, existingIdentity, resolvedAt)
		}

		return SessionIdentity{}, err
	}

	return SessionIdentity{Client: input.Client, User: user, SSHIdentity: sshIdentity}, nil
}

func (r *ResolveSSHIdentityUseCase) resolveExistingSSHIdentity(ctx context.Context, client, fingerprint string, sshIdentity SSHIdentity, resolvedAt time.Time) (SessionIdentity, error) {
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
