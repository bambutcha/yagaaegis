package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/bambutcha/localguard/internal/scanner"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails backend.
type App struct {
	ctx    context.Context
	log    *slog.Logger
	engine *scanner.Engine
}

func NewApp(log *slog.Logger) *App {
	if log == nil {
		log = slog.Default()
	}
	return &App{log: log}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if eng, err := scanner.DetectEngine(a.log); err == nil {
		a.engine = eng
	} else {
		a.log.Warn("clamav not available at startup", "err", err)
	}
}

func (a *App) baseCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// EngineStatus reports whether a local ClamAV binary is available.
func (a *App) EngineStatus() map[string]any {
	out := map[string]any{
		"available": false,
		"offline":   true,
		"privacy":   "File bytes are never uploaded. Only local scanning.",
	}
	if a.engine == nil {
		eng, err := scanner.DetectEngine(a.log)
		if err != nil {
			out["message"] = err.Error()
			return out
		}
		a.engine = eng
	}
	ctx, cancel := context.WithTimeout(a.baseCtx(), 5*time.Second)
	defer cancel()
	ver, err := a.engine.Version(ctx)
	if err != nil {
		out["message"] = err.Error()
		return out
	}
	out["available"] = true
	out["binary"] = a.engine.Binary
	out["version"] = ver
	out["message"] = "Local ClamAV ready"
	return out
}

// PickFile opens a native file dialog and returns the selected path (or empty).
func (a *App) PickFile() (string, error) {
	return runtime.OpenFileDialog(a.baseCtx(), runtime.OpenDialogOptions{
		Title: "Select a file to scan locally",
		Filters: []runtime.FileFilter{
			{DisplayName: "Documents", Pattern: "*.pdf;*.doc;*.docx;*.odt;*.xls;*.xlsx;*.ppt;*.pptx"},
			{DisplayName: "Archives", Pattern: "*.zip;*.rar;*.7z;*.tar;*.gz"},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
}

// ScanPath scans a file on disk. Contents never leave the machine.
func (a *App) ScanPath(path string) scanner.Result {
	if a.engine == nil {
		eng, err := scanner.DetectEngine(a.log)
		if err != nil {
			return scanner.Result{
				Path:      path,
				Status:    scanner.StatusUnavailable,
				Offline:   true,
				Message:   err.Error(),
				ScannedAt: time.Now().UTC().Format(time.RFC3339),
			}
		}
		a.engine = eng
	}
	ctx, cancel := context.WithTimeout(a.baseCtx(), 10*time.Minute)
	defer cancel()
	return a.engine.ScanFile(ctx, path)
}

// AppInfo is shown in the UI header.
func (a *App) AppInfo() string {
	return "LocalGuard — local-only malware scan (files stay on your device)"
}
