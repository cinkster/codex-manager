package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSession(t *testing.T) {
	base := t.TempDir()
	filePath := filepath.Join(base, "session.jsonl")
	data := "" +
		"{\"timestamp\":\"2026-01-09T01:00:00Z\",\"type\":\"session_meta\",\"payload\":{\"id\":\"abc\",\"timestamp\":\"2026-01-09T01:00:00Z\",\"cwd\":\"/tmp\",\"originator\":\"cli\",\"cli_version\":\"0.1\",\"instructions\":\"hello\"}}\n" +
		"{\"timestamp\":\"2026-01-09T01:00:01Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"message\",\"role\":\"user\",\"content\":[{\"type\":\"input_text\",\"text\":\"Context that should be dropped\"}]}}\n" +
		"{\"timestamp\":\"2026-01-09T01:00:01Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"message\",\"role\":\"user\",\"content\":[{\"type\":\"input_text\",\"text\":\"Hello\\n\\n## My request for Codex:\\nOnly this\"}]}}\n" +
		"{\"timestamp\":\"2026-01-09T01:00:02Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"function_call\",\"name\":\"shell_command\",\"arguments\":\"{}\",\"call_id\":\"call_1\"}}\n" +
		"{\"timestamp\":\"2026-01-09T01:00:03Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"function_call_output\",\"call_id\":\"call_1\",\"output\":\"done\"}}\n" +
		"{\"timestamp\":\"2026-01-09T01:00:04Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"reasoning\",\"summary\":[{\"type\":\"summary_text\",\"text\":\"Reason\"}]}}\n" +
		"{\"timestamp\":\"2026-01-09T01:00:05Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"message\",\"role\":\"user\",\"content\":[{\"type\":\"input_text\",\"text\":\"Earlier\"}]}}\n" +
		"{\"timestamp\":\"2026-01-09T01:00:06Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"message\",\"role\":\"user\",\"content\":[{\"type\":\"input_text\",\"text\":\"Later\"}]}}\n" +
		"{\"timestamp\":\"2026-01-09T01:00:05Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"user_message\",\"message\":\"Context\",\"images\":[]}}\n" +
		"not-json\n"

	if err := os.WriteFile(filePath, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	session, err := ParseSession(filePath)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if session.Meta == nil || session.Meta.ID != "abc" {
		t.Fatalf("expected session meta")
	}
	if len(session.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(session.Items))
	}
	if session.Items[0].Content != "Only this" {
		t.Fatalf("unexpected message content: %q", session.Items[0].Content)
	}
	if session.Items[1].Content != "Reason" {
		t.Fatalf("expected reasoning summary, got %q", session.Items[1].Content)
	}
	if session.Items[2].Content != "Later" {
		t.Fatalf("expected last user message, got %q", session.Items[2].Content)
	}
}

func TestParseSessionDirectFormat(t *testing.T) {
	base := t.TempDir()
	filePath := filepath.Join(base, "session.jsonl")
	data := "" +
		"{\"id\":\"xyz\",\"timestamp\":\"2025-08-27T16:17:00.964Z\",\"instructions\":null}\n" +
		"{\"record_type\":\"state\"}\n" +
		"{\"type\":\"message\",\"id\":null,\"role\":\"user\",\"content\":[{\"type\":\"input_text\",\"text\":\"<environment_context>\\nCurrent working directory: /tmp\\n</environment_context>\"}]}\n" +
		"{\"type\":\"message\",\"id\":null,\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hello\"}]}\n" +
		"{\"type\":\"reasoning\",\"id\":\"rs_1\",\"summary\":[{\"type\":\"summary_text\",\"text\":\"Reason\"}]}\n"

	if err := os.WriteFile(filePath, []byte(data), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	session, err := ParseSession(filePath)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if session.Meta == nil || session.Meta.ID != "xyz" {
		t.Fatalf("expected session meta id, got %#v", session.Meta)
	}
	if session.Meta.Cwd != "/tmp" {
		t.Fatalf("expected cwd /tmp, got %q", session.Meta.Cwd)
	}
	if len(session.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(session.Items))
	}
	if session.Items[1].Content != "Hello" {
		t.Fatalf("unexpected assistant content: %q", session.Items[1].Content)
	}
	if session.Items[2].Content != "Reason" {
		t.Fatalf("unexpected reasoning content: %q", session.Items[2].Content)
	}
}
