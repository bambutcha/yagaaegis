package scanner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bambutcha/localguard/internal/runtimebin"
	"github.com/bambutcha/localguard/internal/sigdb"
)

const (
	StatusClean       = "clean"
	StatusInfected    = "infected"
	StatusError       = "error"
	StatusUnavailable = "unavailable"
)

// Hit — одна находка при скане папки.
type Hit struct {
	Path   string `json:"path"`
	Threat string `json:"threat"`
}

// Result — ответ UI после локального скана.
type Result struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256,omitempty"`
	Status     string `json:"status"`
	Threat     string `json:"threat,omitempty"`
	Hits       []Hit  `json:"hits,omitempty"`
	IsDir      bool   `json:"isDir"`
	Engine     string `json:"engine"`
	ScannedAt  string `json:"scannedAt"`
	DurationMs int64  `json:"durationMs"`
	Message    string `json:"message,omitempty"`
	Offline    bool   `json:"offline"`
}

// Engine вызывает clamscan. Сначала вшитый runtime, потом системный PATH.
type Engine struct {
	Binary      string
	DatabaseDir string
	CertsDir    string
	log         *slog.Logger
}

// DetectEngine находит движок и каталог баз приложения.
func DetectEngine(log *slog.Logger) (*Engine, error) {
	if log == nil {
		log = slog.Default()
	}

	if seed := runtimebin.SeedDir(); seed != "" {
		if err := sigdb.SeedFrom(seed); err != nil {
			log.Debug("seed баз пропущен", "err", err)
		}
	}

	dbDir, err := sigdb.Dir()
	if err != nil {
		return nil, err
	}

	binary, err := runtimebin.ClamscanPath()
	if err != nil {
		for _, name := range []string{"clamscan", "clamdscan"} {
			path, lookErr := exec.LookPath(name)
			if lookErr == nil {
				binary = path
				err = nil
				break
			}
		}
	}
	if binary == "" {
		return nil, errors.New("движок не найден: нужен вшитый runtime/…/clamscan или системный ClamAV")
	}

	eng := &Engine{
		Binary:      binary,
		DatabaseDir: dbDir,
		CertsDir:    certsDirNextTo(binary),
		log:         log,
	}
	if !sigdb.HasDatabases(dbDir) {
		log.Warn("базы сигнатур отсутствуют", "dir", dbDir)
	}
	log.Info("движок готов", "binary", binary, "db", dbDir, "certs", eng.CertsDir)
	return eng, nil
}

func certsDirNextTo(binary string) string {
	dir := filepath.Join(filepath.Dir(binary), "certs")
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir
	}
	return ""
}

func (e *Engine) clamEnv() []string {
	libDir := filepath.Dir(e.Binary)
	env := append(os.Environ(), "LD_LIBRARY_PATH="+libDir+string(os.PathListSeparator)+os.Getenv("LD_LIBRARY_PATH"))
	if e.CertsDir != "" {
		env = append(env, "CVD_CERTS_DIR="+e.CertsDir)
	}
	return env
}

func (e *Engine) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, e.Binary, "--version")
	cmd.Dir = filepath.Dir(e.Binary)
	cmd.Env = e.clamEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return "", fmt.Errorf("версия clamav: %v (%s)", err, firstLine(detail))
		}
		return "", fmt.Errorf("версия clamav: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ScanPath проверяет файл или папку (рекурсивно) только локально.
func (e *Engine) ScanPath(ctx context.Context, path string) Result {
	start := time.Now()
	abs, err := filepath.Abs(path)
	res := Result{
		Path:      abs,
		Engine:    filepath.Base(e.Binary),
		ScannedAt: time.Now().UTC().Format(time.RFC3339),
		Offline:   true,
	}
	if err != nil {
		res.Status = StatusError
		res.Message = err.Error()
		return res
	}

	info, err := os.Stat(abs)
	if err != nil {
		res.Status = StatusError
		res.Message = err.Error()
		return res
	}
	res.IsDir = info.IsDir()

	if e.DatabaseDir == "" || !sigdb.HasDatabases(e.DatabaseDir) {
		res.Status = StatusUnavailable
		res.Message = "Нет баз сигнатур. Нажмите «Обновить базы» или переустановите с seed."
		return res
	}

	if !res.IsDir {
		hash, err := FileSHA256(abs)
		if err != nil {
			res.Status = StatusError
			res.Message = fmt.Sprintf("хэш: %v", err)
			return res
		}
		res.SHA256 = hash
	}

	args := []string{"--no-summary", "--infected", "-d", e.DatabaseDir}
	if e.CertsDir != "" {
		args = append(args, "--cvdcertsdir="+e.CertsDir)
	}
	if res.IsDir {
		args = append(args, "-r")
	}
	args = append(args, abs)

	cmd := exec.CommandContext(ctx, e.Binary, args...)
	cmd.Dir = filepath.Dir(e.Binary)
	cmd.Env = e.clamEnv()
	out, err := cmd.CombinedOutput()
	res.DurationMs = time.Since(start).Milliseconds()
	text := strings.TrimSpace(string(out))

	if err == nil {
		res.Status = StatusClean
		if res.IsDir {
			res.Message = "В папке угроз не найдено"
		} else {
			res.Message = "Угроз не найдено (локальный движок)"
		}
		return res
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.ExitCode() {
		case 1:
			res.Status = StatusInfected
			res.Hits = parseHits(text)
			if len(res.Hits) > 0 {
				res.Threat = res.Hits[0].Threat
				if res.IsDir {
					res.Message = fmt.Sprintf("Обнаружено угроз: %d", len(res.Hits))
				} else {
					res.Message = "Обнаружена угроза"
				}
			} else {
				res.Threat = parseThreat(text, abs)
				res.Message = "Обнаружена угроза"
			}
			return res
		default:
			res.Status = StatusError
			res.Message = firstLine(text)
			if res.Message == "" {
				res.Message = err.Error()
			}
			return res
		}
	}

	res.Status = StatusError
	res.Message = err.Error()
	return res
}

// ScanFile — алиас для одного файла (тесты / совместимость).
func (e *Engine) ScanFile(ctx context.Context, path string) Result {
	return e.ScanPath(ctx, path)
}

func parseHits(output string) []Hit {
	var hits []Hit
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasSuffix(line, "FOUND") {
			continue
		}
		// /path/to/file: Threat.Name FOUND
		i := strings.LastIndex(line, ": ")
		if i < 0 {
			continue
		}
		p := strings.TrimSpace(line[:i])
		threat := strings.TrimSpace(strings.TrimSuffix(line[i+2:], "FOUND"))
		hits = append(hits, Hit{Path: p, Threat: threat})
	}
	return hits
}

func parseThreat(output, path string) string {
	hits := parseHits(output)
	for _, h := range hits {
		if h.Path == path || strings.HasSuffix(h.Path, path) {
			return h.Threat
		}
	}
	if len(hits) > 0 {
		return hits[0].Threat
	}
	return "unknown"
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
