package web

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"codex-manager/internal/render"
	"codex-manager/internal/search"
	"codex-manager/internal/sessions"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// Server serves the HTML views.
type Server struct {
	idx           *sessions.Index
	search        *search.Index
	renderer      *render.Renderer
	sessionsDir   string
	shareDir      string
	shareAddr     string
	themeClass    string
	useTailscale  bool
	tailscaleHost string
}

// NewServer wires up the HTTP server.
func NewServer(idx *sessions.Index, searchIdx *search.Index, renderer *render.Renderer, sessionsDir, shareDir, shareAddr string, theme int) *Server {
	return &Server{
		idx:         idx,
		search:      searchIdx,
		renderer:    renderer,
		sessionsDir: sessionsDir,
		shareDir:    shareDir,
		shareAddr:   shareAddr,
		themeClass:  themeClass(theme),
	}
}

// EnableTailscale sets the host used for share redirects.
func (s *Server) EnableTailscale(host string) {
	s.useTailscale = true
	s.tailscaleHost = strings.TrimSuffix(host, ".")
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pathValue := strings.Trim(r.URL.Path, "/")
	if pathValue == "" {
		s.handleIndex(w, r)
		return
	}
	if pathValue == "dir" {
		s.handleDir(w, r)
		return
	}
	if pathValue == "search" {
		s.handleSearch(w, r)
		return
	}
	if strings.HasPrefix(pathValue, "raw/") {
		s.handleRaw(w, r, strings.TrimPrefix(pathValue, "raw/"))
		return
	}

	parts := strings.Split(pathValue, "/")
	if len(parts) == 5 && r.Method == http.MethodPost && parts[0] == "share" {
		s.handleShare(w, r, parts[1:])
		return
	}
	if len(parts) == 3 {
		s.handleDay(w, r, parts)
		return
	}
	if len(parts) == 4 {
		s.handleSession(w, r, parts)
		return
	}

	http.NotFound(w, r)
}

type dateView struct {
	Label string
	Path  string
	Count int
}

type dirView struct {
	Label       string
	Value       string
	Count       int
	RecentCount int
	HeatColor   template.CSS
}

type sessionView struct {
	Name                 string
	Size                 string
	ModTime              string
	ModTimeOnly          string
	ResumeCommand        string
	Cwd                  string
	DateLabel            string
	DatePath             string
	LastUserSnippet      string
	LastAssistantSnippet string
}

type sessionNavLink struct {
	Path  string
	Title string
}

type sessionNavView struct {
	CwdLabel string
	Prev     *sessionNavLink
	Next     *sessionNavLink
}

type indexView struct {
	Dates       []dateView
	Dirs        []dirView
	SessionsDir string
	LastScan    string
	View        string
	HeatMode    string
	ThemeClass  string
}

type dayView struct {
	Date             dateView
	Sessions         []sessionView
	Dirs             []dirView
	SelectedCwd      string
	SelectedCwdLabel string
	FallbackDate     *dateView
	FallbackSessions []sessionView
	FallbackDirs     []dirView
	Page             int
	TotalPages       int
	HasPrev          bool
	HasNext          bool
	PrevPage         int
	NextPage         int
	ShowAll          bool
	View             string
	ThemeClass       string
}

type dirPageView struct {
	Dir        dirView
	Dates      []dateView
	Sessions   []sessionView
	Page       int
	TotalPages int
	HasPrev    bool
	HasNext    bool
	PrevPage   int
	NextPage   int
	ShowAll    bool
	ThemeClass string
}

type sessionPageView struct {
	Date          dateView
	File          sessionView
	Meta          *sessions.SessionMeta
	Items         []itemView
	AllMarkdown   string
	ResumeCommand string
	ThemeClass    string
	IsJSONL       bool
	LastUserLine  int
	LastAgentLine int
	LastItemLine  int
	CwdNav        *sessionNavView
}

