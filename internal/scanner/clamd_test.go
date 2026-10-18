package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListScanFiles(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.txt"), []byte("y"), 0o644)
	files, err := listScanFiles(dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d", len(files))
	}
}

func TestScanWorkerCount(t *testing.T) {
	n := scanWorkerCount()
	if n < 2 || n > 12 {
		t.Fatalf("%d", n)
	}
}
