package scanner

import (
	"os"
	"path/filepath"
	"strings"
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

func TestClassifyLimitHits(t *testing.T) {
	hits := classifyHits([]Hit{
		{Path: "/a.exe", Threat: "Heuristics.Limits.Exceeded.MaxFileSize", Kind: "virus"},
		{Path: "/b.com", Threat: "Eicar-Test-Signature", Kind: "virus"},
	})
	if hits[0].Kind != "limit" {
		t.Fatalf("kind0=%s", hits[0].Kind)
	}
	if !strings.Contains(hits[0].Threat, "лимит") {
		t.Fatalf("ru=%s", hits[0].Threat)
	}
	vir := virusHits(hits)
	if len(vir) != 1 || vir[0].Threat != "Eicar-Test-Signature" {
		t.Fatalf("%+v", vir)
	}
}
