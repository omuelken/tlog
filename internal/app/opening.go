package app

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Opening is what following a link does with what it points at.
type Opening int

const (
	// OpenAddress: a web or mail address, for the browser or the mail
	// program.
	OpenAddress Opening = iota
	// OpenDocument: a file, handed to the application the system has for
	// its type — Word for a .docx, Preview for a .pdf.
	OpenDocument
	// OpenFolder: a place to be. The window opens it in the file manager;
	// the outliner opens a terminal there.
	OpenFolder
	// OpenProgram: something that would run if opened. A note is only text
	// and can name anything — an import, a paste, a synced file — so a link
	// never runs it; it is shown in the folder that holds it instead.
	OpenProgram
)

// LinkOpening resolves a link as ResolveLink does and says what following it
// does. Nothing is opened and nothing is written.
//
// A folder is not always a folder: on a Mac a .pages document or an .app is a
// directory the system calls a package, and the user calls a file. So the
// system is asked — mdls, for the type Launch Services gave it — rather than
// the extension guessed at. Where it cannot answer, a short list stands in.
// It runs a process, so it is for a click, never for drawing.
func (s *Service) LinkOpening(fromRel, target string) (string, Opening, error) {
	resolved, err := s.ResolveLink(fromRel, target)
	if err != nil {
		return "", 0, err
	}
	if u, err := url.Parse(resolved); err == nil && len(u.Scheme) > 1 {
		return resolved, OpenAddress, nil // ResolveLink returns paths for file:// links
	}
	st, err := os.Stat(resolved)
	if err != nil {
		return "", 0, err
	}
	types, known := contentTypes(resolved)
	if !known {
		types = guessTypes(resolved, st.IsDir(), st.Mode())
	}
	switch {
	case program(types):
		return resolved, OpenProgram, nil
	case st.IsDir() && !has(types, "com.apple.package"):
		return resolved, OpenFolder, nil
	default:
		return resolved, OpenDocument, nil
	}
}

// programTypes are the types opening would run rather than show: programs,
// and the files Terminal and Installer act on when opened. A .sh is not
// among them: opening one opens an editor.
var programTypes = []string{
	"public.executable", // binaries, .jar, .app
	"com.apple.application",
	"com.apple.terminal.shell-script", // .command, .tool: Terminal runs them
	"com.apple.terminal.settings",     // .terminal: can carry a command
	"com.apple.installer-package-archive",
	"com.apple.installer-meta-package",
	"com.apple.automator-workflow",
	"com.apple.applescript.script-bundle",
	"com.apple.applescript.text",
	"com.apple.applescript.script",
	"application/x-desktop", // a Linux .desktop launcher
	"application/vnd.appimage",
}

func program(types []string) bool {
	for _, t := range programTypes {
		if has(types, t) {
			return true
		}
	}
	return false
}

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// contentTypes asks the system what a path is: the type tree Launch Services
// gave it. False where there is no answer — not a Mac, or a volume Spotlight
// does not know. A variable so the tests can be any system.
var contentTypes = func(path string) ([]string, bool) {
	if runtime.GOOS != "darwin" {
		return nil, false
	}
	out, err := exec.Command("mdls", "-raw", "-name", "kMDItemContentTypeTree", path).Output()
	if err != nil {
		return nil, false
	}
	return parseTypeTree(string(out))
}

// parseTypeTree reads mdls's raw output: ( "a", "b" ), or (null).
func parseTypeTree(out string) ([]string, bool) {
	out = strings.TrimSpace(out)
	if !strings.HasPrefix(out, "(") || !strings.HasSuffix(out, ")") {
		return nil, false
	}
	var types []string
	for _, f := range strings.Split(out[1:len(out)-1], ",") {
		if t := strings.Trim(strings.TrimSpace(f), `"`); t != "" && t != "null" {
			types = append(types, t)
		}
	}
	return types, len(types) > 0
}

// guessTypes stands in for the system where it cannot be asked. It errs
// towards calling a thing a program: the cost of that is a folder shown
// rather than a file opened, and the cost of the other is a program run.
func guessTypes(path string, isDir bool, mode os.FileMode) []string {
	ext := strings.ToLower(filepath.Ext(path))
	if isDir {
		switch ext {
		case ".app":
			return []string{"com.apple.application", "com.apple.package"}
		case ".pages", ".numbers", ".key", ".rtfd", ".bundle":
			return []string{"com.apple.package"}
		}
		return nil
	}
	switch ext {
	case ".command", ".tool":
		return []string{"com.apple.terminal.shell-script"}
	case ".terminal":
		return []string{"com.apple.terminal.settings"}
	case ".pkg", ".mpkg":
		return []string{"com.apple.installer-package-archive"}
	case ".scpt", ".applescript", ".workflow":
		return []string{"com.apple.applescript.script"}
	case ".jar", ".exe", ".bat", ".cmd", ".com", ".msi":
		return []string{"public.executable"}
	case ".desktop":
		return []string{"application/x-desktop"}
	case ".appimage":
		return []string{"application/vnd.appimage"}
	}
	if mode&0o111 != 0 {
		return []string{"public.executable"}
	}
	return nil
}
