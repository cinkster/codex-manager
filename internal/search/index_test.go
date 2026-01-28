package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codex-manager/internal/sessions"
)

func TestIndexSearch(t *testing.T) {
	baseDir := t.TempDir()
	writeSessionFile(t, baseDir, "2024/01/02/session.jsonl", []string{
		`{"timestamp":"t1","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"text","text":"Hello world"}]}}`,
		`{"timestamp":"t2","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"text","text":"Here is a search result"}]}}`,
	})

	idx := sessions.NewIndex(baseDir)
	if err := idx.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	searchIdx := NewIndex()
	if err := searchIdx.RefreshFrom(idx); err != nil {
		t.Fatalf("search refresh: %v", err)
	}

	results := searchIdx.Search("hello", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].File != "session.jsonl" {
		t.Fatalf("expected file session.jsonl, got %q", results[0].File)
	}
	if results[0].Line != 1 {
		t.Fatalf("expected line 1, got %d", results[0].Line)
	}
	if !strings.Contains(results[0].Preview, "Hello") {
		t.Fatalf("expected preview to contain Hello, got %q", results[0].Preview)
	}

	writeSessionFile(t, baseDir, "2024/01/02/session.jsonl", []string{
		`{"timestamp":"t3","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"text","text":"Updated content for search"}]}}`,
	})
	if err := idx.Refresh(); err != nil {
		t.Fatalf("refresh after update: %v", err)
	}
	if err := searchIdx.RefreshFrom(idx); err != nil {
		t.Fatalf("search refresh after update: %v", err)
	}

	results = searchIdx.Search("hello", 10)
	if len(results) != 0 {
		t.Fatalf("expected 0 results after update, got %d", len(results))
	}
	results = searchIdx.Search("updated", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for updated, got %d", len(results))
	}
	if results[0].Line != 1 {
		t.Fatalf("expected line 1 after update, got %d", results[0].Line)
	}
}

func writeSessionFile(t *testing.T, baseDir, relPath string, lines []string) {
	t.Helper()
	fullPath := filepath.Join(baseDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	payload := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(fullPath, []byte(payload), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
