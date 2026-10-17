// Package uninstall — удаление LocalGuard на Linux и Windows.
package uninstall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Options — что снести.
type Options struct {
	// InstallDir — папка с LocalGuard (обычно рядом с Uninstall).
	InstallDir string
	// KeepUserData — не трогать ~/.config/localguard (базы CVD и т.п.).
	KeepUserData bool
}

// Progress — опциональный колбэк прогресса.
type Progress func(fraction float64, status string)

// UserDataDir — каталог данных приложения (базы сигнатур и т.д.).
func UserDataDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "localguard")
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

	if !opt.KeepUserData {
		step(0.2, "данные пользователя")
		if ud := UserDataDir(); ud != "" && !looksDangerous(ud) {
			_ = os.RemoveAll(ud)
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
			_ = os.Remove(filepath.Join(desktop, "LocalGuard.lnk"))
		}
		base := os.Getenv("AppData")
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, "AppData", "Roaming")
		}
		_ = os.Remove(filepath.Join(base, "Microsoft", "Windows", "Start Menu", "Programs", "LocalGuard.lnk"))
		return nil
	}
	// Linux: .desktop если ставили
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	for _, p := range []string{
		filepath.Join(home, ".local", "share", "applications", "localguard.desktop"),
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
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("localguard-uninst-%d.bat", time.Now().UnixNano()))
	body := fmt.Sprintf("@echo off\r\n"+
		"ping 127.0.0.1 -n 3 >nul\r\n"+
		"rd /s /q \"%s\"\r\n"+
		"del \"%%~f0\"\r\n",
		escapeCmd(installDir))
	if err := os.WriteFile(tmp, []byte(body), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("cmd", "/C", "start", "", "/MIN", tmp)
	return cmd.Start()
}

func escapeCmd(s string) string {
	return strings.ReplaceAll(s, `"`, `""`)
}