type itemView struct {
	Line               int
	Timestamp          string
	Type               string
	Subtype            string
	Role               string
	Title              string
	Content            string
	Class              string
	AutoCtx            bool
	IsTurnAborted      bool
	TurnAbortedMessage string
	Markdown           string
	HTML               template.HTML
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	view := r.URL.Query().Get("view")
	heat := r.URL.Query().Get("heat")
	if view == "" && r.URL.RawQuery == "" {
		view = "dir"
		heat = "1h"
	} else if view != "dir" {
		view = "date"
	}

	indexView := s.buildIndexView(view, heat)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.renderer.Execute(w, "index", indexView)
}

func (s *Server) handleDir(w http.ResponseWriter, r *http.Request) {
	cwd := normalizeCwdParam(r.URL.Query().Get("cwd"))
	if cwd == "" {
		indexView := s.buildIndexView("dir", r.URL.Query().Get("heat"))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = s.renderer.Execute(w, "index", indexView)
		return
	}

	files := s.idx.SessionsByCwd(cwd)
	counts := make(map[sessions.DateKey]int, len(files))
	for _, file := range files {
		counts[file.Date]++
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].ModTime.Equal(files[j].ModTime) {
			dateI := files[i].Date.String()
			dateJ := files[j].Date.String()
			if dateI != dateJ {
				return dateI > dateJ
			}
			return files[i].Name > files[j].Name
		}
		return files[i].ModTime.After(files[j].ModTime)
	})
	page := parsePageParam(r)
	showAll := parseBoolParam(r, "all")
	sessionsView, pager := buildSessionViewsPage(files, page, 10, showAll)

	dates := s.idx.Dates()
	dateViews := make([]dateView, 0, len(counts))
	for _, date := range dates {
		if count, ok := counts[date]; ok {
			dateViews = append(dateViews, dateView{
				Label: date.String(),
				Path:  date.Path(),
				Count: count,
			})
		}
	}

	dir := dirView{
		Label: dirLabel(cwd),
		Value: cwd,
		Count: len(files),
	}

	view := dirPageView{
		Dir:        dir,
		Dates:      dateViews,
		Sessions:   sessionsView,
		Page:       pager.Page,
		TotalPages: pager.TotalPages,
		HasPrev:    pager.HasPrev,
		HasNext:    pager.HasNext,
		PrevPage:   pager.PrevPage,
		NextPage:   pager.NextPage,
		ShowAll:    showAll,
		ThemeClass: s.themeClass,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.renderer.Execute(w, "dir", view)
}

