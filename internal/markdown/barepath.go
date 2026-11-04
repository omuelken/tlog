package markdown

import (
	"regexp"
	"strings"
)

// A path written straight into a block's text — ~/work/tlog, /etc/hosts,
// ./draft.md, ../reports — standing on its own between spaces. Like a bare
// web address it is ordinary text, nothing about storage changes for it, and
// it is found here only so that every surface shows it alike. Whether it is a
// link at all depends on whether anything is there, which this package cannot
// know: "and/or" and "/todo" are prose. The caller checks.
var barePathRe = regexp.MustCompile(`(?:^|\s)((?:~/|\.\.?/|/)[^\s<>()\[\]` + "`" + `]*)`)

// BarePaths returns the paths standing on their own in s, in order. Paths
// inside code, inside [[links]] and inside markdown links are not among them.
func BarePaths(s string) []URLLink {
	blank := []byte(s)
	clear := func(a, b int) {
		for i := a; i < b && i < len(blank); i++ {
			if blank[i] != '\n' {
				blank[i] = ' '
			}
		}
	}
	for _, l := range URLLinks(s) {
		clear(l.Start, l.End)
	}
	for _, loc := range regexp.MustCompile(`\[\[[^\]]*\]\]|`+"`[^`\n]*`").FindAllStringIndex(s, -1) {
		clear(loc[0], loc[1])
	}
	var out []URLLink
	for _, m := range barePathRe.FindAllStringSubmatchIndex(string(blank), -1) {
		start, end := m[2], m[3]
		for end > start && strings.ContainsRune(".,;:!?'\"", rune(s[end-1])) {
			end-- // the sentence's punctuation, not the path's
		}
		p := s[start:end]
		if strings.TrimLeft(p, "~./") == "" {
			continue // a lone / or ./ names nothing anyone meant
		}
		out = append(out, URLLink{Label: p, URL: p, Bare: true, Start: start, End: end})
	}
	return out
}
