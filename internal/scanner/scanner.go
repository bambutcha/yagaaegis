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

// Status values returned to the UI.
const (
	StatusClean       = "clean"
	StatusInfected    = "infected"
	StatusError       = "error"
	StatusUnavailable = "unavailable"
)

// Result is returned to the UI after a local scan.
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

// Engine invokes a local ClamAV binary. File bytes are never uploaded.
type Engine struct {
	Binary string
	log    *slog.Logger
}

func DetectEngine(log *slog.Logger) (*Engine, error) {
	if log == nil {
		log = slog.Default()
	}
	for _, name := range []string{"clamscan", "clamdscan"} {
		path, err := exec.LookPath(name)
		if err == nil {
			log.Info("clamav engine detected", "binary", path)
			return &Engine{Binary: path, log: log}, nil
		}
	}
	return nil, errors.New("ClamAV not found (install clamav; clamscan must be on PATH)")
}

func (e *Engine) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, e.Binary, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("clamav version: %w", err)
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

// ScanFile scans one file on local disk only.
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
		res.Message = "path is a directory; pick a file"
		return res
	}

	hash, err := FileSHA256(abs)
	if err != nil {
		res.Status = StatusError
		res.Message = fmt.Sprintf("hash: %v", err)
		return res
	}
	res.SHA256 = hash

	log.Info("local scan start", "path", abs, "sha256", hash, "size", info.Size())

	cmd := exec.CommandContext(ctx, e.Binary, "--no-summary", "--infected", abs)
	out, err := cmd.CombinedOutput()
	res.DurationMs = time.Since(start).Milliseconds()
	text := strings.TrimSpace(string(out))

	if err == nil {
		res.Status = StatusClean
		res.Message = "No threats found (local ClamAV)"
		log.Info("local scan clean", "path", abs, "duration_ms", res.DurationMs)
		return res
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.ExitCode() {
		case 1:
			res.Status = StatusInfected
			res.Threat = parseThreat(text, abs)
			res.Message = "Threat detected by local ClamAV"
			log.Warn("local scan infected", "path", abs, "threat", res.Threat)
			return res
		default:
			res.Status = StatusError
			res.Message = firstLine(text)
			if res.Message == "" {
				res.Message = err.Error()
			}
			log.Error("local scan engine error", "path", abs, "err", res.Message)
			return res
		}
	}

	res.Status = StatusError
	res.Message = err.Error()
	log.Error("local scan failed", "path", abs, "err", err)
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
