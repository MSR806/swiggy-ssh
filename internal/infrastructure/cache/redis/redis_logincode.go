package redis

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"swiggy-ssh/internal/domain/auth"

	"github.com/redis/go-redis/v9"
)

// RedisLoginCodeService implements auth.LoginCodeService backed by Redis.
// Keys: "logincode:<sha256-hex-of-raw-code>"
// Value: JSON-encoded loginCodeRecord
// TTL: set on IssueLoginCode; key eviction = expiry.
type RedisLoginCodeService struct {
	client *redis.Client
	ttl    time.Duration
	now    func() time.Time
}

// loginCodeRecord is what is stored in Redis (never contains raw code).
type loginCodeRecord struct {
	CodeHash          string    `json:"code_hash"`
	UserID            string    `json:"user_id"`
	TerminalSessionID string    `json:"terminal_session_id"`
	Status            string    `json:"status"`
	ExpiresAt         time.Time `json:"expires_at"`
	CreatedAt         time.Time `json:"created_at"`
}

func NewRedisLoginCodeService(client *redis.Client, ttl time.Duration) *RedisLoginCodeService {
	return &RedisLoginCodeService{
		client: client,
		ttl:    ttl,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// hashCode returns the SHA-256 hex digest of the raw code.
// Raw code is never stored — only this digest is persisted.
func hashCode(rawCode string) string {
	h := sha256.Sum256([]byte(rawCode))
	return hex.EncodeToString(h[:])
}

// redisKey returns the Redis key for a given raw code.
func redisKey(rawCode string) string {
	return "logincode:" + hashCode(rawCode)
}

// generateRawCode produces a human-readable 8-character alphanumeric code
// formatted as XXXX-XXXX (e.g. "XK7M-P2NQ").
func generateRawCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no O/0/I/1 to avoid confusion
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

func (s *RedisLoginCodeService) IssueLoginCode(ctx context.Context, userID, terminalSessionID string) (string, auth.LoginCode, error) {
	rawCode, err := generateRawCode()
	if err != nil {
		return "", auth.LoginCode{}, err
	}

	now := s.now()
	record := loginCodeRecord{
		CodeHash:          hashCode(rawCode),
		UserID:            userID,
		TerminalSessionID: terminalSessionID,
		Status:            auth.LoginCodeStatusPending,
		ExpiresAt:         now.Add(s.ttl),
		CreatedAt:         now,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return "", auth.LoginCode{}, fmt.Errorf("marshal login code: %w", err)
	}

	key := redisKey(rawCode)
	if err := s.client.Set(ctx, key, data, s.ttl).Err(); err != nil {
		return "", auth.LoginCode{}, fmt.Errorf("store login code: %w", err)
	}

	return rawCode, toLoginCode(record), nil
}

func (s *RedisLoginCodeService) GetLoginCode(ctx context.Context, rawCode string) (auth.LoginCode, error) {
	record, err := s.loadRecord(ctx, rawCode)
	if err != nil {
		return auth.LoginCode{}, err
	}
	return toLoginCode(record), nil
}

func (s *RedisLoginCodeService) CompleteLoginCode(ctx context.Context, rawCode string) error {
	return s.transition(ctx, rawCode, auth.LoginCodeStatusPending, auth.LoginCodeStatusCompleted)
}

func (s *RedisLoginCodeService) CancelLoginCode(ctx context.Context, rawCode string) error {
	return s.transition(ctx, rawCode, auth.LoginCodeStatusPending, auth.LoginCodeStatusCancelled)
}

// loadRecord fetches and unmarshals the record for rawCode.
func (s *RedisLoginCodeService) loadRecord(ctx context.Context, rawCode string) (loginCodeRecord, error) {
	key := redisKey(rawCode)
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return loginCodeRecord{}, auth.ErrLoginCodeNotFound
		}
		return loginCodeRecord{}, fmt.Errorf("get login code: %w", err)
	}

	var record loginCodeRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return loginCodeRecord{}, fmt.Errorf("unmarshal login code: %w", err)
	}
	return record, nil
}

// transition atomically reads, validates status, updates, and re-stores.
// Uses a Lua script for atomicity: read-check-write in a single round-trip.
func (s *RedisLoginCodeService) transition(ctx context.Context, rawCode, fromStatus, toStatus string) error {
	// Lua script: atomically load, check status, update if matching.
	// KEYS[1] = redis key
	// ARGV[1] = expected current status (fromStatus)
	// ARGV[2] = new status (toStatus)
	// Returns: 0 = key not found, 1 = success, 2 = wrong status
	const luaScript = `
local data = redis.call('GET', KEYS[1])
if not data then return 0 end
local rec = cjson.decode(data)
if rec.status ~= ARGV[1] then return 2 end
rec.status = ARGV[2]
local pttl = redis.call('PTTL', KEYS[1])
if pttl > 0 then
  redis.call('SET', KEYS[1], cjson.encode(rec), 'PX', pttl)
elseif pttl == -1 then
  -- key has no TTL (unexpected; IssueLoginCode always sets one) — preserve state
  redis.call('SET', KEYS[1], cjson.encode(rec))
else
  -- pttl == -2: key vanished between GET and SET (race) — treat as not found
  return 0
end
return 1
`
	key := redisKey(rawCode)
	result, err := s.client.Eval(ctx, luaScript, []string{key}, fromStatus, toStatus).Int()
	if err != nil {
		return fmt.Errorf("transition login code: %w", err)
	}
	switch result {
	case 0:
		return auth.ErrLoginCodeNotFound
	case 2:
		return auth.ErrLoginCodeAlreadyUsed
	}
	return nil
}

func toLoginCode(r loginCodeRecord) auth.LoginCode {
	return auth.LoginCode{
		CodeHash:          r.CodeHash,
		UserID:            r.UserID,
		TerminalSessionID: r.TerminalSessionID,
		Status:            r.Status,
		ExpiresAt:         r.ExpiresAt,
		CreatedAt:         r.CreatedAt,
	}
}

// Compile-time interface assertion.
var _ auth.LoginCodeService = (*RedisLoginCodeService)(nil)
