package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/tilman-schieber/tlog/internal/app"
)

// Opening a terminal at a file link. The outliner runs inside some terminal,
// and the useful thing is a new tab of *that* terminal, so tlog asks it rather
// than assuming one. Every terminal spells "new tab here" differently, so this
// is a table of the known ones, a setting for any other, and a fallback that
// works for most: start $TERMINAL with the folder as its working directory.

// launch is one way of opening a terminal.
type launch struct {
	argv []string
	// quick is true for a command that hands the request to a running program
	// and exits, so its exit status says whether it worked and the next way
	// can be tried. A terminal started directly does not exit; it is started
	// and left.
	quick bool
}

// termEnv is everything the choice depends on, so that it can be tested
// without any of these terminals installed.
type termEnv struct {
	getenv   func(string) string
	lookPath func(string) (string, error)
	exists   func(string) bool
	goos     string
}

func realTermEnv() termEnv {
	return termEnv{
		getenv:   os.Getenv,
		lookPath: exec.LookPath,
		exists:   func(p string) bool { _, err := os.Stat(p); return err == nil },
		goos:     runtime.GOOS,
	}
}

// iTerm2 has no command line for a new tab, only AppleScript. The folder is
// passed as an argument rather than spliced into the script, so no path can
// become script.
const itermTab = `on run argv
	tell application "iTerm2"
		tell current window
			create tab with default profile
			tell current session to write text "cd " & quoted form of (item 1 of argv)
		end tell
	end tell
end run`

// terminalPlan lists the ways to open a terminal in dir, best first. A
// configured command is the only way when there is one.
func terminalPlan(dir, command string, e termEnv) ([]launch, error) {
	if strings.TrimSpace(command) != "" {
		words := app.CommandWords(command)
		if len(words) == 0 {
			return nil, fmt.Errorf("outliner.terminal is not a command: %q", command)
		}
		found := false
		for i, w := range words {
			if strings.Contains(w, "{dir}") {
				words[i] = strings.ReplaceAll(w, "{dir}", dir)
				found = true
			}
		}
		if !found {
			words = append(words, dir)
		}
		return []launch{{argv: words}}, nil
	}

	var plan []launch
	add := func(quick bool, argv ...string) { plan = append(plan, launch{argv: argv, quick: quick}) }

	// A multiplexer first: inside one, a tab of it is a tab where you are,
	// whatever the terminal around it — and the only kind that works over ssh.
	if e.getenv("TMUX") != "" {
		add(true, "tmux", "new-window", "-c", dir)
	}
	if e.getenv("ZELLIJ") != "" {
		add(true, "zellij", "action", "new-tab", "--cwd", dir)
	}
	if len(plan) == 0 && e.getenv("SSH_CONNECTION") != "" {
		return nil, fmt.Errorf("over ssh there is no terminal here to open — the folder is %s", dir)
	}

	term := e.getenv("TERM_PROGRAM")
	switch {
	case e.getenv("KITTY_WINDOW_ID") != "" || e.getenv("TERM") == "xterm-kitty":
		kitty := e.binary("kitty", "/Applications/kitty.app/Contents/MacOS/kitty")
		// A tab needs remote control, over a socket: talking over the
		// terminal itself would fight the outliner for its input. Without
		// it, a new window is the next best thing.
		if sock := e.getenv("KITTY_LISTEN_ON"); sock != "" && kitty != "" {
			add(true, kitty, "@", "--to", sock, "launch", "--type=tab", "--cwd", dir)
		}
		if e.goos == "darwin" {
			// Handed to the kitty already running, which opens the folder
			// per its launch-actions.conf — a window by default, a tab if
			// that file says so. Starting kitty itself would be a second
			// process, with a second icon in the Dock and in cmd-tab.
			add(true, "open", "-a", "kitty", dir)
		} else if kitty != "" {
			// --session=none, or a fresh instance loads startup_session from
			// kitty.conf and one click reopens a whole saved workspace.
			add(true, kitty, "--detach", "--session=none", "--directory", dir)
		}
	case e.getenv("WEZTERM_PANE") != "":
		if wez := e.binary("wezterm", "/Applications/WezTerm.app/Contents/MacOS/wezterm"); wez != "" {
			add(true, wez, "cli", "spawn", "--cwd", dir)
		}
	case term == "iTerm.app":
		add(true, "osascript", "-e", itermTab, dir)
	case term == "ghostty":
		// Ghostty has no way to ask a running instance for a tab yet. On a
		// Mac the folder is handed to the running one, as with kitty, rather
		// than starting a second app with its own Dock icon.
		if e.goos == "darwin" {
			add(true, "open", "-a", "Ghostty", dir)
		} else {
			add(false, "ghostty", "--working-directory="+dir)
		}
	case term == "Apple_Terminal":
		add(true, "open", "-a", "Terminal", dir)
	}

	// Whatever terminal the user named, started in the folder: nearly every
	// one opens its shell in its own working directory.
	if t := e.getenv("TERMINAL"); t != "" {
		if words := app.CommandWords(t); len(words) > 0 {
			add(false, words...)
		}
	}
	if e.goos == "darwin" {
		add(true, "open", "-a", "Terminal", dir)
	} else if _, err := e.lookPath("x-terminal-emulator"); err == nil {
		add(false, "x-terminal-emulator")
	}
	if len(plan) == 0 {
		return nil, fmt.Errorf("no terminal found to open — set outliner.terminal, with {dir} for the folder")
	}
	return plan, nil
}

// binary finds a program on PATH, or at the place its app bundle keeps it.
func (e termEnv) binary(name, bundled string) string {
	if p, err := e.lookPath(name); err == nil {
		return p
	}
	if e.goos == "darwin" && e.exists(bundled) {
		return bundled
	}
	return ""
}

// openTerminal opens a terminal in dir, by the configured command or by
// asking the terminal tlog runs in. A variable so the tests can see what
// would have been opened without opening it.
var openTerminal = func(dir, command string) error {
	plan, err := terminalPlan(dir, command, realTermEnv())
	if err != nil {
		return err
	}
	var last error
	for _, l := range plan {
		if last = run(l, dir); last == nil {
			return nil
		}
	}
	return last
}

func run(l launch, dir string) error {
	if !l.quick {
		cmd := exec.Command(l.argv[0], l.argv[1:]...)
		cmd.Dir = dir
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("could not start %s: %w", l.argv[0], err)
		}
		go cmd.Wait() // reap it whenever it closes; it is not ours to watch
		return nil
	}
	// A request to a running program comes back at once, or it never will.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, l.argv[0], l.argv[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%s: %s", l.argv[0], msg)
		}
		return fmt.Errorf("%s: %w", l.argv[0], err)
	}
	return nil
}

// overSSH reports whether the outliner runs on another machine than the
// screen, where nothing it opens would be seen. A variable for the tests.
var overSSH = func() bool { return os.Getenv("SSH_CONNECTION") != "" }
