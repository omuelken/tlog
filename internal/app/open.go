package app

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// OpenLink hands what a link points at to the desktop: a web address to the
// browser, a file — an attachment, an image — to whatever opens that kind of
// file. fromRel is the note the link is written in, which is what a relative
// path is relative to.
//
// A note is just a file and a file could name any scheme it likes, so only the
// schemes a link in prose is actually meant to be are opened.
//
// A program named by a link is never run: it is shown selected in the folder
// that holds it instead. See LinkOpening.
func (s *Service) OpenLink(fromRel, target string) error {
	resolved, how, err := s.LinkOpening(fromRel, target)
	if err != nil {
		return err
	}
	if how == OpenProgram {
		if s.Reveal != nil {
			return s.Reveal(resolved)
		}
		return systemReveal(resolved)
	}
	if s.Launch != nil {
		return s.Launch(resolved)
	}
	return systemOpen(resolved)
}

// ResolveLink turns a link target into something the desktop can open: an
// address with an allowed scheme, or the absolute path of a file that exists.
func (s *Service) ResolveLink(fromRel, target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("the link is empty")
	}
	u, err := url.Parse(target)
	if err == nil && u.Scheme != "" && len(u.Scheme) > 1 { // C:\ is a path, not a scheme
		switch strings.ToLower(u.Scheme) {
		case "http", "https", "mailto":
			return target, nil
		case "file":
			return existing(u.Path)
		default:
			return "", fmt.Errorf("refusing to open a %q link", u.Scheme)
		}
	}
	// No scheme: a path. Relative ones are relative to the note, the way
	// markdown means them and the way Logseq's ../assets links are written.
	p := target
	if unescaped, err := url.PathUnescape(p); err == nil {
		p = unescaped
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[2:])
		}
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(s.Store.Root, filepath.Dir(fromRel), p)
	}
	return existing(p)
}

func existing(p string) (string, error) {
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("nothing at %s", p)
	}
	return p, nil
}

// systemOpen starts the platform's opener and does not wait for it: the
// viewer belongs to the desktop now, not to tlog. Its output is discarded,
// because in the outliner it would be written across the screen.
func systemOpen(target string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	cmd := exec.Command(name, target)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not open %s: %w", target, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// systemReveal shows a path selected in the file manager: Finder on a Mac.
// There is no portable "select this file", so elsewhere the folder holding it
// is opened, which is the honest next best thing.
func systemReveal(path string) error {
	cmd := exec.Command("open", "-R", path)
	if runtime.GOOS != "darwin" {
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not show %s: %w", path, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// CommandWords splits a configured command line into its arguments, quotes
// and backslashes understood, so that it can be run without a shell.
func CommandWords(text string) []string { return shellWords(text) }

// DroppedFiles recognises what a terminal types when files are dragged onto
// it: their paths, separated by spaces or newlines, shell-quoted or escaped, or
// as file:// addresses, depending on the terminal. It returns the paths only
// when every one of them is an existing file, so pasting ordinary text that
// happens to contain a path is never mistaken for a drop.
func DroppedFiles(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	// Some terminals write one plain path per line, spaces and all, which a
	// shell would split; those are tried whole when the shell reading fails.
	for _, parts := range [][]string{shellWords(text), strings.Split(text, "\n")} {
		if files := existingFiles(parts); files != nil {
			return files
		}
	}
	return nil
}

// existingFiles is the parts as paths, or nil unless every one of them is an
// absolute path to a regular file.
func existingFiles(parts []string) []string {
	var out []string
	for _, tok := range parts {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if strings.HasPrefix(tok, "file://") {
			u, err := url.Parse(tok)
			if err != nil {
				return nil
			}
			tok = u.Path
		}
		if !filepath.IsAbs(tok) {
			return nil
		}
		info, err := os.Stat(tok)
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		out = append(out, tok)
	}
	return out
}

// shellWords splits text the way a shell would read it: whitespace separates,
// quotes group, a backslash escapes the next character.
func shellWords(s string) []string {
	var out []string
	var cur strings.Builder
	in := false // inside a word, even an empty quoted one
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped, in = true, true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, in = r, true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if in {
				out = append(out, cur.String())
				cur.Reset()
				in = false
			}
		default:
			cur.WriteRune(r)
			in = true
		}
	}
	if in {
		out = append(out, cur.String())
	}
	return out
}
