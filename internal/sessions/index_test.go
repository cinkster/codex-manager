package sessions

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIndexRefreshAndLookup(t *testing.T) {
	base := t.TempDir()
	pathA := filepath.Join(base, "2026", "01", "09")
	if err := os.MkdirAll(pathA, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	filePath := filepath.Join(pathA, "session-a.jsonl")
	if err := os.WriteFile(filePath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chtimes(filePath, time.Now(), time.Now()); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	idx := NewIndex(base)
	if err := idx.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	dates := idx.Dates()
	if len(dates) != 1 {
		t.Fatalf("expected 1 date, got %d", len(dates))
	}
	date := dates[0]
	if date.String() != "2026-01-09" {
		t.Fatalf("unexpected date: %s", date.String())
	}

	files := idx.SessionsByDate(date)
	if len(files) != 1 {
		t.Fatalf("expected 1 session, got %d", len(files))
	}
	if files[0].Name != "session-a.jsonl" {
		t.Fatalf("unexpected file: %s", files[0].Name)
	}

	lookup, ok := idx.Lookup(date, "session-a.jsonl")
	if !ok {
		t.Fatalf("expected lookup to succeed")
	}
	if lookup.Path != filePath {
		t.Fatalf("unexpected path: %s", lookup.Path)
	}
}
