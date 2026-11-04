package markdown

import (
	"reflect"
	"testing"
)

func TestBarePaths(t *testing.T) {
	got := func(s string) []string {
		var out []string
		for _, l := range BarePaths(s) {
			if s[l.Start:l.End] != l.URL {
				t.Fatalf("%q: span %d-%d is %q, not %q", s, l.Start, l.End, s[l.Start:l.End], l.URL)
			}
			out = append(out, l.URL)
		}
		return out
	}
	for s, want := range map[string][]string{
		"see ~/work/tlog.":                    {"~/work/tlog"},
		"/etc/hosts and ./draft.md, ../x":     {"/etc/hosts", "./draft.md", "../x"},
		"and/or is prose":                     nil,
		"a lone / is nothing, nor ./":         nil,
		"`~/in/code` and [[~/a page]]":        nil,
		"[label](~/linked) and https://x.y/z": nil,
		"two lines\n/first\n  ~/second":       {"/first", "~/second"},
	} {
		if g := got(s); !reflect.DeepEqual(g, want) {
			t.Errorf("%q: got %v, want %v", s, g, want)
		}
	}
}
