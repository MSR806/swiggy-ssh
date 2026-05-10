package crypto_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"swiggy-ssh/internal/domain/auth"
	"swiggy-ssh/internal/infrastructure/crypto"
)

// Compile-time interface assertions.
var _ auth.TokenEncryptor = (*crypto.AESGCMEncryptor)(nil)
var _ auth.TokenEncryptor = crypto.NoOpEncryptor{}

func testKey() []byte {
	// 32-byte key for tests — fixed, never used in production.
	return []byte("swiggy-ssh-test-key-32bytes!!!!!")
}

func TestAESGCMEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := crypto.NewAESGCMEncryptor(testKey())
	if err != nil {
		t.Fatalf("new encryptor: %v", err)
	}
	ctx := context.Background()
	plaintext := "super-secret-access-token"

	ciphertext, err := enc.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if ciphertext == plaintext {
		t.Fatal("ciphertext must not equal plaintext")
	}
	if strings.Contains(ciphertext, plaintext) {
		t.Fatal("plaintext must not appear in ciphertext")
	}

	got, err := enc.Decrypt(ctx, ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, got)
	}
}

func TestAESGCMEncryptProducesUniqueCiphertexts(t *testing.T) {
	enc, err := crypto.NewAESGCMEncryptor(testKey())
	if err != nil {
		t.Fatalf("new encryptor: %v", err)
	}
	ctx := context.Background()
	const plaintext = "same-token"
	seen := make(map[string]bool)
	for i := 0; i < 5; i++ {
		ct, err := enc.Encrypt(ctx, plaintext)
		if err != nil {
			t.Fatalf("encrypt %d: %v", i, err)
		}
		if seen[ct] {
			t.Fatal("duplicate ciphertext detected — nonce reuse suspected")
		}
		seen[ct] = true
	}
}

func TestAESGCMDecryptTamperedCiphertextFails(t *testing.T) {
	enc, err := crypto.NewAESGCMEncryptor(testKey())
	if err != nil {
		t.Fatalf("new encryptor: %v", err)
	}
	ctx := context.Background()
	ct, err := enc.Encrypt(ctx, "token")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// Flip the last character to tamper.
	tampered := ct[:len(ct)-1] + "X"
	if tampered == ct {
		tampered = ct[:len(ct)-1] + "Y"
	}
	if _, err := enc.Decrypt(ctx, tampered); err == nil {
		t.Fatal("expected error on tampered ciphertext, got nil")
	}
}

func TestAESGCMDecryptWrongKeyFails(t *testing.T) {
	enc, err := crypto.NewAESGCMEncryptor(testKey())
	if err != nil {
		t.Fatalf("new encryptor: %v", err)
	}
	ct, err := enc.Encrypt(context.Background(), "token")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	otherKey := []byte("different-key-that-is-32-bytes!!")
	dec, err := crypto.NewAESGCMEncryptor(otherKey)
	if err != nil {
		t.Fatalf("new decryptor with different key: %v", err)
	}
	if _, err := dec.Decrypt(context.Background(), ct); err == nil {
		t.Fatal("expected error decrypting with wrong key, got nil")
	}
}

func TestAESGCMWrongKeySizeFails(t *testing.T) {
	if _, err := crypto.NewAESGCMEncryptor([]byte("tooshort")); err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestNoOpEncryptorRoundTrip(t *testing.T) {
	enc := crypto.NoOpEncryptor{}
	ctx := context.Background()
	plaintext := "my-token"

	ct, err := enc.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ct == plaintext {
		t.Fatal("noop ciphertext must differ from plaintext (has prefix)")
	}

	got, err := enc.Decrypt(ctx, ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, got)
	}
}

func TestNoOpDecryptMissingPrefixFails(t *testing.T) {
	enc := crypto.NoOpEncryptor{}
	if _, err := enc.Decrypt(context.Background(), "no-prefix"); err == nil {
		t.Fatal("expected error for missing noop prefix")
	}
}

func TestValidateTokenForUseActive(t *testing.T) {
	expiresAt := time.Now().UTC().Add(time.Hour)
	acc := auth.OAuthAccount{Status: auth.OAuthAccountStatusActive, TokenExpiresAt: &expiresAt}
	if err := auth.ValidateTokenForUse(acc, time.Now().UTC()); err != nil {
		t.Fatalf("expected nil for active account, got %v", err)
	}
}

func TestValidateTokenForUseActiveWithoutExpiryRequiresReconnect(t *testing.T) {
	acc := auth.OAuthAccount{Status: auth.OAuthAccountStatusActive}
	if err := auth.ValidateTokenForUse(acc, time.Now().UTC()); err != auth.ErrTokenReconnectRequired {
		t.Fatalf("expected ErrTokenReconnectRequired, got %v", err)
	}
}

func TestValidateTokenForUseExpiredStatus(t *testing.T) {
	acc := auth.OAuthAccount{Status: auth.OAuthAccountStatusExpired}
	if err := auth.ValidateTokenForUse(acc, time.Now().UTC()); err != auth.ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestValidateTokenForUseReconnectRequired(t *testing.T) {
	acc := auth.OAuthAccount{Status: auth.OAuthAccountStatusReconnectRequired}
	if err := auth.ValidateTokenForUse(acc, time.Now().UTC()); err != auth.ErrTokenReconnectRequired {
		t.Fatalf("expected ErrTokenReconnectRequired, got %v", err)
	}
}

func TestValidateTokenForUseRevoked(t *testing.T) {
	acc := auth.OAuthAccount{Status: auth.OAuthAccountStatusRevoked}
	if err := auth.ValidateTokenForUse(acc, time.Now().UTC()); err != auth.ErrTokenRevoked {
		t.Fatalf("expected ErrTokenRevoked, got %v", err)
	}
}

func TestValidateTokenForUseExpiredByTime(t *testing.T) {
	past := time.Now().UTC().Add(-1 * time.Hour)
	acc := auth.OAuthAccount{
		Status:         auth.OAuthAccountStatusActive,
		TokenExpiresAt: &past,
	}
	if err := auth.ValidateTokenForUse(acc, time.Now().UTC()); err != auth.ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired for past expiry, got %v", err)
	}
}
