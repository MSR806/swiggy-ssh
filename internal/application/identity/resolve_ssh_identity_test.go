package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type testRepo struct {
	userByID        map[string]User
	identityByFP    map[string]SSHIdentity
	createCalled    bool
	updatedUserID   string
	updatedFP       string
	updatedLastSeen time.Time
}

func newTestRepo() *testRepo {
	return &testRepo{userByID: map[string]User{}, identityByFP: map[string]SSHIdentity{}}
}

func (r *testRepo) CreateUser(context.Context, User) (User, error) { panic("unexpected call") }
func (r *testRepo) CreateSSHIdentity(context.Context, SSHIdentity) (SSHIdentity, error) {
	panic("unexpected call")
}

func (r *testRepo) FindUserByID(_ context.Context, userID string) (User, error) {
	user, ok := r.userByID[userID]
	if !ok {
		return User{}, ErrNotFound
	}
	return user, nil
}

func (r *testRepo) UpdateUserLastSeen(_ context.Context, userID string, lastSeenAt time.Time) error {
	r.updatedUserID = userID
	r.updatedLastSeen = lastSeenAt
	user := r.userByID[userID]
	user.LastSeenAt = &lastSeenAt
	r.userByID[userID] = user
	return nil
}

func (r *testRepo) FindSSHIdentityByFingerprint(_ context.Context, fingerprint string) (SSHIdentity, error) {
	identity, ok := r.identityByFP[fingerprint]
	if !ok {
		return SSHIdentity{}, ErrNotFound
	}
	return identity, nil
}

func (r *testRepo) UpdateSSHIdentityLastSeen(_ context.Context, fingerprint string, lastSeenAt time.Time) error {
	r.updatedFP = fingerprint
	r.updatedLastSeen = lastSeenAt
	identity := r.identityByFP[fingerprint]
	identity.LastSeenAt = &lastSeenAt
	r.identityByFP[fingerprint] = identity
	return nil
}

func (r *testRepo) CreateUserWithSSHIdentity(_ context.Context, user User, sshIdentity SSHIdentity) (User, SSHIdentity, error) {
	r.createCalled = true
	if user.ID == "" {
		user.ID = "new-user"
	}
	now := time.Now().UTC()
	user.CreatedAt = now
	sshIdentity.ID = "new-identity"
	sshIdentity.UserID = user.ID
	sshIdentity.FirstSeenAt = now
	r.userByID[user.ID] = user
	r.identityByFP[sshIdentity.PublicKeyFingerprint] = sshIdentity
	return user, sshIdentity, nil
}

func newSSHPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	return signer.PublicKey()
}

func TestResolveSSHIdentityFoundIdentity(t *testing.T) {
	repo := newTestRepo()
	useCase := NewResolveSSHIdentityUseCase(repo)
	fixedNow := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	useCase.now = func() time.Time { return fixedNow }

	key := newSSHPublicKey(t)
	fingerprint := ssh.FingerprintSHA256(key)
	repo.userByID["u1"] = User{ID: "u1", DisplayName: "Existing"}
	repo.identityByFP[fingerprint] = SSHIdentity{ID: "i1", UserID: "u1", PublicKeyFingerprint: fingerprint}

	resolved, err := useCase.Execute(context.Background(), ResolveSSHIdentityInput{Client: "ssh", Key: key})
	if err != nil {
		t.Fatalf("resolve key: %v", err)
	}

	if resolved.User.ID != "u1" {
		t.Fatalf("expected user u1, got %s", resolved.User.ID)
	}
	if repo.updatedFP != fingerprint {
		t.Fatalf("expected fingerprint update %s, got %s", fingerprint, repo.updatedFP)
	}
	if repo.updatedUserID != "u1" {
		t.Fatalf("expected user last-seen update for u1, got %s", repo.updatedUserID)
	}
	if !repo.updatedLastSeen.Equal(fixedNow) {
		t.Fatalf("expected updated last-seen %v, got %v", fixedNow, repo.updatedLastSeen)
	}
}

func TestResolveSSHIdentityUnknownIdentityReturnsNotFound(t *testing.T) {
	repo := newTestRepo()
	useCase := NewResolveSSHIdentityUseCase(repo)
	key := newSSHPublicKey(t)

	_, err := useCase.Execute(context.Background(), ResolveSSHIdentityInput{Client: "ssh", Key: key})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if repo.createCalled {
		t.Fatal("unknown key must not create user or ssh identity")
	}
}

func TestResolveSSHIdentityRevokedIdentityRejected(t *testing.T) {
	repo := newTestRepo()
	useCase := NewResolveSSHIdentityUseCase(repo)
	key := newSSHPublicKey(t)
	fingerprint := ssh.FingerprintSHA256(key)
	revokedAt := time.Now().UTC()
	repo.identityByFP[fingerprint] = SSHIdentity{ID: "i1", UserID: "u1", PublicKeyFingerprint: fingerprint, RevokedAt: &revokedAt}

	_, err := useCase.Execute(context.Background(), ResolveSSHIdentityInput{Client: "ssh", Key: key})
	if !errors.Is(err, ErrSSHIdentityRevoked) {
		t.Fatalf("expected ErrSSHIdentityRevoked, got %v", err)
	}
}

func TestResolveSSHIdentityMissingKeyRejected(t *testing.T) {
	repo := newTestRepo()
	useCase := NewResolveSSHIdentityUseCase(repo)

	_, err := useCase.Execute(context.Background(), ResolveSSHIdentityInput{Client: "ssh"})
	if !errors.Is(err, ErrMissingSSHPublicKey) {
		t.Fatalf("expected ErrMissingSSHPublicKey, got %v", err)
	}
}
