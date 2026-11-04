package tui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// stubTerminal records where a terminal would have opened.
func stubTerminal(t *testing.T) *[]string {
	t.Helper()
	var opened []string
	old := openTerminal
	openTerminal = func(dir, _ string) error { opened = append(opened, dir); return nil }
	t.Cleanup(func() { openTerminal = old })
	return &opened
}

// linkTo is a page with one link, the cursor off it so it is drawn rather
// than edited, and an assets folder beside the notes holding what it points at.
func linkTo(t *testing.T, target string, files map[string]os.FileMode, dirs ...string) (*Model, string) {
	t.Helper()
	m := treeModel(t, "- see ["+filepath.Base(target)+"]("+target+")\n\n- below\n")
	assets := filepath.Join(m.svc.Store.Root, "assets")
	for _, d := range append(dirs, ".") {
		if err := os.MkdirAll(filepath.Join(assets, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, mode := range files {
		// Not text: a plain-text file is shown inside tlog, and these tests
		// are about what goes to the desktop.
		if err := os.WriteFile(filepath.Join(assets, name), []byte("\x00bin"), mode); err != nil {
			t.Fatal(err)
		}
	}
	send(t, m, k(tea.KeyDown))
	m.View()
	return m, assets
}

func TestClickingADocumentLinkOpensTheDocument(t *testing.T) {
	terms := stubTerminal(t)
	m, assets := linkTo(t, "../assets/q3.pdf", map[string]os.FileMode{"q3.pdf": 0o644})
	got := opened(m)
	m.Update(clickAt(8, 1))
	if len(*got) != 1 || (*got)[0] != filepath.Join(assets, "q3.pdf") {
		t.Fatalf("opened %v", *got)
	}
	if len(*terms) != 0 {
		t.Fatalf("a document opens in its application, not a terminal: %v", *terms)
	}
}

func TestClickingAFolderLinkOpensATerminalThere(t *testing.T) {
	terms := stubTerminal(t)
	m, assets := linkTo(t, "../assets/src", nil, "src")
	got := opened(m)
	m.Update(clickAt(8, 1))
	if len(*terms) != 1 || (*terms)[0] != filepath.Join(assets, "src") {
		t.Fatalf("want a terminal in %s, got %v", filepath.Join(assets, "src"), *terms)
	}
	if len(*got) != 0 {
		t.Fatalf("a folder was handed to the desktop: %v", *got)
	}
}

func TestAProgramIsNeverRunFromALink(t *testing.T) {
	terms := stubTerminal(t)
	m, assets := linkTo(t, "../assets/tool", map[string]os.FileMode{"tool": 0o755})
	got := opened(m)
	revealed := 0
	m.svc.Reveal = func(string) error { revealed++; return nil }
	m.Update(clickAt(8, 1))
	if len(*got) != 0 || revealed != 0 {
		t.Fatalf("ran %v", *got)
	}
	if len(*terms) != 1 || (*terms)[0] != assets {
		t.Fatalf("want a terminal in %s, got %v", assets, *terms)
	}
	if !strings.Contains(m.status, "program") {
		t.Fatalf("should say why: %q", m.status)
	}
}

func TestOverSSHADocumentIsNotOpened(t *testing.T) {
	stubTerminal(t)
	m, _ := linkTo(t, "../assets/q3.pdf", map[string]os.FileMode{"q3.pdf": 0o644})
	got := opened(m)
	overSSH = func() bool { return true }
	t.Cleanup(func() { overSSH = func() bool { return false } })
	m.Update(clickAt(8, 1))
	if len(*got) != 0 || !strings.Contains(m.errMsg, "ssh") {
		t.Fatalf("opened %v, said %q", *got, m.errMsg)
	}
}

func fakeEnv(goos string, vars map[string]string, onPath ...string) termEnv {
	return termEnv{
		getenv: func(k string) string { return vars[k] },
		lookPath: func(name string) (string, error) {
			for _, p := range onPath {
				if p == name {
					return "/usr/bin/" + name, nil
				}
			}
			return "", errors.New("not found")
		},
		exists: func(p string) bool { return strings.HasPrefix(p, "/Applications/kitty.app") },
		goos:   goos,
	}
}

func firstArgv(t *testing.T, dir, command string, e termEnv) []string {
	t.Helper()
	plan, err := terminalPlan(dir, command, e)
	if err != nil {
		t.Fatal(err)
	}
	return plan[0].argv
}

func TestTheTerminalTlogRunsInIsAskedForATab(t *testing.T) {
	const dir = "/n/my dir"
	cases := []struct {
		name string
		e    termEnv
		want []string
	}{
		{"tmux beats the terminal around it",
			fakeEnv("darwin", map[string]string{"TMUX": "x", "KITTY_WINDOW_ID": "1"}),
			[]string{"tmux", "new-window", "-c", dir}},
		{"zellij",
			fakeEnv("linux", map[string]string{"ZELLIJ": "0"}),
			[]string{"zellij", "action", "new-tab", "--cwd", dir}},
		{"kitty with remote control, from its app bundle",
			fakeEnv("darwin", map[string]string{"KITTY_WINDOW_ID": "1", "KITTY_LISTEN_ON": "unix:/tmp/k"}),
			[]string{"/Applications/kitty.app/Contents/MacOS/kitty", "@", "--to", "unix:/tmp/k", "launch", "--type=tab", "--cwd", dir}},
		{"kitty without it opens a window, not the saved session",
			fakeEnv("linux", map[string]string{"TERM": "xterm-kitty"}, "kitty"),
			[]string{"/usr/bin/kitty", "--detach", "--session=none", "--directory", dir}},
		{"wezterm",
			fakeEnv("linux", map[string]string{"WEZTERM_PANE": "3"}, "wezterm"),
			[]string{"/usr/bin/wezterm", "cli", "spawn", "--cwd", dir}},
		{"iterm gets the folder as an argument, not as script",
			fakeEnv("darwin", map[string]string{"TERM_PROGRAM": "iTerm.app"}),
			[]string{"osascript", "-e", itermTab, dir}},
		{"ghostty on a mac",
			fakeEnv("darwin", map[string]string{"TERM_PROGRAM": "ghostty"}),
			[]string{"open", "-a", "Ghostty", dir}},
		{"terminal.app",
			fakeEnv("darwin", map[string]string{"TERM_PROGRAM": "Apple_Terminal"}),
			[]string{"open", "-a", "Terminal", dir}},
		{"anything else on linux: $TERMINAL, started in the folder",
			fakeEnv("linux", map[string]string{"TERMINAL": "foot"}),
			[]string{"foot"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := firstArgv(t, dir, "", c.e); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestAKittyTabFallsBackToAWindow(t *testing.T) {
	plan, err := terminalPlan("/d", "", fakeEnv("darwin", map[string]string{"KITTY_WINDOW_ID": "1", "KITTY_LISTEN_ON": "unix:/tmp/k"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) < 2 || !reflect.DeepEqual(plan[1].argv, []string{"open", "-a", "kitty", "/d"}) {
		t.Fatalf("remote control can be off; a window must follow the tab: %+v", plan)
	}
}

// A second kitty process is a second app: its own Dock icon, its own place in
// cmd-tab. On a Mac the folder goes to the kitty already running.
func TestOnAMacKittyIsNeverStartedTwice(t *testing.T) {
	plan, err := terminalPlan("/d", "", fakeEnv("darwin", map[string]string{"KITTY_WINDOW_ID": "1"}, "kitty"))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range plan {
		for _, a := range l.argv {
			if a == "--detach" || a == "-na" || a == "-n" {
				t.Fatalf("would start a second kitty: %q", l.argv)
			}
		}
	}
}

func TestAConfiguredCommandIsTheOnlyWay(t *testing.T) {
	e := fakeEnv("darwin", map[string]string{"TMUX": "x"})
	if got := firstArgv(t, "/n/my dir", `alacritty --working-directory {dir}`, e); !reflect.DeepEqual(got,
		[]string{"alacritty", "--working-directory", "/n/my dir"}) {
		t.Fatalf("{dir} stays one argument, spaces and all: %q", got)
	}
	if got := firstArgv(t, "/d", `open -a "My Term"`, e); !reflect.DeepEqual(got,
		[]string{"open", "-a", "My Term", "/d"}) {
		t.Fatalf("quotes group, and without {dir} the folder goes last: %q", got)
	}
	plan, _ := terminalPlan("/d", "foo", e)
	if len(plan) != 1 {
		t.Fatalf("a configured command is not second-guessed: %+v", plan)
	}
}

func TestOverSSHThereIsNothingHereToOpen(t *testing.T) {
	if _, err := terminalPlan("/d", "", fakeEnv("linux", map[string]string{"SSH_CONNECTION": "1 2 3 4"})); err == nil {
		t.Fatal("a terminal opened on the server would be no use to anyone")
	}
	if got := firstArgv(t, "/d", "", fakeEnv("linux", map[string]string{"SSH_CONNECTION": "x", "TMUX": "y"})); got[0] != "tmux" {
		t.Fatalf("but a tmux window over ssh is exactly right: %q", got)
	}
}

func TestTheSettingReachesTheLauncher(t *testing.T) {
	var gotCmd string
	old := openTerminal
	openTerminal = func(_, command string) error { gotCmd = command; return nil }
	t.Cleanup(func() { openTerminal = old })
	m, _ := linkTo(t, "../assets/src", nil, "src")
	m.svc.Cfg.Outliner.Terminal = "alacritty --working-directory {dir}"
	m.Update(clickAt(8, 1))
	if gotCmd != "alacritty --working-directory {dir}" {
		t.Fatalf("the launcher got %q", gotCmd)
	}
}

func TestAScriptIsShownNotRun(t *testing.T) {
	terms := stubTerminal(t)
	m := treeModel(t, "- see [go](../assets/go.command)\n\n- below\n")
	assets := filepath.Join(m.svc.Store.Root, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "go.command"), []byte("rm -rf ~\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := opened(m)
	send(t, m, k(tea.KeyDown))
	m.View()
	m.Update(clickAt(6, 1)) // "• see " is six columns, then the label
	if m.mode != modeFile || len(*got) != 0 || len(*terms) != 0 {
		t.Fatalf("a script is text: it is shown, never run (mode %v, opened %v, terminals %v)", m.mode, *got, *terms)
	}
}

func TestAPathThatLeadsSomewhereIsALink(t *testing.T) {
	terms := stubTerminal(t)
	m := treeModel(t, "- see ../assets/src or and/or\n\n- below\n")
	if err := os.MkdirAll(filepath.Join(m.svc.Store.Root, "assets", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	opened(m)
	send(t, m, k(tea.KeyDown))
	m.View()
	if len(m.lineHits) == 0 || len(m.lineHits[0]) != 1 {
		t.Fatalf("want one link on the line, got %+v", m.lineHits)
	}
	m.Update(clickAt(8, 1)) // "• see " is six columns, then the path
	want := filepath.Join(m.svc.Store.Root, "assets", "src")
	if len(*terms) != 1 || (*terms)[0] != want {
		t.Fatalf("want a terminal in %s, got %v", want, *terms)
	}
}

func TestAPastedFolderIsWrittenAsAPath(t *testing.T) {
	m := treeModel(t, "- ")
	dir := filepath.Join(m.svc.Store.Root, "assets", "My Folder")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(dir), Paste: true})
	if m.ed == nil || !strings.HasPrefix(m.ed.String(), "[My Folder](") || !strings.Contains(m.ed.String(), "My%20Folder)") {
		t.Fatalf("got %q", m.ed.String())
	}
}