func (s *Server) handleDay(w http.ResponseWriter, r *http.Request, parts []string) {
	date, ok := sessions.ParseDate(parts[0], parts[1], parts[2])
	if !ok {
		http.NotFound(w, r)
		return
	}
	selectedCwd := normalizeCwdParam(r.URL.Query().Get("cwd"))
	viewMode := strings.TrimSpace(r.URL.Query().Get("view"))
	if viewMode != "dir" {
		viewMode = "sessions"
	}

	requestedFiles := s.idx.SessionsByDate(date)
	filtered := filterSessionFilesByCwd(requestedFiles, selectedCwd)
	requestedViews := []sessionView{}
	dirViews := buildDirViewsFromFiles(requestedFiles)
	page := parsePageParam(r)
	showAll := parseBoolParam(r, "all")
	pager := paginationInfo{Page: page}
	var fallbackDate *dateView
	var fallbackSessions []sessionView
	var fallbackDirs []dirView
	if selectedCwd != "" && len(filtered) == 0 {
		if prevDate, ok := previousDateKey(date); ok {
			prevFiles := s.idx.SessionsByDate(prevDate)
			prevFiltered := filterSessionFilesByCwd(prevFiles, selectedCwd)
			if len(prevFiltered) > 0 {
				fallbackDate = &dateView{
					Label: prevDate.String(),
					Path:  prevDate.Path(),
					Count: len(prevFiles),
				}
				pageSessions, pageInfo := buildSessionViewsPage(prevFiltered, page, 10, showAll)
				pager = pageInfo
				fallbackSessions = pageSessions
				fallbackDirs = buildDirViewsFromFiles(prevFiles)
			}
		}
	}
	if fallbackDate == nil {
		pageSessions, pageInfo := buildSessionViewsPage(filtered, page, 10, showAll)
		pager = pageInfo
		requestedViews = pageSessions
	}

	selectedLabel := ""
	if selectedCwd != "" {
		selectedLabel = dirLabel(selectedCwd)
	}

	view := dayView{
		Date: dateView{
			Label: date.String(),
			Path:  date.Path(),
			Count: len(requestedFiles),
		},
		Sessions:         requestedViews,
		Dirs:             dirViews,
		SelectedCwd:      selectedCwd,
		SelectedCwdLabel: selectedLabel,
		FallbackDate:     fallbackDate,
		FallbackSessions: fallbackSessions,
		FallbackDirs:     fallbackDirs,
		Page:             pager.Page,
		TotalPages:       pager.TotalPages,
		HasPrev:          pager.HasPrev,
		HasNext:          pager.HasNext,
		PrevPage:         pager.PrevPage,
		NextPage:         pager.NextPage,
		ShowAll:          showAll,
		View:             viewMode,
		ThemeClass:       s.themeClass,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.renderer.Execute(w, "day", view)
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request, parts []string) {
	view, err := s.buildSessionView(parts)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.renderer.Execute(w, "session", view)
}

type searchResponse struct {
	Query   string          `json:"query"`
	Results []search.Result `json:"results"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if s.search == nil {
		http.Error(w, "search index not available", http.StatusServiceUnavailable)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	cwdFilter := normalizeSearchCwdFilter(r.URL.Query().Get("cwd"))
	limit := 50
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 200 {
		limit = 200
	}

	var results []search.Result
	if len(query) >= 2 {
		results = s.search.SearchWithCwd(query, limit, cwdFilter)
	} else {
		results = []search.Result{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(searchResponse{Query: query, Results: results})
}

func (s *Server) handleShare(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	view, err := s.buildSessionView(parts)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := os.MkdirAll(s.shareDir, 0o700); err != nil {
		http.Error(w, fmt.Sprintf("failed to create share dir: %v", err), http.StatusInternalServerError)
		return
	}

	token, err := randomToken(16)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create share token: %v", err), http.StatusInternalServerError)
		return
	}

	fileName := formatUUID(token) + ".html"
	targetFile := filepath.Join(s.shareDir, fileName)
	var buf bytes.Buffer
	if err := s.renderer.Execute(&buf, "session", view); err != nil {
		http.Error(w, fmt.Sprintf("failed to render html: %v", err), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(targetFile, buf.Bytes(), 0o600); err != nil {
		http.Error(w, fmt.Sprintf("failed to write share file: %v", err), http.StatusInternalServerError)
		return
	}

	shareURL := s.buildShareURL(r, fileName)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"url": shareURL})
}

func (s *Server) handleRaw(w http.ResponseWriter, r *http.Request, rawPath string) {
	parts := strings.Split(strings.Trim(rawPath, "/"), "/")
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}
	date, ok := sessions.ParseDate(parts[0], parts[1], parts[2])
	if !ok {
		http.NotFound(w, r)
		return
	}
	filename := parts[3]
	if filename == "" || strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		http.NotFound(w, r)
		return
	}
	file, ok := s.idx.Lookup(date, filename)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.Name))
	http.ServeFile(w, r, file.Path)
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div := float64(size)
	units := []string{"KB", "MB", "GB", "TB"}
	for _, suffix := range units {
		div = div / unit
		if div < unit {
			return fmt.Sprintf("%.1f %s", div, suffix)
		}
	}
	return fmt.Sprintf("%.1f PB", div/unit)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func formatTimeOnly(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("15:04:05")
}

func formatScanTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format(time.RFC3339)
}

type paginationInfo struct {
	Page       int
	TotalPages int
	HasPrev    bool
	HasNext    bool
	PrevPage   int
	NextPage   int
}

func parsePageParam(r *http.Request) int {
	page := 1
	if rawPage := r.URL.Query().Get("page"); rawPage != "" {
		if parsed, err := strconv.Atoi(rawPage); err == nil {
			page = parsed
		}
	}
	if page < 1 {
		page = 1
	}
	return page
}

func parseBoolParam(r *http.Request, key string) bool {
	value := strings.TrimSpace(strings.ToLower(r.URL.Query().Get(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func paginateSessionFiles(files []sessions.SessionFile, page int, perPage int) ([]sessions.SessionFile, paginationInfo) {
	info := paginationInfo{Page: page}
	total := len(files)
	if perPage <= 0 {
		info.TotalPages = 1
		info.HasPrev = false
		info.HasNext = false
		info.PrevPage = 1
		info.NextPage = 1
		return files, info
	}
	if total == 0 {
		info.Page = 1
		info.TotalPages = 0
		info.PrevPage = 1
		info.NextPage = 1
		return nil, info
	}
	totalPages := (total + perPage - 1) / perPage
	if page > totalPages {
		page = totalPages
	}
	if page < 1 {
		page = 1
	}
	start := (page - 1) * perPage
	end := start + perPage
	if start < 0 {
		start = 0
	}
	if end > total {
		end = total
	}
	pageFiles := files[start:end]
	info.Page = page
	info.TotalPages = totalPages
	info.HasPrev = page > 1
	info.HasNext = page < totalPages
	info.PrevPage = 1
	info.NextPage = totalPages
	if info.HasPrev {
		info.PrevPage = page - 1
	}
	if info.HasNext {
		info.NextPage = page + 1
	}
	return pageFiles, info
}

func (s *Server) buildIndexView(view string, heatMode string) indexView {
	heatMode = parseHeatMode(heatMode)
	dates := s.idx.Dates()
	dateViews := make([]dateView, 0, len(dates))
	for _, date := range dates {
		files := s.idx.SessionsByDate(date)
		dateViews = append(dateViews, dateView{
			Label: date.String(),
			Path:  date.Path(),
			Count: len(files),
		})
	}

	recentCounts := map[string]int{}
	recentMax := 0
	if view == "dir" {
		now := time.Now()
		since := now.AddDate(0, 0, -7)
		allowFallback := true
		switch heatMode {
		case "today":
			since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			allowFallback = false
		case "1h":
			since = now.Add(-1 * time.Hour)
			allowFallback = false
		}
		recentCounts, recentMax = s.recentCwdCounts(since)
		if allowFallback && recentMax == 0 {
			recentCounts, recentMax = s.recentCwdCountsFromLatestDates(7)
		}
	}
	dirViews := buildDirViewsFromCounts(s.idx.CwdCounts(), recentCounts, recentMax, view == "dir")
	lastScan := s.idx.LastUpdated()

	return indexView{
		Dates:       dateViews,
		Dirs:        dirViews,
		SessionsDir: s.sessionsDir,
		LastScan:    formatScanTime(lastScan),
		View:        view,
		HeatMode:    heatMode,
		ThemeClass:  s.themeClass,
	}
}

func filterSessionFilesByCwd(files []sessions.SessionFile, cwd string) []sessions.SessionFile {
	if cwd == "" {
		return files
	}
	filtered := make([]sessions.SessionFile, 0, len(files))
	for _, file := range files {
		if sessions.CwdForFile(file) == cwd {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func buildSessionViews(files []sessions.SessionFile) []sessionView {
	views := make([]sessionView, 0, len(files))
	for _, file := range files {
		views = append(views, buildSessionView(file))
	}
	return views
}

func buildSessionView(file sessions.SessionFile) sessionView {
	resumeCommand := buildResumeCommand(file.Meta)
	cwd := sessions.CwdForFile(file)
	if cwd == sessions.UnknownCwd {
		cwd = ""
	}
	return sessionView{
		Name:          file.Name,
		Size:          formatBytes(file.Size),
		ModTime:       formatTime(file.ModTime),
		ResumeCommand: resumeCommand,
		Cwd:           cwd,
		DateLabel:     file.Date.String(),
		DatePath:      file.Date.Path(),
	}
}

func buildSessionViewsWithSnippets(files []sessions.SessionFile) []sessionView {
	views := buildSessionViews(files)
	for i, file := range files {
		userSnippet, assistantSnippet, hasUser := extractLastSnippets(file)
		if hasUser && userSnippet == "" {
			userSnippet = "(empty)"
		}
		views[i].LastUserSnippet = userSnippet
		views[i].LastAssistantSnippet = assistantSnippet
	}
	return views
}

func buildSessionViewsPage(files []sessions.SessionFile, page int, perPage int, includeAll bool) ([]sessionView, paginationInfo) {
	if includeAll {
		pageFiles, pager := paginateSessionFiles(files, page, perPage)
		return buildSessionViewsWithSnippets(pageFiles), pager
	}
	return buildSessionViewsPageFiltered(files, page, perPage)
}

func buildSessionViewsPageFiltered(files []sessions.SessionFile, page int, perPage int) ([]sessionView, paginationInfo) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 10
	}
	start := (page - 1) * perPage
	end := start + perPage
	total := 0
	foundNext := false
	scannedAll := true
	views := make([]sessionView, 0, perPage)
	for _, file := range files {
		userSnippet, assistantSnippet, hasUser := extractLastSnippets(file)
		if !hasUser {
			continue
		}
		if total >= start && total < end {
			view := buildSessionView(file)
			if userSnippet == "" {
				userSnippet = "(empty)"
			}
			view.LastUserSnippet = userSnippet
			view.LastAssistantSnippet = assistantSnippet
			views = append(views, view)
		}
		if total >= end {
			foundNext = true
			scannedAll = false
			break
		}
		total++
	}
	totalPages := 0
	if scannedAll && total > 0 {
		totalPages = (total + perPage - 1) / perPage
		if page > totalPages {
			return buildSessionViewsPageFiltered(files, totalPages, perPage)
		}
	}
	info := paginationInfo{
		Page:       page,
		TotalPages: totalPages,
		HasPrev:    page > 1 && total > start,
		HasNext:    foundNext || (totalPages > 0 && page < totalPages),
		PrevPage:   1,
		NextPage:   totalPages,
	}
	if info.HasPrev {
		info.PrevPage = page - 1
	}
	if info.HasNext {
		info.NextPage = page + 1
	} else if totalPages > 0 {
		info.NextPage = totalPages
	}
	return views, info
}

func extractLastSnippets(file sessions.SessionFile) (string, string, bool) {
	session, err := sessions.ParseSession(file.Path)
	if err != nil {
		return "", "", false
	}
	lastUser := ""
	lastAssistant := ""
	hasUser := false
	for _, item := range session.Items {
		switch item.Role {
		case "user":
			if sessions.IsAutoContextUserMessage(item.Content) {
				continue
			}
			hasUser = true
			lastUser = item.Content
		case "assistant":
			lastAssistant = item.Content
		}
	}
	userSnippet := snippetFromContent(lastUser, 180)
	assistantSnippet := snippetFromContent(lastAssistant, 180)
	return userSnippet, assistantSnippet, hasUser
}

func snippetFromContent(value string, max int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.Join(strings.Fields(value), " ")
	if max <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	if max > 3 {
		return string(runes[:max-3]) + "..."
	}
	return string(runes[:max])
}

func buildDirViewsFromFiles(files []sessions.SessionFile) []dirView {
	counts := make(map[string]int, len(files))
	for _, file := range files {
		cwd := sessions.CwdForFile(file)
		counts[cwd]++
	}
	return buildDirViewsFromCounts(counts, nil, 0, false)
}

func buildDirViewsFromCounts(counts map[string]int, recentCounts map[string]int, recentMax int, withHeat bool) []dirView {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == sessions.UnknownCwd {
			return false
		}
		if keys[j] == sessions.UnknownCwd {
			return true
		}
		return keys[i] < keys[j]
	})

	views := make([]dirView, 0, len(keys))
	for _, key := range keys {
		view := dirView{
			Label: dirLabel(key),
			Value: key,
			Count: counts[key],
		}
		if withHeat {
			view.RecentCount = recentCounts[key]
			view.HeatColor = heatColor(view.RecentCount, recentMax)
		}
		views = append(views, view)
	}
	return views
}

func previousDateKey(date sessions.DateKey) (sessions.DateKey, bool) {
	year, err := strconv.Atoi(date.Year)
	if err != nil {
		return sessions.DateKey{}, false
	}
	month, err := strconv.Atoi(date.Month)
	if err != nil {
		return sessions.DateKey{}, false
	}
	day, err := strconv.Atoi(date.Day)
	if err != nil {
		return sessions.DateKey{}, false
	}
	current := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	if current.Year() != year || int(current.Month()) != month || current.Day() != day {
		return sessions.DateKey{}, false
	}
	prev := current.AddDate(0, 0, -1)
	return sessions.DateKey{
		Year:  fmt.Sprintf("%04d", prev.Year()),
		Month: fmt.Sprintf("%02d", int(prev.Month())),
		Day:   fmt.Sprintf("%02d", prev.Day()),
	}, true
}

func dirLabel(cwd string) string {
	if sessions.NormalizeCwd(cwd) == sessions.UnknownCwd {
		return "Unknown (no CWD)"
	}
	return cwd
}

func displayCwd(cwd string) string {
	if sessions.NormalizeCwd(cwd) == sessions.UnknownCwd {
		return ""
	}
	return cwd
}

func (s *Server) recentCwdCounts(since time.Time) (map[string]int, int) {
	counts := map[string]int{}
	max := 0
	for _, date := range s.idx.Dates() {
		files := s.idx.SessionsByDate(date)
		for _, file := range files {
			if file.ModTime.Before(since) {
				continue
			}
			cwd := sessions.CwdForFile(file)
			counts[cwd]++
			if counts[cwd] > max {
				max = counts[cwd]
			}
		}
	}
	return counts, max
}

func (s *Server) recentCwdCountsFromLatestDates(limit int) (map[string]int, int) {
	counts := map[string]int{}
	max := 0
	if limit <= 0 {
		return counts, max
	}
	dates := s.idx.Dates()
	if len(dates) > limit {
		dates = dates[:limit]
	}
	for _, date := range dates {
		files := s.idx.SessionsByDate(date)
		for _, file := range files {
			cwd := sessions.CwdForFile(file)
			counts[cwd]++
			if counts[cwd] > max {
				max = counts[cwd]
			}
		}
	}
	return counts, max
}

func parseHeatMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "today":
		return "today"
	case "1h", "1hr", "1hour":
		return "1h"
	case "7d", "week", "7days":
		return "7d"
	default:
		return "7d"
	}
}

func heatColor(count int, max int) template.CSS {
	const (
		hotR     = 210
		hotG     = 55
		hotB     = 50
		alphaMin = 0.25
		alphaMax = 0.92
	)
	if max <= 0 || count <= 0 {
		return template.CSS("")
	}
	ratio := float64(count) / float64(max)
	if ratio > 1 {
		ratio = 1
	}
	alpha := alphaMin + (alphaMax-alphaMin)*ratio
	alpha = math.Max(alphaMin, math.Min(alpha, alphaMax))
	return template.CSS(fmt.Sprintf("rgba(%d, %d, %d, %.3f)", hotR, hotG, hotB, alpha))
}

func normalizeCwdParam(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "%") {
		if decoded, err := url.QueryUnescape(value); err == nil {
			value = decoded
		}
	}
	return value
}

func normalizeSearchCwdFilter(value string) string {
	value = normalizeCwdParam(value)
	if value == "" {
		return ""
	}
	if value != "/" && strings.HasSuffix(value, "/") {
		value = strings.TrimRight(value, "/")
	}
	if value != "\\" && strings.HasSuffix(value, "\\") {
		value = strings.TrimRight(value, "\\")
	}
	return value
}

func buildResumeCommand(meta *sessions.SessionMeta) string {
	if meta == nil || meta.ID == "" {
		return ""
	}
	if meta.Cwd != "" {
		return fmt.Sprintf("cd %s\ncodex resume %s", shellQuote(meta.Cwd), meta.ID)
	}
	return fmt.Sprintf("codex resume %s", meta.ID)
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (s *Server) buildSessionView(parts []string) (sessionPageView, error) {
	date, ok := sessions.ParseDate(parts[0], parts[1], parts[2])
	if !ok {
		return sessionPageView{}, errors.New("invalid date")
	}
	filename := parts[3]
	if filename == "" || strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return sessionPageView{}, errors.New("invalid filename")
	}

	file, ok := s.idx.Lookup(date, filename)
	if !ok {
		return sessionPageView{}, errors.New("file not found")
	}

	session, err := sessions.ParseSession(file.Path)
	if err != nil {
		return sessionPageView{}, err
	}

	items := make([]itemView, 0, len(session.Items))
	lastUserLine := 0
	lastAnyUserLine := 0
	lastAgentLine := 0
	lastItemLine := 0
	for _, item := range session.Items {
		autoCtx := item.Role == "user" && sessions.IsAutoContextUserMessage(item.Content)
		turnAbortedMessage, isTurnAborted := "", false
		if autoCtx {
			if msg, ok := sessions.ExtractTurnAbortedMessage(item.Content); ok {
				turnAbortedMessage = msg
				isTurnAborted = true
			}
		}
		renderText := item.Content
		if autoCtx && !isTurnAborted {
			renderText = escapeAutoContextTags(renderText)
		}
		view := itemView{
			Line:      item.Line,
			Timestamp: item.Timestamp,
			Type:      item.Type,
			Subtype:   item.Subtype,
			Role:      item.Role,
			Title:     item.Title,
			Content:   item.Content,
			Class:     item.Class,
			Markdown:  renderItemMarkdown(item),
			HTML:      markdownToHTML(renderText),
		}
		if autoCtx {
			view.AutoCtx = true
			view.Class = strings.TrimSpace(view.Class + " auto-context")
		}
		if isTurnAborted {
			view.IsTurnAborted = true
			view.TurnAbortedMessage = turnAbortedMessage
		}
		if item.Role == "user" {
			lastAnyUserLine = item.Line
			if !autoCtx {
				lastUserLine = item.Line
			}
		}
		if item.Role == "assistant" {
			lastAgentLine = item.Line
		}
		lastItemLine = item.Line
		items = append(items, view)
	}
	if lastUserLine == 0 {
		lastUserLine = lastAnyUserLine
	}

	cwdNav := buildSessionNav(s.idx, file)

	view := sessionPageView{
		Date: dateView{
			Label: date.String(),
			Path:  date.Path(),
			Count: 0,
		},
		File: sessionView{
			Name:        file.Name,
			Size:        formatBytes(file.Size),
			ModTime:     formatTime(file.ModTime),
			ModTimeOnly: formatTimeOnly(file.ModTime),
			Cwd:         displayCwd(sessions.CwdForFile(file)),
			DateLabel:   date.String(),
			DatePath:    date.Path(),
		},
		Meta:          session.Meta,
		Items:         items,
		AllMarkdown:   renderSessionMarkdown(session.Items),
		ResumeCommand: buildResumeCommand(session.Meta),
		ThemeClass:    s.themeClass,
		IsJSONL:       strings.HasSuffix(strings.ToLower(file.Name), ".jsonl"),
		LastUserLine:  lastUserLine,
		LastAgentLine: lastAgentLine,
		LastItemLine:  lastItemLine,
		CwdNav:        cwdNav,
	}
	return view, nil
}

func buildSessionNav(idx *sessions.Index, current sessions.SessionFile) *sessionNavView {
	cwd := sessions.CwdForFile(current)
	if sessions.NormalizeCwd(cwd) == sessions.UnknownCwd {
		return nil
	}
	files := idx.SessionsByCwd(cwd)
	if len(files) < 2 {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].ModTime.Equal(files[j].ModTime) {
			dateI := files[i].Date.String()
			dateJ := files[j].Date.String()
			if dateI != dateJ {
				return dateI < dateJ
			}
			return files[i].Name < files[j].Name
		}
		return files[i].ModTime.Before(files[j].ModTime)
	})
	currentIndex := -1
	for i := range files {
		if files[i].Path == current.Path {
			currentIndex = i
			break
		}
	}
	if currentIndex == -1 {
		return nil
	}
	nav := &sessionNavView{CwdLabel: dirLabel(cwd)}
	if currentIndex > 0 {
		nav.Prev = buildSessionNavLink(files[currentIndex-1])
	}
	if currentIndex+1 < len(files) {
		nav.Next = buildSessionNavLink(files[currentIndex+1])
	}
	if nav.Prev == nil && nav.Next == nil {
		return nil
	}
	return nav
}

func buildSessionNavLink(file sessions.SessionFile) *sessionNavLink {
	title := fmt.Sprintf("%s / %s", file.Date.String(), file.Name)
	mod := formatTime(file.ModTime)
	if mod != "" {
		title = title + " (" + mod + ")"
	}
	return &sessionNavLink{
		Path:  "/" + file.Date.Path() + "/" + file.Name + "#last-user",
		Title: title,
	}
}

func themeClass(theme int) string {
	switch theme {
	case 1:
		return "theme-noir-blue"
	case 2:
		return "theme-espresso-amber"
	case 3:
		return "theme-graphite-teal"
	case 4:
		return "theme-obsidian-lime"
	case 5:
		return "theme-ink-rose"
	case 6:
		return "theme-iron-cyan"
	default:
		return "theme-graphite-teal"
	}
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func formatUUID(token string) string {
	if len(token) != 32 {
		return token
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", token[0:8], token[8:12], token[12:16], token[16:20], token[20:32])
}

func (s *Server) buildShareURL(r *http.Request, filename string) string {
	if s.useTailscale && s.tailscaleHost != "" {
		return fmt.Sprintf("https://%s/%s", s.tailscaleHost, filename)
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	host := r.Host
	hostName := host
	if strings.Contains(host, ":") {
		if parsedHost, _, err := net.SplitHostPort(host); err == nil {
			hostName = parsedHost
		}
	}

	if s.shareAddr != "" {
		if strings.HasPrefix(s.shareAddr, ":") {
			host = hostName + s.shareAddr
		} else {
			host = s.shareAddr
		}
	}
	return fmt.Sprintf("%s://%s/%s", scheme, host, filename)
}

func renderItemMarkdown(item sessions.RenderItem) string {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = "Message"
	}
	content := strings.TrimSpace(item.Content)
	if content == "" {
		content = "(empty)"
	}
	return fmt.Sprintf("## %s\n\n%s\n", title, content)
}

func renderSessionMarkdown(items []sessions.RenderItem) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, renderItemMarkdown(item))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n")) + "\n"
}

func escapeAutoContextTags(text string) string {
	replacer := strings.NewReplacer(
		"<INSTRUCTIONS>", "&lt;INSTRUCTIONS&gt;",
		"</INSTRUCTIONS>", "&lt;/INSTRUCTIONS&gt;",
		"<environment_context>", "&lt;environment_context&gt;",
		"</environment_context>", "&lt;/environment_context&gt;",
		"<turn_aborted>", "&lt;turn_aborted&gt;",
		"</turn_aborted>", "&lt;/turn_aborted&gt;",
	)
	return replacer.Replace(text)
}

var markdownEngine = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
)

func markdownToHTML(text string) template.HTML {
	var buf bytes.Buffer
	if err := markdownEngine.Convert([]byte(text), &buf); err != nil {
		return template.HTML(html.EscapeString(text))
	}
	return template.HTML(buf.String())
}
