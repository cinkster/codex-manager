package sessions

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
)

// ParseSessionMeta extracts session metadata and (if available) a working directory.
func ParseSessionMeta(path string) (*SessionMeta, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var meta *SessionMeta
	var cwdCandidate string

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			lineText := strings.TrimRight(string(line), "\r\n")

			if cwdCandidate == "" {
				if content := extractMessageTextFromLine(lineText); content != "" {
					if cwd := extractCwdFromText(content); cwd != "" {
						cwdCandidate = cwd
					}
				}
			}

			if meta == nil {
				meta = metaFromLine(lineText)
			} else if meta.ID == "" || meta.Cwd == "" || meta.Timestamp == "" {
				if parsed := metaFromLine(lineText); parsed != nil {
					mergeMeta(meta, *parsed)
				}
			}

			if meta != nil && meta.Cwd == "" && cwdCandidate != "" {
				meta.Cwd = cwdCandidate
			}

			if meta != nil && meta.ID != "" && meta.Cwd != "" {
				break
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	if meta == nil {
		return nil, nil
	}
	return meta, nil
}

func metaFromLine(lineText string) *SessionMeta {
	var env envelope
	if err := json.Unmarshal([]byte(lineText), &env); err != nil {
		return nil
	}
	if env.Type == "session_meta" {
		var meta SessionMeta
		if err := json.Unmarshal(env.Payload, &meta); err != nil {
			return nil
		}
		return &meta
	}
	if env.Type != "" {
		return nil
	}

	var metaLine metaLinePayload
	if err := json.Unmarshal([]byte(lineText), &metaLine); err != nil {
		return nil
	}
	if metaLine.ID == "" && metaLine.Timestamp == "" && metaLine.Cwd == "" && metaLine.Originator == "" && metaLine.CliVersion == "" && metaLine.Instructions == nil {
		return nil
	}
	out := SessionMeta{
		ID:         metaLine.ID,
		Timestamp:  metaLine.Timestamp,
		Cwd:        metaLine.Cwd,
		Originator: metaLine.Originator,
		CliVersion: metaLine.CliVersion,
	}
	if metaLine.Instructions != nil {
		out.Instructions = *metaLine.Instructions
	}
	return &out
}

func extractMessageTextFromLine(lineText string) string {
	var env envelope
	if err := json.Unmarshal([]byte(lineText), &env); err != nil {
		return ""
	}
	switch env.Type {
	case "response_item":
		var payload responseItemPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return ""
		}
		if payload.Type != "message" {
			return ""
		}
		return extractContentText(payload.Content)
	case "message":
		var payload directMessagePayload
		if err := json.Unmarshal([]byte(lineText), &payload); err != nil {
			return ""
		}
		if payload.Type != "message" {
			return ""
		}
		return extractContentText(payload.Content)
	default:
		return ""
	}
}
