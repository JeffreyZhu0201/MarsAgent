package oj

import (
	"testing"

	"github.com/lib/pq"
)

func TestNormalizeOutput(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"trim trailing spaces", "hello  \n", "hello"},
		{"crlf to lf", "a\r\nb\r\n", "a\nb"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeOutput(tt.in); got != tt.want {
				t.Errorf("NormalizeOutput(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCompareOutput(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
		want     bool
	}{
		{"exact match", "42", "42", true},
		{"float tolerance", "1.0000001", "1.0", true},
		{"wrong answer", "1", "2", false},
		{"token count mismatch", "1 2", "1", false},
		{"non-numeric mismatch", "abc", "abd", false},
		{"whitespace normalized", " 1 \n", "1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp := NormalizeOutput(tt.expected)
			act := NormalizeOutput(tt.actual)
			if got := CompareOutput(exp, act, 1e-6); got != tt.want {
				t.Errorf("CompareOutput(%q, %q) = %v, want %v", exp, act, got, tt.want)
			}
		})
	}
}

func TestApplyProblemDefaults(t *testing.T) {
	d, tl, mem := applyProblemDefaults("", 0, 0)
	if d != "medium" || tl != 2000 || mem != 256 {
		t.Errorf("defaults = (%q, %d, %d), want (medium, 2000, 256)", d, tl, mem)
	}
	d, tl, mem = applyProblemDefaults("hard", 3000, 512)
	if d != "hard" || tl != 3000 || mem != 512 {
		t.Errorf("explicit = (%q, %d, %d)", d, tl, mem)
	}
}

func TestTagsFromScan(t *testing.T) {
	if got := tagsFromScan(nil); len(got) != 0 {
		t.Errorf("nil tags = %v", got)
	}
	if got := tagsFromScan(pq.StringArray{"a", "b"}); len(got) != 2 || got[0] != "a" {
		t.Errorf("tagsFromScan = %v", got)
	}
}
