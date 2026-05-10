package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"
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

func TestResolveSSHIdentityUnknownIdentityCreatesUserAndBinding(t *testing.T) {
	repo := newTestRepo()
	useCase := NewResolveSSHIdentityUseCase(repo)
	key := newSSHPublicKey(t)

	resolved, err := useCase.Execute(context.Background(), ResolveSSHIdentityInput{Client: "ssh", Key: key})
	if err != nil {
		t.Fatalf("resolve key: %v", err)
	}

	if !repo.createCalled {
		t.Fatal("expected create flow for unknown key")
	}
	if resolved.User.ID == "" || resolved.SSHIdentity.ID == "" {
		t.Fatal("expected created user and ssh identity ids")
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

type concurrentRaceRepo struct {
	mu               sync.Mutex
	identityByFP     map[string]SSHIdentity
	userByID         map[string]User
	nextUserID       int
	nextIdentityID   int
	createInProgress bool
	createWait       chan struct{}
	createDone       chan struct{}
}

func newConcurrentRaceRepo() *concurrentRaceRepo {
	return &concurrentRaceRepo{
		identityByFP: map[string]SSHIdentity{},
		userByID:     map[string]User{},
		createWait:   make(chan struct{}),
		createDone:   make(chan struct{}),
	}
}

func (r *concurrentRaceRepo) CreateUser(context.Context, User) (User, error) {
	panic("unexpected call")
}
func (r *concurrentRaceRepo) CreateSSHIdentity(context.Context, SSHIdentity) (SSHIdentity, error) {
	panic("unexpected call")
}

func (r *concurrentRaceRepo) FindUserByID(_ context.Context, userID string) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.userByID[userID]
	if !ok {
		return User{}, ErrNotFound
	}
	return user, nil
}

func (r *concurrentRaceRepo) UpdateUserLastSeen(_ context.Context, userID string, lastSeenAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user := r.userByID[userID]
	user.LastSeenAt = &lastSeenAt
	r.userByID[userID] = user
	return nil
}

func (r *concurrentRaceRepo) FindSSHIdentityByFingerprint(_ context.Context, fingerprint string) (SSHIdentity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	identity, ok := r.identityByFP[fingerprint]
	if !ok {
		return SSHIdentity{}, ErrNotFound
	}
	return identity, nil
}

func (r *concurrentRaceRepo) UpdateSSHIdentityLastSeen(_ context.Context, fingerprint string, lastSeenAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	identity := r.identityByFP[fingerprint]
	identity.LastSeenAt = &lastSeenAt
	r.identityByFP[fingerprint] = identity
	return nil
}

func (r *concurrentRaceRepo) CreateUserWithSSHIdentity(_ context.Context, user User, sshIdentity SSHIdentity) (User, SSHIdentity, error) {
	r.mu.Lock()
	if !r.createInProgress {
		r.createInProgress = true
		r.mu.Unlock()
		<-r.createWait

		r.mu.Lock()
		defer r.mu.Unlock()
		r.nextUserID++
		r.nextIdentityID++
		user.ID = "u-concurrent"
		user.CreatedAt = time.Now().UTC()
		sshIdentity.ID = "i-concurrent"
		sshIdentity.UserID = user.ID
		sshIdentity.FirstSeenAt = time.Now().UTC()
		r.userByID[user.ID] = user
		r.identityByFP[sshIdentity.PublicKeyFingerprint] = sshIdentity
		close(r.createDone)
		return user, sshIdentity, nil
	}
	r.mu.Unlock()
	<-r.createDone

	return User{}, SSHIdentity{}, ErrSSHIdentityAlreadyExists
}

func TestResolveSSHIdentityConcurrentUnknownKeyRaceResolvesSameIdentity(t *testing.T) {
	repo := newConcurrentRaceRepo()
	useCase := NewResolveSSHIdentityUseCase(repo)
	key := newSSHPublicKey(t)

	type result struct {
		identity SessionIdentity
		err      error
	}

	results := make(chan result, 2)
	go func() {
		identity, err := useCase.Execute(context.Background(), ResolveSSHIdentityInput{Client: "ssh", Key: key})
		results <- result{identity: identity, err: err}
	}()
	go func() {
		identity, err := useCase.Execute(context.Background(), ResolveSSHIdentityInput{Client: "ssh", Key: key})
		results <- result{identity: identity, err: err}
	}()

	close(repo.createWait)

	first := <-results
	second := <-results

	if first.err != nil {
		t.Fatalf("first resolve failed: %v", first.err)
	}
	if second.err != nil {
		t.Fatalf("second resolve failed: %v", second.err)
	}

	if first.identity.User.ID != second.identity.User.ID {
		t.Fatalf("expected same user, got %s and %s", first.identity.User.ID, second.identity.User.ID)
	}
	if first.identity.SSHIdentity.ID != second.identity.SSHIdentity.ID {
		t.Fatalf("expected same ssh identity, got %s and %s", first.identity.SSHIdentity.ID, second.identity.SSHIdentity.ID)
	}
}
