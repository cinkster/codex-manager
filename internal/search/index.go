package search

import (
	"strings"
	"sync"
	"time"

	"codex-manager/internal/sessions"
)

const (
	defaultLimit  = 50
	maxLimit      = 200
	snippetRadius = 60
	snippetMax    = 180
)

// Result describes a single search match.
type Result struct {
	Date    string `json:"date"`
	Path    string `json:"path"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Role    string `json:"role"`
	Preview string `json:"preview"`
}

type entry struct {
	date    string
	path    string
	file    string
	line    int
	role    string
	content string
	lower   string
}

type fileIndex struct {
	size    int64
	modTime time.Time
	entries []entry
}

// Index stores a searchable snapshot of sessions.
type Index struct {
	mu      sync.RWMutex
	files   map[string]fileIndex
	ordered []entry
}

// NewIndex creates an empty search index.
func NewIndex() *Index {
	return &Index{files: map[string]fileIndex{}}
}

// RefreshFrom rebuilds entries for new or changed files in the sessions index.
func (idx *Index) RefreshFrom(sessionsIdx *sessions.Index) error {
	dates := sessionsIdx.Dates()
	files := make([]sessions.SessionFile, 0, len(dates))
	for _, date := range dates {
		files = append(files, sessionsIdx.SessionsByDate(date)...)
	}

	idx.mu.RLock()
	existing := idx.files
	idx.mu.RUnlock()

	next := make(map[string]fileIndex, len(files))
	toParse := make([]sessions.SessionFile, 0)
	for _, file := range files {
		key := file.Path
		if meta, ok := existing[key]; ok && meta.size == file.Size && meta.modTime.Equal(file.ModTime) {
			next[key] = meta
			continue
		}
		toParse = append(toParse, file)
	}

	var firstErr error
	for _, file := range toParse {
		entries, err := buildEntries(file)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if meta, ok := existing[file.Path]; ok {
				next[file.Path] = meta
			}
			continue
		}
		next[file.Path] = fileIndex{size: file.Size, modTime: file.ModTime, entries: entries}
	}

	ordered := make([]entry, 0)
	for _, date := range dates {
		for _, file := range sessionsIdx.SessionsByDate(date) {
			if meta, ok := next[file.Path]; ok {
				ordered = append(ordered, meta.entries...)
			}
		}
	}

	idx.mu.Lock()
	idx.files = next
	idx.ordered = ordered
	idx.mu.Unlock()

	return firstErr
}

// Search returns the first N matches for the query.
func (idx *Index) Search(query string, limit int) []Result {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	lower := strings.ToLower(q)

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	results := make([]Result, 0, limit)
	for _, item := range idx.ordered {
		matchIndex := strings.Index(item.lower, lower)
		if matchIndex == -1 {
			continue
		}
		preview := makePreview(item.content, matchIndex, len(q))
		results = append(results, Result{
			Date:    item.date,
			Path:    item.path,
			File:    item.file,
			Line:    item.line,
			Role:    item.role,
			Preview: preview,
		})
		if len(results) >= limit {
			break
		}
	}

	return results
}

func buildEntries(file sessions.SessionFile) ([]entry, error) {
	session, err := sessions.ParseSession(file.Path)
	if err != nil {
		return nil, err
	}

	entries := make([]entry, 0, len(session.Items))
	dateLabel := file.Date.String()
	datePath := file.Date.Path()
	for _, item := range session.Items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		entries = append(entries, entry{
			date:    dateLabel,
			path:    datePath,
			file:    file.Name,
			line:    item.Line,
			role:    item.Role,
			content: content,
			lower:   strings.ToLower(content),
		})
	}
	return entries, nil
}

func makePreview(content string, matchIndex int, queryLen int) string {
	cleaned := strings.ReplaceAll(content, "\r", " ")
	cleaned = strings.ReplaceAll(cleaned, "\n", " ")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return ""
	}
	if matchIndex < 0 || matchIndex >= len(cleaned) || queryLen <= 0 {
		return truncate(cleaned, snippetMax)
	}
	start := matchIndex - snippetRadius
	if start < 0 {
		start = 0
	}
	end := matchIndex + queryLen + snippetRadius
	if end > len(cleaned) {
		end = len(cleaned)
	}
	snippet := strings.TrimSpace(cleaned[start:end])
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(cleaned) {
		snippet = snippet + "..."
	}
	return snippet
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}
