package markdown

import "testing"

func TestURLLinksFindsEveryForm(t *testing.T) {
	s := "see [the docs](https://go.dev/doc) and ![plan](file:///home/a/plan.png), " +
		"or https://example.com/x. Also [[Page]](not a link)"
	got := URLLinks(s)
	want := []URLLink{
		{Label: "the docs", URL: "https://go.dev/doc"},
		{Label: "plan", URL: "file:///home/a/plan.png", Image: true},
		{Label: "https://example.com/x", URL: "https://example.com/x", Bare: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, w := range want {
		g := got[i]
		if g.Label != w.Label || g.URL != w.URL || g.Image != w.Image || g.Bare != w.Bare {
			t.Errorf("link %d: got %+v, want %+v", i, g, w)
		}
		// The span covers exactly the link, so it can be cut out and redrawn.
		if i < 2 && s[g.Start] != '[' && s[g.Start] != '!' {
			t.Errorf("link %d starts at %q", i, s[g.Start:])
		}
		if g.Bare && s[g.Start:g.End] != g.URL {
			t.Errorf("bare span %q", s[g.Start:g.End])
		}
	}
}

func TestURLLinksIgnoresCodeSamplesInABlock(t *testing.T) {
	b := &Block{Text: "real [a](https://a.example)\n```\n[b](https://b.example)\n```"}
	got := b.URLLinks()
	if len(got) != 1 || got[0].URL != "https://a.example" {
		t.Fatalf("got %+v", got)
	}
}
