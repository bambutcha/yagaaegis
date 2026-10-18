package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPDFJavaScriptCaution(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pdf")
	// минимальный PDF с /JS
	pdf := "%PDF-1.3\n1 0 obj<< /Type /Catalog /OpenAction 2 0 R >>endobj\n" +
		"2 0 obj<< /S /JavaScript /JS (app.alert\\('hi'\\);) >>endobj\n%%EOF\n"
	if err := os.WriteFile(path, []byte(pdf), 0o644); err != nil {
		t.Fatal(err)
	}
	risks, err := InspectDocRisks(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(risks) == 0 {
		t.Fatal("expected PDF script risk")
	}
	if risks[0].Kind != "pdf-js" {
		t.Fatalf("kind=%s", risks[0].Kind)
	}
}

func TestPlainTextNoRisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	risks, err := InspectDocRisks(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(risks) != 0 {
		t.Fatalf("unexpected risks: %+v", risks)
	}
}

func TestScriptFileRisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.js")
	if err := os.WriteFile(path, []byte("alert(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	risks, err := InspectDocRisks(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(risks) != 1 || risks[0].Kind != "script-file" {
		t.Fatalf("%+v", risks)
	}
}

func TestCleanPDFNoRisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.pdf")
	pdf := "%PDF-1.3\n1 0 obj<< /Type /Catalog /Pages 2 0 R >>endobj\n%%EOF\n"
	if err := os.WriteFile(path, []byte(pdf), 0o644); err != nil {
		t.Fatal(err)
	}
	risks, err := InspectDocRisks(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(risks) != 0 {
		t.Fatalf("unexpected: %+v", risks)
	}
}
