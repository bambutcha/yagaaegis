package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/bambutcha/stillroom/internal/scanner"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App — бэкенд Wails, методы вызываются из React.
type App struct {
	ctx    context.Context
	log    *slog.Logger
	engine *scanner.Engine
}

// NewApp создаёт приложение с логгером.
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
		a.log.Warn("ClamAV недоступен при старте", "err", err)
	}
}

func (a *App) baseCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// EngineStatus — доступен ли локальный ClamAV.
func (a *App) EngineStatus() map[string]any {
	out := map[string]any{
		"available": false,
		"offline":   true,
		"privacy":   "Содержимое файлов не загружается в сеть. Только локальный скан.",
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
	out["message"] = "Локальный ClamAV готов"
	return out
}

// PickFile открывает системный диалог выбора файла.
func (a *App) PickFile() (string, error) {
	return runtime.OpenFileDialog(a.baseCtx(), runtime.OpenDialogOptions{
		Title: "Выберите файл для локальной проверки",
		Filters: []runtime.FileFilter{
			{DisplayName: "Документы", Pattern: "*.pdf;*.doc;*.docx;*.odt;*.xls;*.xlsx;*.ppt;*.pptx"},
			{DisplayName: "Архивы", Pattern: "*.zip;*.rar;*.7z;*.tar;*.gz"},
			{DisplayName: "Все файлы", Pattern: "*.*"},
		},
	})
}

// ScanPath проверяет файл на диске. Содержимое не уходит в сеть.
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

// AppInfo — текст в шапке UI.
func (a *App) AppInfo() string {
	return "Stillroom — локальная проверка на вирусы (файлы остаются на устройстве)"
}
