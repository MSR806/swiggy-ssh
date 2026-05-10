package sshserver

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

func LoadOrCreateHostKey(path string) (ssh.Signer, error) {
	keyBytes, err := os.ReadFile(path)
	if err == nil {
		fileInfo, statErr := os.Stat(path)
		if statErr != nil {
			return nil, fmt.Errorf("stat host key %s: %w", path, statErr)
		}

		if fileInfo.Mode().Perm() != 0o600 {
			if chmodErr := os.Chmod(path, 0o600); chmodErr != nil {
				return nil, fmt.Errorf("chmod host key %s: %w", path, chmodErr)
			}
		}

		signer, parseErr := ssh.ParsePrivateKey(keyBytes)
		if parseErr != nil {
			return nil, fmt.Errorf("parse host key %s: %w", path, parseErr)
		}
		return signer, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read host key %s: %w", path, err)
	}

	if mkErr := os.MkdirAll(filepath.Dir(path), 0o700); mkErr != nil {
		return nil, fmt.Errorf("mkdir host key dir: %w", mkErr)
	}

	_, privateKey, genErr := ed25519.GenerateKey(rand.Reader)
	if genErr != nil {
		return nil, fmt.Errorf("generate ed25519 host key: %w", genErr)
	}

	pkcs8Key, marshalErr := x509.MarshalPKCS8PrivateKey(privateKey)
	if marshalErr != nil {
		return nil, fmt.Errorf("marshal host key: %w", marshalErr)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Key})
	if writeErr := os.WriteFile(path, pemBytes, 0o600); writeErr != nil {
		return nil, fmt.Errorf("write host key %s: %w", path, writeErr)
	}

	signer, parseErr := ssh.ParsePrivateKey(pemBytes)
	if parseErr != nil {
		return nil, fmt.Errorf("parse generated host key %s: %w", path, parseErr)
	}

	return signer, nil
}
