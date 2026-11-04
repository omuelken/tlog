package app

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/tilman-schieber/tlog/internal/markdown"
)

// Paths written in a note: ~/work/tlog, /etc/hosts, ../reports. One standing
// on its own is a link only when something is there — "and/or" and "/todo"
// are prose — so the read model says which ones are, and the adapters draw
// those as links and open them like any other: a folder in a terminal or
// Finder, a file in its own application.

// LivePaths is the bare paths in a block's text that lead somewhere, as
// written. rel is the note they are written in, which a relative path is
// relative to. Reading only: nothing is created or written.
func (s *Service) LivePaths(rel, text string) []string {
	var out []string
	for _, l := range markdown.BarePaths(text) {
		if _, err := s.ResolveLink(rel, l.URL); err == nil {
			out = append(out, l.URL)
		}
	}
	return out
}

// PastedPath is what to put in a block for pasted text that names one folder
// or file that exists: the path itself when it can stand in the text as it is,
// or a markdown link when it cannot — a path with a space would otherwise stop
// being a path at the space. It reads what terminals and Finder put on the
// clipboard: a plain path, one escaped or quoted for a shell, or a file://
// address. A path under the home folder is written from ~, so the note means
// the same on another machine with another user name.
func (s *Service) PastedPath(text string) (string, bool) {
	t := strings.TrimSpace(text)
	if t == "" || strings.Contains(t, "\n") {
		return "", false
	}
	var tries []string
	if strings.HasPrefix(t, "file://") {
		if u, err := url.Parse(t); err == nil {
			tries = append(tries, u.Path)
		}
	} else {
		tries = append(tries, t)
		if w := shellWords(t); len(w) == 1 {
			tries = append(tries, w[0])
		}
	}
	for _, p := range tries {
		if strings.HasPrefix(p, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				p = filepath.Join(home, p[2:])
			}
		}
		if !filepath.IsAbs(p) {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			continue
		}
		return pathText(filepath.Clean(p)), true
	}
	return "", false
}

// pathText writes an absolute path for a note: from ~ when it is under the
// home folder, bare when nothing in it would end it early, and otherwise as a
// markdown link with the awkward characters escaped.
func pathText(p string) string {
	shown := p
	if home, err := os.UserHomeDir(); err == nil && (p == home || strings.HasPrefix(p, home+"/")) {
		shown = "~" + strings.TrimPrefix(p, home)
	}
	if !strings.ContainsAny(shown, " \t()[]<>`%") {
		return shown
	}
	esc := strings.NewReplacer("%", "%25", " ", "%20", "\t", "%09", "(", "%28", ")", "%29",
		"[", "%5B", "]", "%5D", "<", "%3C", ">", "%3E", "`", "%60").Replace(shown)
	label := strings.NewReplacer("[", "", "]", "").Replace(filepath.Base(p))
	return "[" + label + "](" + esc + ")"
}
