package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(path, []byte("yagaaegis"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := FileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum) != 64 {
		t.Fatalf("unexpected hash length: %d", len(sum))
	}
}

func TestFolderThreatLabel(t *testing.T) {
	one := folderThreatLabel([]Hit{{Threat: "Eicar-Test-Signature"}, {Threat: "Eicar-Test-Signature"}})
	if one != "Eicar-Test-Signature (×2)" {
		t.Fatalf("got %q", one)
	}
	multi := folderThreatLabel([]Hit{
		{Threat: "Eicar-Test-Signature"},
		{Threat: "Win.Malware.Nymeria-10036360-0"},
	})
	if multi != "Eicar-Test-Signature · Win.Malware.Nymeria-10036360-0" {
		t.Fatalf("got %q", multi)
	}
}

func TestParseScanCounts(t *testing.T) {
	out := "foo\nScanned files: 7\nInfected files: 5\n"
	s, i := parseScanCounts(out)
	if s != 7 || i != 5 {
		t.Fatalf("scanned=%d infected=%d", s, i)
	}
}
