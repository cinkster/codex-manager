package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codex-manager/internal/config"
	"codex-manager/internal/htmlbucket"
)

func TestSetupHTMLBucketUsesExistingAuthByDefault(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)

	authPath := filepath.Join(home, ".hb", "auth.json")
	if err := htmlbucket.WriteAuth(authPath, "existing-key"); err != nil {
		t.Fatalf("WriteAuth: %v", err)
	}

	cfg := config.Config{UseHTMLBucket: false}
	client, gotPath, err := setupHTMLBucket(cfg, strings.NewReader(""), io.Discard)
	if err != nil {
		t.Fatalf("setupHTMLBucket: %v", err)
	}
	if client == nil {
		t.Fatalf("expected client")
	}
	if gotPath != authPath {
		t.Fatalf("auth path mismatch: got %q want %q", gotPath, authPath)
	}
}

func TestSetupHTMLBucketMissingWithoutFlag(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)

	cfg := config.Config{UseHTMLBucket: false}
	client, _, err := setupHTMLBucket(cfg, strings.NewReader(""), io.Discard)
	if err != nil {
		t.Fatalf("setupHTMLBucket: %v", err)
	}
	if client != nil {
		t.Fatalf("expected nil client when auth is missing and -hb is off")
	}
}

func TestSetupHTMLBucketPromptsAndWritesAuth(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)

	cfg := config.Config{UseHTMLBucket: true}
	client, authPath, err := setupHTMLBucket(cfg, strings.NewReader("prompted-key\n"), io.Discard)
	if err != nil {
		t.Fatalf("setupHTMLBucket: %v", err)
	}
	if client == nil {
		t.Fatalf("expected client")
	}
	auth, err := htmlbucket.LoadAuth(authPath)
	if err != nil {
		t.Fatalf("LoadAuth: %v", err)
	}
	if auth.APIKey != "prompted-key" {
		t.Fatalf("unexpected key: %q", auth.APIKey)
	}
}

func TestSetupHTMLBucketInvalidAuthFails(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)

	authPath := filepath.Join(home, ".hb", "auth.json")
	if err := os.MkdirAll(filepath.Dir(authPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(authPath, []byte("{not-json}\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := config.Config{UseHTMLBucket: false}
	if _, _, err := setupHTMLBucket(cfg, strings.NewReader(""), io.Discard); err == nil {
		t.Fatalf("expected error for invalid auth file")
	}
}

func TestSetupHTMLBucketPromptFailure(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)

	cfg := config.Config{UseHTMLBucket: true}
	if _, _, err := setupHTMLBucket(cfg, strings.NewReader(""), io.Discard); err == nil {
		t.Fatalf("expected prompt/read failure")
	}
}

func setHomeEnv(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}
