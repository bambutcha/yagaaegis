package appupdate

import "testing"

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
