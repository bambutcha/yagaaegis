package appupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPayloadRootAndMerge(t *testing.T) {
	extract := t.TempDir()
	plat := filepath.Join(extract, "linux-amd64")
	if err := os.MkdirAll(filepath.Join(plat, "runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plat, "LocalGuard"), []byte("newbin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plat, "runtime", "clamscan"), []byte("scan"), 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := findPayloadRoot(extract)
	if err != nil {
		t.Fatal(err)
	}
	if root != plat {
		t.Fatalf("root=%s", root)
	}

	dst := t.TempDir()
	_ = os.WriteFile(filepath.Join(dst, "LocalGuard"), []byte("old"), 0o755)
	if err := mergePayload(root, dst); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dst, "LocalGuard"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "newbin" {
		t.Fatalf("got %q", b)
	}
	if _, err := os.Stat(filepath.Join(dst, "runtime", "clamscan")); err != nil {
		t.Fatal(err)
	}
}

func TestCompareSemVer(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "0.1.0", 0},
		{"0.1.1", "0.1.0", 1},
		{"0.2.0", "0.1.9", 1},
		{"1.0.0", "0.9.9", 1},
		{"0.1.0-alpha", "0.1.0", 0},
		{"v0.2.0", "0.1.0", 1},
		{"0.0.9", "0.1.0", -1},
	}
	for _, c := range cases {
		got := compareSemVer(c.a, c.b)
		if got != c.want {
			t.Fatalf("%s vs %s: got %d want %d", c.a, c.b, got, c.want)
		}
	}
}
