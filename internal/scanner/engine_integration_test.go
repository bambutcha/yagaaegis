package scanner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestBundledRuntimeScan — полный прогон вшитого runtime из dist/ (без системного clamscan).
func TestBundledRuntimeScan(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux dist only")
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "../.."))
	rt := filepath.Join(root, "dist", "linux-amd64", "runtime", "linux-amd64")
	seed := filepath.Join(root, "dist", "linux-amd64", "seed")
	clam := filepath.Join(rt, "clamscan")
	certs := filepath.Join(rt, "certs")
	if _, err := os.Stat(clam); err != nil {
		t.Skip("нет dist/linux-amd64 — сначала ./scripts/package.sh")
	}
	if _, err := os.Stat(filepath.Join(seed, "main.cvd")); err != nil {
		t.Skip("нет seed CVD")
	}

	// Убеждаемся, что системный clamscan не нужен
	t.Setenv("PATH", "/usr/bin:/bin")

	eng := &Engine{
		Binary:      clam,
		DatabaseDir: seed,
		CertsDir:    certs,
		log:         nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ver, err := eng.Version(ctx)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if ver == "" {
		t.Fatal("пустая версия")
	}
	t.Log("version:", ver)

	clean := filepath.Join(t.TempDir(), "clean.txt")
	if err := os.WriteFile(clean, []byte("localguard clean"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := eng.ScanFile(ctx, clean)
	if res.Status != StatusClean {
		t.Fatalf("clean: status=%s msg=%s", res.Status, res.Message)
	}

	eicar := filepath.Join(t.TempDir(), "eicar.com")
	payload := []byte(`X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`)
	if err := os.WriteFile(eicar, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	res = eng.ScanFile(ctx, eicar)
	if res.Status != StatusInfected {
		t.Fatalf("eicar: status=%s msg=%s threat=%s", res.Status, res.Message, res.Threat)
	}
	t.Log("threat:", res.Threat)
}
