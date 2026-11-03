package app

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func attachSvc(t *testing.T) *Service {
	t.Helper()
	t.Setenv("ATT_DIR", t.TempDir())
	return newSvc(t)
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAPastedScreenshotLandsOnTheShelfAsAFile(t *testing.T) {
	s := attachSvc(t)
	at := time.Date(2026, 11, 3, 14, 5, 9, 0, time.Local)

	v, err := s.AttachImage(tinyPNG(t), at)
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "screenshot-2026-11-03-140509.png" {
		t.Fatalf("name: %q", v.Name)
	}
	if filepath.Dir(v.Path) != s.AttachDir() {
		t.Fatalf("not on the shelf: %s", v.Path)
	}
	if _, err := os.Stat(v.Path); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v.Link, "![screenshot-2026-11-03-140509.png](file://") {
		t.Fatalf("an image should be linked as an embed: %s", v.Link)
	}

	// The same second again: a second file, never the first one overwritten.
	w, err := s.AttachImage(tinyPNG(t), at)
	if err != nil {
		t.Fatal(err)
	}
	if w.Path == v.Path || w.Name != "screenshot-2026-11-03-140509-2.png" {
		t.Fatalf("second paste: %+v", w)
	}
}

func TestWhatIsNotAPictureIsRefused(t *testing.T) {
	s := attachSvc(t)
	for name, data := range map[string][]byte{
		"nothing": nil,
		"text":    []byte("hello, this is not a picture"),
		"html":    []byte("<html><body>nope</body></html>"),
	} {
		if _, err := s.AttachImage(data, time.Now()); err == nil {
			t.Errorf("%s was accepted as a picture", name)
		}
	}
	entries, _ := os.ReadDir(s.AttachDir())
	if len(entries) != 0 {
		t.Fatalf("a refused paste left something on the shelf: %v", entries)
	}
}

func TestAnOversizedPictureIsRefused(t *testing.T) {
	s := attachSvc(t)
	big := append(tinyPNG(t), make([]byte, MaxImage)...)
	if _, err := s.AttachImage(big, time.Now()); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("got %v", err)
	}
}
