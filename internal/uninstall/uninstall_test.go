package uninstall

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunRemovesInstallAndData(t *testing.T) {
	root := t.TempDir()
	install := filepath.Join(root, "LocalGuardApp")
	if err := os.MkdirAll(filepath.Join(install, "runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(install, "LocalGuard"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(install, "seed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(install, "seed", "main.cvd"), []byte("db"), 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", filepath.Join(home, "AppData"))
	}
	ud := UserDataDir()
	if err := os.MkdirAll(filepath.Join(ud, "clamav"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(ud, "clamav", "main.cvd"), []byte("x"), 0o644)

	if err := Run(Options{InstallDir: install, KeepUserData: false}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(install, "LocalGuard")); !os.IsNotExist(err) {
		t.Fatalf("app file still exists: %v", err)
	}
	if _, err := os.Stat(ud); !os.IsNotExist(err) {
		t.Fatalf("user data still exists: %v", err)
	}
}

func TestKeepUserData(t *testing.T) {
	root := t.TempDir()
	install := filepath.Join(root, "app")
	_ = os.MkdirAll(install, 0o755)
	_ = os.WriteFile(filepath.Join(install, "LocalGuard"), []byte("x"), 0o755)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	ud := UserDataDir()
	_ = os.MkdirAll(ud, 0o755)
	marker := filepath.Join(ud, "keep-me")
	_ = os.WriteFile(marker, []byte("1"), 0o644)

	if err := Run(Options{InstallDir: install, KeepUserData: true}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("user data should remain")
	}
}

func TestLooksDangerous(t *testing.T) {
	if !looksDangerous("/") {
		t.Fatal("root should be dangerous")
	}
}
