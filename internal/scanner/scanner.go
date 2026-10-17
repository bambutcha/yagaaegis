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

// Result — ответ UI после локального скана.
type Result struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Status     string `json:"status"`
	Threat     string `json:"threat,omitempty"`
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
	CertsDir    string // CVD CA (ClamAV 1.4+), рядом с вшитым runtime
	log         *slog.Logger
}

// DetectEngine находит движок и каталог баз приложения.
func DetectEngine(log *slog.Logger) (*Engine, error) {
	if log == nil {
		log = slog.Default()
	}

	// Вшитые базы из установщика → в user config (один раз).
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
		log.Warn("базы сигнатур отсутствуют — нужна кнопка «Обновить базы» или seed в установщике", "dir", dbDir)
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

// ScanFile проверяет файл только локально.
func (e *Engine) ScanFile(ctx context.Context, path string) Result {
	start := time.Now()
	log := e.log
	if log == nil {
		log = slog.Default()
	}

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
	if info.IsDir() {
		res.Status = StatusError
		res.Message = "указана папка; выберите файл"
		return res
	}

	if e.DatabaseDir == "" || !sigdb.HasDatabases(e.DatabaseDir) {
		res.Status = StatusUnavailable
		res.Message = "Нет баз сигнатур. Нажмите «Обновить базы» (зеркало для РФ) или переустановите с seed."
		return res
	}

	hash, err := FileSHA256(abs)
	if err != nil {
		res.Status = StatusError
		res.Message = fmt.Sprintf("хэш: %v", err)
		return res
	}
	res.SHA256 = hash

	args := []string{"--no-summary", "--infected", "-d", e.DatabaseDir}
	if e.CertsDir != "" {
		args = append(args, "--cvdcertsdir="+e.CertsDir)
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
		res.Message = "Угроз не найдено (локальный движок)"
		return res
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.ExitCode() {
		case 1:
			res.Status = StatusInfected
			res.Threat = parseThreat(text, abs)
			res.Message = "Обнаружена угроза"
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

func parseThreat(output, path string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "FOUND") {
			line = strings.TrimPrefix(line, path+":")
			line = strings.TrimSpace(line)
			return strings.TrimSuffix(line, " FOUND")
		}
		if i := strings.Index(line, ": "); i >= 0 && strings.HasSuffix(line, "FOUND") {
			return strings.TrimSpace(strings.TrimSuffix(line[i+2:], "FOUND"))
		}
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
