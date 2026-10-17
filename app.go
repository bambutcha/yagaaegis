package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/bambutcha/localguard/internal/appupdate"
	"github.com/bambutcha/localguard/internal/scanner"
	"github.com/bambutcha/localguard/internal/sigdb"
	"github.com/bambutcha/localguard/internal/version"
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
	a.log.Info("старт LocalGuard", "version", version.Label())
	if eng, err := scanner.DetectEngine(a.log); err == nil {
		a.engine = eng
	} else {
		a.log.Warn("движок недоступен при старте", "err", err)
	}
}

func (a *App) baseCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) ensureEngine() error {
	if a.engine != nil {
		return nil
	}
	eng, err := scanner.DetectEngine(a.log)
	if err != nil {
		return err
	}
	a.engine = eng
	return nil
}

// AppVersion — SemVer для UI (из internal/version).
func (a *App) AppVersion() map[string]any {
	return map[string]any{
		"version": version.String,
		"label":   version.Label(),
	}
}

// CheckAppUpdate — есть ли новый релиз на GitHub (не базы ClamAV).
func (a *App) CheckAppUpdate() map[string]any {
	ctx, cancel := context.WithTimeout(a.baseCtx(), 25*time.Second)
	defer cancel()
	return updateResultMap(appupdate.Check(ctx))
}

// ApplyAppUpdate скачивает portable-пакет с GitHub и ставит поверх текущей папки.
func (a *App) ApplyAppUpdate() map[string]any {
	exe, err := os.Executable()
	if err != nil {
		return map[string]any{"ok": false, "message": err.Error()}
	}
	if resolved, e := filepath.EvalSymlinks(exe); e == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	ctx, cancel := context.WithTimeout(a.baseCtx(), 45*time.Minute)
	defer cancel()
	res := appupdate.Apply(ctx, dir)
	a.log.Info("apply update", "ok", res.OK, "latest", res.Latest, "msg", res.Message)
	return updateResultMap(res)
}

func updateResultMap(res appupdate.Result) map[string]any {
	return map[string]any{
		"ok":          res.OK,
		"current":     res.Current,
		"latest":      res.Latest,
		"hasUpdate":   res.HasUpdate,
		"url":         res.URL,
		"assetUrl":    res.AssetURL,
		"assetName":   res.AssetName,
		"message":     res.Message,
		"checkedAt":   res.CheckedAt,
		"needRestart": res.NeedRestart,
	}
}

// OpenURL открывает ссылку в системном браузере (страница релиза).
func (a *App) OpenURL(url string) {
	if url == "" {
		return
	}
	runtime.BrowserOpenURL(a.baseCtx(), url)
}

// EngineStatus — доступен ли локальный движок и базы.
func (a *App) EngineStatus() map[string]any {
	out := map[string]any{
		"available": false,
		"offline":   true,
		"dbReady":   false,
		"privacy":   "Содержимое файлов не загружается в сеть. Только локальный скан.",
		"appLabel":  version.Label(),
	}
	dbDir, err := sigdb.Dir()
	if err == nil {
		out["dbDir"] = dbDir
		out["dbReady"] = sigdb.HasDatabases(dbDir)
	}
	if err := a.ensureEngine(); err != nil {
		out["message"] = err.Error()
		return out
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
	if out["dbReady"] == true {
		out["message"] = "Локальный движок и базы готовы"
	} else {
		out["message"] = "Движок найден, но баз нет — нажмите «Обновить базы»"
	}
	return out
}

// UpdateDatabases качает CVD с зеркала (Microsoft). Файлы пользователя не отправляются.
func (a *App) UpdateDatabases() map[string]any {
	ctx, cancel := context.WithTimeout(a.baseCtx(), 20*time.Minute)
	defer cancel()
	res := sigdb.Update(ctx, a.log, nil)
	out := map[string]any{
		"ok":      res.OK,
		"dir":     res.Dir,
		"mirror":  res.Mirror,
		"message": res.Message,
		"files":   res.Files,
	}
	if res.OK {
		a.engine = nil
		_ = a.ensureEngine()
	}
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
	if err := a.ensureEngine(); err != nil {
		return scanner.Result{
			Path:      path,
			Status:    scanner.StatusUnavailable,
			Offline:   true,
			Message:   err.Error(),
			ScannedAt: time.Now().UTC().Format(time.RFC3339),
		}
	}
	ctx, cancel := context.WithTimeout(a.baseCtx(), 10*time.Minute)
	defer cancel()
	return a.engine.ScanFile(ctx, path)
}

// AppInfo — текст в шапке UI.
func (a *App) AppInfo() string {
	return "локальная проверка на вирусы — файлы остаются на устройстве"
}
