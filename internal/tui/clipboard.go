package tui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// The clipboard. What is copied or cut goes onto the system clipboard as well
// as into the outliner's own register, so a block copied here pastes into any
// other program, and text copied anywhere pastes here.
//
// Locally the system's own tools do it: pbcopy and pbpaste on a Mac, wl-copy
// or xclip elsewhere. Over ssh those would reach the server's clipboard, which
// is nobody's, so the text is sent to the terminal instead as an OSC 52
// sequence, which kitty, iTerm2, WezTerm and most others put on the clipboard
// of the machine you are sitting at. Reading it back that way is rarely
// allowed, so over ssh p pastes what was last copied here.

// register is what was last copied or cut. lines says it was whole blocks
// rather than characters inside one, which decides how p puts it back.
type register struct {
	text  string
	lines bool
}

// clipWrite and clipRead are the system clipboard. Variables so that the
// tests use a clipboard of their own instead of the person's.
var (
	clipWrite = systemClipWrite
	clipRead  = systemClipRead
)

// copyText puts text in the register and on the system clipboard. Failing to
// reach the system clipboard is said, but the register still has it.
func (m *Model) copyText(text string, lines bool) {
	m.reg = register{text: text, lines: lines}
	if err := clipWrite(text); err != nil {
		m.status = "copied here only — the system clipboard said: " + err.Error()
	}
}

// pasteText is what p pastes: the system clipboard when it can be read, which
// is also what was copied here last unless something else was copied since,
// and otherwise the register.
func (m *Model) pasteText() (register, bool) {
	if text, ok := clipRead(); ok && text != "" {
		if text == m.reg.text {
			return m.reg, true
		}
		return register{text: text, lines: looksLikeOutline(text)}, true
	}
	return m.reg, m.reg.text != ""
}

// looksLikeOutline says whether text copied elsewhere is blocks: its first
// line that says anything is a bullet.
func looksLikeOutline(text string) bool {
	for _, l := range strings.Split(text, "\n") {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		return t == "-" || strings.HasPrefix(t, "- ")
	}
	return false
}

func systemClipWrite(text string) error {
	if overSSH() {
		return osc52(text)
	}
	if argv := clipTool(true); argv != nil {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %v %s", argv[0], err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	return osc52(text)
}

func systemClipRead() (string, bool) {
	if overSSH() {
		return "", false
	}
	argv := clipTool(false)
	if argv == nil {
		return "", false
	}
	var out bytes.Buffer
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		return "", false
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return "", false
		}
	case <-time.After(time.Second):
		_ = cmd.Process.Kill()
		return "", false
	}
	return out.String(), true
}

// clipTool is the command that writes (or reads) the clipboard here, or nil.
func clipTool(write bool) []string {
	type tool struct{ write, read []string }
	var tools []tool
	switch {
	case runtime.GOOS == "darwin":
		tools = []tool{{[]string{"pbcopy"}, []string{"pbpaste"}}}
	case os.Getenv("WAYLAND_DISPLAY") != "":
		tools = []tool{{[]string{"wl-copy"}, []string{"wl-paste", "--no-newline"}}}
	}
	if os.Getenv("DISPLAY") != "" {
		tools = append(tools,
			tool{[]string{"xclip", "-selection", "clipboard"}, []string{"xclip", "-selection", "clipboard", "-o"}},
			tool{[]string{"xsel", "--clipboard", "--input"}, []string{"xsel", "--clipboard", "--output"}})
	}
	for _, t := range tools {
		argv := t.read
		if write {
			argv = t.write
		}
		if _, err := exec.LookPath(argv[0]); err == nil {
			return argv
		}
	}
	return nil
}

// osc52 asks the terminal to put text on its clipboard. Inside tmux the
// sequence is wrapped so tmux hands it through to the terminal around it.
func osc52(text string) error {
	seq := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\x07"
	if os.Getenv("TMUX") != "" {
		seq = "\x1bPtmux;" + strings.ReplaceAll(seq, "\x1b", "\x1b\x1b") + "\x1b\\"
	}
	_, err := os.Stdout.WriteString(seq)
	return err
}
