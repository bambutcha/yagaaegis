package sigdb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasDatabases(t *testing.T) {
	dir := t.TempDir()
	if HasDatabases(dir) {
		t.Fatal("empty dir should be false")
	}
	if err := os.WriteFile(filepath.Join(dir, "main.cvd"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasDatabases(dir) {
		t.Fatal("expected true after main.cvd")
	}
}

func TestSeedFrom(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "daily.cvd"), []byte("daily"), 0o644); err != nil {
		t.Fatal(err)
	}
	// SeedFrom пишет в UserConfigDir — подменим через HOME
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := SeedFrom(src); err != nil {
		t.Fatal(err)
	}
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if !HasDatabases(dir) {
		t.Fatal("seed did not copy")
	}
}
