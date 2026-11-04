package tui

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeClip is the system clipboard as far as the tests are concerned.
var fakeClip string

// Every navigation in these tests would otherwise be remembered in the real
// ~/.local/state/tlog/recent.json.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tlog-state-")
	if err != nil {
		panic(err)
	}
	os.Setenv("TLOG_STATE", filepath.Join(dir, "recent.json"))
	// Nor may copying in a test reach the person's clipboard.
	clipWrite = func(text string) error { fakeClip = text; return nil }
	clipRead = func() (string, bool) { return fakeClip, fakeClip != "" }
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
