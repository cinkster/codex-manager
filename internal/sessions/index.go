package sessions

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DateKey identifies a sessions date folder.
type DateKey struct {
	Year  string
	Month string
	Day   string
}

func (d DateKey) String() string {
	return d.Year + "-" + d.Month + "-" + d.Day
}

func (d DateKey) Path() string {
	return path.Join(d.Year, d.Month, d.Day)
}

// SessionFile represents a jsonl file on disk.
type SessionFile struct {
	Date    DateKey
	Name    string
	Path    string
	Size    int64
	ModTime time.Time
}

// Index stores a snapshot of sessions on disk.
type Index struct {
	baseDir string
	mu      sync.RWMutex
	byDate  map[DateKey][]SessionFile
	byName  map[string]SessionFile
	updated time.Time
}

// NewIndex creates an empty index.
func NewIndex(baseDir string) *Index {
	return &Index{baseDir: baseDir, byDate: map[DateKey][]SessionFile{}, byName: map[string]SessionFile{}}
}

// BaseDir returns the sessions root.
func (idx *Index) BaseDir() string {
	return idx.baseDir
}

// LastUpdated returns when Refresh last succeeded.
func (idx *Index) LastUpdated() time.Time {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.updated
}

// Refresh rescans the sessions directory.
func (idx *Index) Refresh() error {
	if idx.baseDir == "" {
		return errors.New("sessions base directory is empty")
	}
	if _, err := os.Stat(idx.baseDir); err != nil {
		return err
	}

	byDate := map[DateKey][]SessionFile{}
	byName := map[string]SessionFile{}

	walkErr := filepath.WalkDir(idx.baseDir, func(fullPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}

		rel, err := filepath.Rel(idx.baseDir, fullPath)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) != 4 {
			return nil
		}
		date, ok := ParseDate(parts[0], parts[1], parts[2])
		if !ok {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		file := SessionFile{
			Date:    date,
			Name:    parts[3],
			Path:    fullPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}

		byDate[date] = append(byDate[date], file)
		byName[path.Join(date.Path(), file.Name)] = file
		return nil
	})

	if walkErr != nil {
		return walkErr
	}

	for dateKey, files := range byDate {
		sort.Slice(files, func(i, j int) bool {
			if files[i].ModTime.Equal(files[j].ModTime) {
				return files[i].Name < files[j].Name
			}
			return files[i].ModTime.After(files[j].ModTime)
		})
		byDate[dateKey] = files
	}

	idx.mu.Lock()
	idx.byDate = byDate
	idx.byName = byName
	idx.updated = time.Now()
	idx.mu.Unlock()
	return nil
}

// Dates returns sorted date keys.
func (idx *Index) Dates() []DateKey {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	keys := make([]DateKey, 0, len(idx.byDate))
	for key := range idx.byDate {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return dateGreater(keys[i], keys[j])
	})
	return keys
}

// SessionsByDate returns sessions for a date.
func (idx *Index) SessionsByDate(date DateKey) []SessionFile {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	files := idx.byDate[date]
	out := make([]SessionFile, len(files))
	copy(out, files)
	return out
}

// Lookup returns the file for a date+name.
func (idx *Index) Lookup(date DateKey, filename string) (SessionFile, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	file, ok := idx.byName[path.Join(date.Path(), filename)]
	return file, ok
}

func ParseDate(year, month, day string) (DateKey, bool) {
	if len(year) != 4 || len(month) != 2 || len(day) != 2 {
		return DateKey{}, false
	}
	if !isDigits(year) || !isDigits(month) || !isDigits(day) {
		return DateKey{}, false
	}
	return DateKey{Year: year, Month: month, Day: day}, true
}

func isDigits(value string) bool {
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func dateGreater(a, b DateKey) bool {
	ay, _ := strconv.Atoi(a.Year)
	by, _ := strconv.Atoi(b.Year)
	if ay != by {
		return ay > by
	}
	am, _ := strconv.Atoi(a.Month)
	bm, _ := strconv.Atoi(b.Month)
	if am != bm {
		return am > bm
	}
	ad, _ := strconv.Atoi(a.Day)
	bd, _ := strconv.Atoi(b.Day)
	if ad != bd {
		return ad > bd
	}
	return a.String() > b.String()
}
