package sshserver

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestLoadOrCreateHostKeyPersistsAndReusesKey(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	hostKeyPath := filepath.Join(tempDir, "ssh_host_ed25519_key")

	signer1, err := LoadOrCreateHostKey(hostKeyPath)
	if err != nil {
		t.Fatalf("LoadOrCreateHostKey first call failed: %v", err)
	}

	fileInfo, err := os.Stat(hostKeyPath)
	if err != nil {
		t.Fatalf("host key file missing: %v", err)
	}

	if perms := fileInfo.Mode().Perm(); perms != 0o600 {
		t.Fatalf("expected host key perms 0600, got %#o", perms)
	}

	signer2, err := LoadOrCreateHostKey(hostKeyPath)
	if err != nil {
		t.Fatalf("LoadOrCreateHostKey second call failed: %v", err)
	}

	fp1 := ssh.FingerprintSHA256(signer1.PublicKey())
	fp2 := ssh.FingerprintSHA256(signer2.PublicKey())
	if fp1 != fp2 {
		t.Fatalf("expected same host key fingerprint across reload, got %s vs %s", fp1, fp2)
	}
}

func TestLoadOrCreateHostKeyTightensExistingPermissions(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	hostKeyPath := filepath.Join(tempDir, "ssh_host_ed25519_key")

	_, err := LoadOrCreateHostKey(hostKeyPath)
	if err != nil {
		t.Fatalf("create key failed: %v", err)
	}

	if err := os.Chmod(hostKeyPath, 0o644); err != nil {
		t.Fatalf("chmod key to broad perms failed: %v", err)
	}

	_, err = LoadOrCreateHostKey(hostKeyPath)
	if err != nil {
		t.Fatalf("reload key failed: %v", err)
	}

	fileInfo, err := os.Stat(hostKeyPath)
	if err != nil {
		t.Fatalf("stat host key failed: %v", err)
	}

	if perms := fileInfo.Mode().Perm(); perms != 0o600 {
		t.Fatalf("expected tightened perms 0600, got %#o", perms)
	}
}
