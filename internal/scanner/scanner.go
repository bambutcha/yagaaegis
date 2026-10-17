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
)

// Статусы результата для UI (коды стабильные, подписи — на фронте).
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

// Engine вызывает локальный бинарник ClamAV. Байты файла никуда не грузятся.
type Engine struct {
	Binary string
	log    *slog.Logger
}

// DetectEngine ищет clamscan/clamdscan в PATH.
func DetectEngine(log *slog.Logger) (*Engine, error) {
	if log == nil {
		log = slog.Default()
	}
	for _, name := range []string{"clamscan", "clamdscan"} {
		path, err := exec.LookPath(name)
		if err == nil {
			log.Info("найден движок ClamAV", "binary", path)
			return &Engine{Binary: path, log: log}, nil
		}
	}
	return nil, errors.New("ClamAV не найден (установи clamav; clamscan должен быть в PATH)")
}

func (e *Engine) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, e.Binary, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("версия clamav: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// FileSHA256 считает отпечаток файла локально.
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

// ScanFile проверяет один файл только с диска.
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

	hash, err := FileSHA256(abs)
	if err != nil {
		res.Status = StatusError
		res.Message = fmt.Sprintf("хэш: %v", err)
		return res
	}
	res.SHA256 = hash

	log.Info("старт локального скана", "path", abs, "sha256", hash, "size", info.Size())

	cmd := exec.CommandContext(ctx, e.Binary, "--no-summary", "--infected", abs)
	out, err := cmd.CombinedOutput()
	res.DurationMs = time.Since(start).Milliseconds()
	text := strings.TrimSpace(string(out))

	if err == nil {
		res.Status = StatusClean
		res.Message = "Угроз не найдено (локальный ClamAV)"
		log.Info("скан чистый", "path", abs, "duration_ms", res.DurationMs)
		return res
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.ExitCode() {
		case 1:
			res.Status = StatusInfected
			res.Threat = parseThreat(text, abs)
			res.Message = "Обнаружена угроза (локальный ClamAV)"
			log.Warn("скан: угроза", "path", abs, "threat", res.Threat)
			return res
		default:
			res.Status = StatusError
			res.Message = firstLine(text)
			if res.Message == "" {
				res.Message = err.Error()
			}
			log.Error("ошибка движка", "path", abs, "err", res.Message)
			return res
		}
	}

	res.Status = StatusError
	res.Message = err.Error()
	log.Error("сбой скана", "path", abs, "err", err)
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
