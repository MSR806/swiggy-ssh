package crypto

import (
	"context"
	"fmt"
	"strings"
)

const noopPrefix = "noop:"

// NoOpEncryptor stores tokens as "noop:<plaintext>".
// FOR TESTING ONLY — never use in production.
type NoOpEncryptor struct{}

func (NoOpEncryptor) Encrypt(_ context.Context, plaintext string) (string, error) {
	return noopPrefix + plaintext, nil
}

func (NoOpEncryptor) Decrypt(_ context.Context, ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, noopPrefix) {
		return "", fmt.Errorf("noop decrypt: missing prefix in %q", ciphertext)
	}
	return strings.TrimPrefix(ciphertext, noopPrefix), nil
}
