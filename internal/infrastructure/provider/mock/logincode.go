package mock

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"swiggy-ssh/internal/domain/auth"
)

// MockLoginCodeService is an in-memory implementation of auth.LoginCodeService for tests.
type MockLoginCodeService struct {
	mu           sync.Mutex
	records      map[string]*mockRecord // keyed by rawCode
	ttl          time.Duration
	now          func() time.Time
	generateCode func() (string, error)
}

type mockRecord struct {
	record auth.LoginCode
}

func NewMockLoginCodeService(ttl time.Duration) *MockLoginCodeService {
	return &MockLoginCodeService{
		records:      make(map[string]*mockRecord),
		ttl:          ttl,
		now:          func() time.Time { return time.Now().UTC() },
		generateCode: defaultGenerateCode,
	}
}

func defaultGenerateCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	out := make([]byte, 8)
	for i, v := range b {
		out[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(out[:4]) + "-" + string(out[4:]), nil
}

func mockHashCode(rawCode string) string {
	h := sha256.Sum256([]byte(rawCode))
	return hex.EncodeToString(h[:])
}

func (m *MockLoginCodeService) IssueLoginCode(ctx context.Context, userID, terminalSessionID string) (string, auth.LoginCode, error) {
	rawCode, err := m.generateCode()
	if err != nil {
		return "", auth.LoginCode{}, err
	}

	now := m.now()
	record := auth.LoginCode{
		CodeHash:          mockHashCode(rawCode),
		UserID:            userID,
		TerminalSessionID: terminalSessionID,
		Status:            auth.LoginCodeStatusPending,
		ExpiresAt:         now.Add(m.ttl),
		CreatedAt:         now,
	}

	m.mu.Lock()
	m.records[rawCode] = &mockRecord{record: record}
	m.mu.Unlock()

	return rawCode, record, nil
}

func (m *MockLoginCodeService) GetLoginCode(ctx context.Context, rawCode string) (auth.LoginCode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.records[rawCode]
	if !ok {
		return auth.LoginCode{}, auth.ErrLoginCodeNotFound
	}
	if m.now().After(r.record.ExpiresAt) {
		return auth.LoginCode{}, auth.ErrLoginCodeNotFound
	}
	return r.record, nil
}

func (m *MockLoginCodeService) CompleteLoginCode(ctx context.Context, rawCode string) error {
	return m.transition(rawCode, auth.LoginCodeStatusPending, auth.LoginCodeStatusCompleted)
}

func (m *MockLoginCodeService) CancelLoginCode(ctx context.Context, rawCode string) error {
	return m.transition(rawCode, auth.LoginCodeStatusPending, auth.LoginCodeStatusCancelled)
}

func (m *MockLoginCodeService) transition(rawCode, fromStatus, toStatus string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.records[rawCode]
	if !ok {
		return auth.ErrLoginCodeNotFound
	}
	if m.now().After(r.record.ExpiresAt) {
		return auth.ErrLoginCodeNotFound
	}
	if r.record.Status != fromStatus {
		return auth.ErrLoginCodeAlreadyUsed
	}
	r.record.Status = toStatus
	return nil
}

// SetNow replaces the clock function for deterministic tests.
func (m *MockLoginCodeService) SetNow(f func() time.Time) {
	m.mu.Lock()
	m.now = f
	m.mu.Unlock()
}

// SetGenerateCode replaces the code generator for deterministic tests.
func (m *MockLoginCodeService) SetGenerateCode(f func() (string, error)) {
	m.mu.Lock()
	m.generateCode = f
	m.mu.Unlock()
}

// Compile-time interface assertion.
var _ auth.LoginCodeService = (*MockLoginCodeService)(nil)
