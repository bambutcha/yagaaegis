package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxEntries = 40

// Entry — одна запись истории сканов.
type Entry struct {
	Path       string `json:"path"`
	Status     string `json:"status"`
	Threat     string `json:"threat,omitempty"`
	Message    string `json:"message,omitempty"`
	IsDir      bool   `json:"isDir"`
	HitCount   int    `json:"hitCount,omitempty"`
	DurationMs int64  `json:"durationMs"`
	ScannedAt  string `json:"scannedAt"`
}

var mu sync.Mutex

func filePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	dir := filepath.Join(base, "yagaaegis")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "history.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		legacy := filepath.Join(base, "localguard", "history.json")
		if b, err := os.ReadFile(legacy); err == nil {
			_ = os.WriteFile(path, b, 0o644)
		}
	}
	return path, nil
}

// List возвращает записи (новые сверху).
func List() ([]Entry, error) {
	mu.Lock()
	defer mu.Unlock()
	return load()
}

// Append добавляет запись в начало.
func Append(e Entry) error {
	mu.Lock()
	defer mu.Unlock()
	if e.ScannedAt == "" {
		e.ScannedAt = time.Now().UTC().Format(time.RFC3339)
	}
	list, err := load()
	if err != nil {
		list = nil
	}
	list = append([]Entry{e}, list...)
	if len(list) > maxEntries {
		list = list[:maxEntries]
	}
	return save(list)
}

// Clear очищает историю.
func Clear() error {
	mu.Lock()
	defer mu.Unlock()
	return save(nil)
}

func load() ([]Entry, error) {
	path, err := filePath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var list []Entry
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func save(list []Entry) error {
	path, err := filePath()
	if err != nil {
		return err
	}
	if list == nil {
		list = []Entry{}
	}
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
