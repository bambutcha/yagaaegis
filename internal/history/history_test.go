package history

import (
	"path/filepath"
	"testing"
)

func TestAppendAndList(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	if err := Append(Entry{Path: "/a", Status: "clean"}); err != nil {
		t.Fatal(err)
	}
	if err := Append(Entry{Path: "/b", Status: "infected", Threat: "X", HitCount: 1}); err != nil {
		t.Fatal(err)
	}
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Path != "/b" {
		t.Fatalf("%+v", list)
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	list, err = List()
	if err != nil || len(list) != 0 {
		t.Fatalf("clear: %v %+v", err, list)
	}
}
