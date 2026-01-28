package sessions

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
)

// Session represents a parsed conversation file.
type Session struct {
	Path  string
	Meta  *SessionMeta
	Items []RenderItem
}

// SessionMeta holds metadata from session_meta entries.
type SessionMeta struct {
	ID           string `json:"id"`
	Timestamp    string `json:"timestamp"`
	Cwd          string `json:"cwd"`
	Originator   string `json:"originator"`
	CliVersion   string `json:"cli_version"`
	Instructions string `json:"instructions"`
}

// RenderItem is a display-ready entry for the HTML view.
type RenderItem struct {
	Line      int
	Timestamp string
	Type      string
	Subtype   string
	Role      string
	Title     string
	Content   string
	Raw       string
	Class     string
}

type envelope struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type responseContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responseItemPayload struct {
	Type      string            `json:"type"`
	Role      string            `json:"role"`
	Content   []responseContent `json:"content"`
	Name      string            `json:"name"`
	Arguments string            `json:"arguments"`
	CallID    string            `json:"call_id"`
	Output    string            `json:"output"`
}

type eventMsgPayload struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ParseSession reads a jsonl file and returns a parsed Session.
func ParseSession(path string) (*Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	session := &Session{Path: path}
	reader := bufio.NewReader(file)
	lineNum := 0

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			lineNum++
			lineText := strings.TrimRight(string(line), "\r\n")
			item := parseLine(lineText, lineNum, session)
			if item != nil {
				session.Items = append(session.Items, *item)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	session.Items = mergeConsecutive(session.Items)

	return session, nil
}

func parseLine(lineText string, lineNum int, session *Session) *RenderItem {
	var env envelope
	if err := json.Unmarshal([]byte(lineText), &env); err != nil {
		return nil
	}

	switch env.Type {
	case "session_meta":
		var meta SessionMeta
		if err := json.Unmarshal(env.Payload, &meta); err == nil {
			session.Meta = &meta
		}
		return nil
	case "response_item":
		return parseResponseItem(env, lineText, lineNum)
	default:
		return nil
	}
}

func parseResponseItem(env envelope, lineText string, lineNum int) *RenderItem {
	var payload responseItemPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return nil
	}

	item := RenderItem{
		Line:      lineNum,
		Timestamp: env.Timestamp,
		Type:      env.Type,
		Subtype:   payload.Type,
		Role:      payload.Role,
		Title:     titleForType(env.Type, payload.Type),
		Raw:       lineText,
	}

	switch payload.Type {
	case "message":
		if payload.Role != "user" && payload.Role != "assistant" {
			return nil
		}
		if payload.Role == "user" {
			item.Title = "User"
		} else {
			item.Title = "Agent"
		}
		item.Content = extractContentText(payload.Content)
		if payload.Role == "user" {
			item.Content = trimUserRequest(item.Content)
		}
		if item.Content == "" {
			item.Content = prettyJSON(string(env.Payload))
		}
		item.Class = roleClass(payload.Role)
	case "reasoning":
		item.Role = "assistant"
		item.Class = roleClass("assistant")
		item.Content = extractReasoningSummary(env.Payload)
		if item.Content == "" {
			item.Content = prettyJSON(string(env.Payload))
		}
	default:
		return nil
	}

	if strings.TrimSpace(item.Content) == "" {
		item.Content = "(empty)"
	}

	return &item
}

func parseEventMsg(env envelope, lineText string, lineNum int) *RenderItem {
	var payload eventMsgPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return &RenderItem{
			Line:      lineNum,
			Timestamp: env.Timestamp,
			Type:      env.Type,
			Title:     "event_msg",
			Content:   prettyJSON(lineText),
			Raw:       lineText,
			Class:     roleClass("system"),
		}
	}

	content := payload.Message
	if content == "" {
		content = prettyJSON(string(env.Payload))
	}

	return &RenderItem{
		Line:      lineNum,
		Timestamp: env.Timestamp,
		Type:      env.Type,
		Subtype:   payload.Type,
		Title:     titleForType(env.Type, payload.Type),
		Content:   content,
		Raw:       lineText,
		Class:     roleClass("user"),
	}
}

func extractContentText(contents []responseContent) string {
	if len(contents) == 0 {
		return ""
	}
	parts := make([]string, 0, len(contents))
	for _, item := range contents {
		if item.Text == "" {
			continue
		}
		parts = append(parts, item.Text)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func extractReasoningSummary(raw json.RawMessage) string {
	var payload struct {
		Summary []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if len(payload.Summary) == 0 {
		return ""
	}
	parts := make([]string, 0, len(payload.Summary))
	for _, item := range payload.Summary {
		if item.Text == "" {
			continue
		}
		parts = append(parts, item.Text)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func prettyJSON(raw string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(raw), "", "  "); err != nil {
		return raw
	}
	return buf.String()
}

func titleForType(eventType, subType string) string {
	if eventType == "response_item" {
		switch subType {
		case "message":
			return "Message"
		case "function_call":
			return "Tool call"
		case "function_call_output":
			return "Tool output"
		case "reasoning":
			return "Reasoning"
		default:
			return "Response item"
		}
	}
	if eventType == "event_msg" {
		if subType == "user_message" {
			return "User context"
		}
		return "Event"
	}
	return strings.ReplaceAll(eventType, "_", " ")
}

func roleClass(role string) string {
	switch strings.ToLower(role) {
	case "user":
		return "role-user"
	case "assistant":
		return "role-assistant"
	case "system":
		return "role-system"
	case "tool":
		return "role-tool"
	case "error":
		return "role-error"
	default:
		return "role-unknown"
	}
}

func mergeConsecutive(items []RenderItem) []RenderItem {
	if len(items) == 0 {
		return items
	}
	out := make([]RenderItem, 0, len(items))
	current := items[0]
	inUserGroup := isUserMessage(current)
	for i := 1; i < len(items); i++ {
		item := items[i]
		if current.Type == item.Type && current.Subtype == item.Subtype && current.Role == item.Role {
			if inUserGroup && isUserMessage(item) {
				current.Content = item.Content
				current.Line = item.Line
				current.Timestamp = item.Timestamp
				current.Raw = item.Raw
			} else if strings.TrimSpace(item.Content) != "" {
				if strings.TrimSpace(current.Content) != "" {
					current.Content = current.Content + "\n\n" + item.Content
				} else {
					current.Content = item.Content
				}
			}
			continue
		}
		out = append(out, current)
		current = item
		inUserGroup = isUserMessage(current)
	}
	out = append(out, current)
	return out
}

func trimUserRequest(content string) string {
	if !trimUserRequestEnabled {
		return content
	}
	marker := "## My request for Codex:"
	index := strings.Index(content, marker)
	if index == -1 {
		return content
	}
	trimmed := content[index+len(marker):]
	return strings.TrimSpace(trimmed)
}

var trimUserRequestEnabled = true

// SetTrimUserRequestEnabled controls whether user messages are trimmed to the request marker.
func SetTrimUserRequestEnabled(enabled bool) {
	trimUserRequestEnabled = enabled
}

func isUserMessage(item RenderItem) bool {
	return item.Subtype == "message" && item.Role == "user"
}
