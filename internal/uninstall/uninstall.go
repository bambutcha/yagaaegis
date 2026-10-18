// Package uninstall — удаление YagaAegis на Linux и Windows.
package uninstall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bambutcha/yagaaegis/internal/winproc"
)

// Options — что снести.
type Options struct {
	// InstallDir — папка с YagaAegis (обычно рядом с Uninstall).
	InstallDir string
	// KeepUserData — не трогать ~/.config/yagaaegis (базы CVD и т.п.).
	KeepUserData bool
}

// Progress — опциональный колбэк прогресса.
type Progress func(fraction float64, status string)

// UserDataDir — каталог данных приложения (базы сигнатур и т.д.).
func UserDataDir() string {
	base := userConfigBase()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "yagaaegis")
}

func legacyUserDataDir() string {
	base := userConfigBase()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "localguard")
}

// userDataDirsToRemove — Roaming + Local AppData на Windows (NSIS all-context раньше чистил не то).
func userDataDirsToRemove() []string {
	dirs := []string{UserDataDir(), legacyUserDataDir()}
	if runtime.GOOS != "windows" {
		return dirs
	}
	home, _ := os.UserHomeDir()
	roaming := os.Getenv("APPDATA")
	if roaming == "" && home != "" {
		roaming = filepath.Join(home, "AppData", "Roaming")
	}
	local := os.Getenv("LOCALAPPDATA")
	if local == "" && home != "" {
		local = filepath.Join(home, "AppData", "Local")
	}
	for _, base := range []string{roaming, local} {
		if base == "" {
			continue
		}
		for _, name := range []string{"yagaaegis", "YagaAegis", "localguard", "LocalGuard", "YagaAegis.exe"} {
			dirs = append(dirs, filepath.Join(base, name))
		}
	}
	return dirs
}

func userConfigBase() string {
	base, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return ""
		}
		if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(home, ".config")
	}
	return base
}

// InstallDirFromExe — каталог установки = папка Uninstall[.exe].
func InstallDirFromExe() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, e := filepath.EvalSymlinks(exe); e == nil {
		exe = resolved
	}
	return filepath.Dir(exe), nil
}

// Run удаляет ярлыки, данные (если нужно) и файлы установки.
func Run(opt Options, progress Progress) error {
	dir := filepath.Clean(opt.InstallDir)
	if dir == "" || dir == "." || dir == string(filepath.Separator) {
		return fmt.Errorf("небезопасный или пустой каталог установки: %q", dir)
	}
	// защита от сноса корня диска / home
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if looksDangerous(dir) {
		return fmt.Errorf("отказ: подозрительный путь установки %q", dir)
	}

	step := func(f float64, s string) {
		if progress != nil {
			progress(f, s)
		}
	}

	step(0.05, "ярлыки")
	_ = removeShortcuts()

	step(0.1, "фоновый clamd")
	stopYagaClamd()

	if !opt.KeepUserData {
		step(0.2, "данные пользователя")
		for _, ud := range userDataDirsToRemove() {
			if ud != "" && !looksDangerous(ud) {
				_ = os.RemoveAll(ud)
			}
		}
	}

	step(0.4, "файлы приложения")
	self, _ := os.Executable()
	if self != "" {
		if resolved, err := filepath.EvalSymlinks(self); err == nil {
			self = resolved
		}
	}
	selfInInstall := false
	selfBase := ""
	if self != "" {
		selfDir := filepath.Clean(filepath.Dir(self))
		if selfDir == dir {
			selfInInstall = true
			selfBase = filepath.Base(self)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("чтение %s: %w", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if selfBase != "" && strings.EqualFold(name, selfBase) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(dir, name))
	}

	step(0.85, "завершение")
	if selfInInstall && runtime.GOOS == "windows" {
		if err := scheduleWindowsSelfDelete(dir); err != nil {
			return fmt.Errorf("отложенное удаление: %w", err)
		}
	} else if selfInInstall {
		_ = os.Remove(self)
		_ = os.Remove(dir) // пустая папка
	} else {
		_ = os.Remove(dir)
	}
	step(1, "готово")
	return nil
}

func stopYagaClamd() {
	base := userConfigBase()
	if base == "" {
		return
	}
	dir := filepath.Join(base, "yagaaegis", "clamd")
	pidPath := filepath.Join(dir, "clamd.pid")
	if b, err := os.ReadFile(pidPath); err == nil {
		var pid int
		for _, c := range string(b) {
			if c >= '0' && c <= '9' {
				pid = pid*10 + int(c-'0')
			}
		}
		if pid > 1 {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
			}
		}
	}
	_ = os.Remove(filepath.Join(dir, "clamd.sock"))
	_ = os.Remove(pidPath)
}

func looksDangerous(dir string) bool {
	clean := filepath.Clean(dir)
	if clean == "/" || clean == "." {
		return true
	}
	if runtime.GOOS == "windows" {
		// C:\ или C:
		if len(clean) <= 3 && strings.Contains(clean, ":") {
			return true
		}
	}
	home, _ := os.UserHomeDir()
	if home != "" && filepath.Clean(home) == clean {
		return true
	}
	return false
}

func removeShortcuts() error {
	if runtime.GOOS == "windows" {
		desktop := windowsDesktopDir()
		if desktop != "" {
			for _, name := range []string{"YagaAegis.lnk", "LocalGuard.lnk"} {
				_ = os.Remove(filepath.Join(desktop, name))
			}
		}
		base := os.Getenv("AppData")
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, "AppData", "Roaming")
		}
		start := filepath.Join(base, "Microsoft", "Windows", "Start Menu", "Programs")
		for _, name := range []string{"YagaAegis.lnk", "LocalGuard.lnk"} {
			_ = os.Remove(filepath.Join(start, name))
		}
		return nil
	}
	// Linux: .desktop если ставили
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	for _, p := range []string{
		filepath.Join(home, ".local", "share", "applications", "yagaaegis.desktop"),
		filepath.Join(home, ".local", "share", "applications", "localguard.desktop"),
		filepath.Join(home, "Desktop", "yagaaegis.desktop"),
		filepath.Join(home, "Desktop", "YagaAegis.desktop"),
		filepath.Join(home, "Desktop", "localguard.desktop"),
		filepath.Join(home, "Desktop", "LocalGuard.desktop"),
	} {
		_ = os.Remove(p)
	}
	return nil
}

func windowsDesktopDir() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Desktop")
}

func scheduleWindowsSelfDelete(installDir string) error {
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("yagaaegis-uninst-%d.bat", time.Now().UnixNano()))
	body := fmt.Sprintf("@echo off\r\n"+
		"ping 127.0.0.1 -n 3 >nul\r\n"+
		"rd /s /q \"%s\"\r\n"+
		"del \"%%~f0\"\r\n",
		escapeCmd(installDir))
	if err := os.WriteFile(tmp, []byte(body), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("cmd", "/C", "start", "", "/MIN", tmp)
	winproc.HideConsole(cmd)
	return cmd.Start()
}

func escapeCmd(s string) string {
	return strings.ReplaceAll(s, `"`, `""`)
}
