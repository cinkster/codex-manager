package htmlbucket

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadAuth(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "auth.json")

	if err := os.WriteFile(path, []byte("{\"api_key\":\"abc123\"}\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	auth, err := LoadAuth(path)
	if err != nil {
		t.Fatalf("LoadAuth: %v", err)
	}
	if auth.APIKey != "abc123" {
		t.Fatalf("unexpected api key: %q", auth.APIKey)
	}
}

func TestLoadAuthInvalid(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "auth.json")

	if err := os.WriteFile(path, []byte("{\"api_key\":\"   \"}\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadAuth(path); err == nil {
		t.Fatalf("expected error for empty api key")
	}
}

func TestWriteAuth(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, ".hb", "auth.json")

	if err := WriteAuth(path, "test-key"); err != nil {
		t.Fatalf("WriteAuth: %v", err)
	}

	auth, err := LoadAuth(path)
	if err != nil {
		t.Fatalf("LoadAuth: %v", err)
	}
	if auth.APIKey != "test-key" {
		t.Fatalf("unexpected api key: %q", auth.APIKey)
	}

	if runtime.GOOS != "windows" {
		dirInfo, err := os.Stat(filepath.Dir(path))
		if err != nil {
			t.Fatalf("stat dir: %v", err)
		}
		if dirInfo.Mode().Perm() != 0o700 {
			t.Fatalf("expected dir mode 0700, got %#o", dirInfo.Mode().Perm())
		}
		fileInfo, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat file: %v", err)
		}
		if fileInfo.Mode().Perm() != 0o600 {
			t.Fatalf("expected file mode 0600, got %#o", fileInfo.Mode().Perm())
		}
	}
}

func TestPromptAPIKey(t *testing.T) {
	var out strings.Builder
	key, err := PromptAPIKey(strings.NewReader("  api-key  \n"), &out)
	if err != nil {
		t.Fatalf("PromptAPIKey: %v", err)
	}
	if key != "api-key" {
		t.Fatalf("unexpected key: %q", key)
	}
	if !strings.Contains(out.String(), "Enter htmlbucket API key:") {
		t.Fatalf("missing prompt output: %q", out.String())
	}
}

func TestPromptAPIKeyEmpty(t *testing.T) {
	if _, err := PromptAPIKey(strings.NewReader("\n"), nil); err == nil {
		t.Fatalf("expected empty-key error")
	}
}
