package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// The handler is the one hole in the window's origin, so what it refuses
// matters more than what it serves.
func TestOnlyTheShelfIsServed(t *testing.T) {
	base := t.TempDir()
	shelf := filepath.Join(base, "store")
	if err := os.MkdirAll(shelf, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, s string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(shelf, "plan.png"), "PNG")
	write(filepath.Join(shelf, "my notes.pdf"), "PDF")
	write(filepath.Join(base, "secret.txt"), "no")
	if err := os.Symlink(filepath.Join(base, "secret.txt"), filepath.Join(shelf, "escape.txt")); err != nil {
		t.Fatal(err)
	}

	h := shelfFiles(func() string { return shelf })
	get := func(method, path string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec
	}

	if rec := get("GET", filePrefix+shelf[1:]+"/plan.png"); rec.Code != 200 || rec.Body.String() != "PNG" {
		t.Fatalf("a file on the shelf: %d %q", rec.Code, rec.Body.String())
	}
	if rec := get("GET", filePrefix+shelf[1:]+"/my%20notes.pdf"); rec.Code != 200 || rec.Body.String() != "PDF" {
		t.Fatalf("a name with a space, percent-encoded as att links it: %d", rec.Code)
	}
	if csp := get("GET", filePrefix+shelf[1:]+"/plan.png").Header().Get("Content-Security-Policy"); csp == "" {
		t.Fatal("served without a policy: an SVG on the shelf could run")
	}

	for name, path := range map[string]string{
		"a sibling of the shelf":      filePrefix + base[1:] + "/secret.txt",
		"a dot-dot out of it":         filePrefix + shelf[1:] + "/../secret.txt",
		"a symlink out of it":         filePrefix + shelf[1:] + "/escape.txt",
		"the shelf directory itself":  filePrefix + shelf[1:],
		"something that is not there": filePrefix + shelf[1:] + "/gone.png",
		"a path outside the prefix":   shelf + "/plan.png",
	} {
		if rec := get("GET", path); rec.Code == 200 {
			t.Errorf("%s was served (%s)", name, path)
		}
	}
	if rec := get("POST", filePrefix+shelf[1:]+"/plan.png"); rec.Code == 200 {
		t.Error("a POST was answered")
	}
}
