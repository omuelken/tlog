package app

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tilman-schieber/tlog/internal/attach"
)

// Attachments are shared with `att`, which is where they already live and
// which owns the workflow that matters — a drop folder and a watcher. tlog's
// job is the other half: putting a file on the shelf from inside a note, and
// finding one again when writing.

// AttachView is one stored file, ready to show and to insert.
type AttachView struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size string `json:"size"`
	Link string `json:"link"`
	When string `json:"when"`
}

func attachView(e attach.Entry) AttachView {
	return AttachView{
		Name: e.Name,
		Path: e.Path,
		Size: attach.Human(e.Size),
		Link: e.Link,
		When: e.ModTime.Format("2006-01-02"),
	}
}

// AttachDir is where the shared shelf is, shown so it is never a mystery.
func (s *Service) AttachDir() string { return s.shelf().Dir() }

func (s *Service) shelf() *attach.Store { return attach.Open(s.Cfg.AttachDir()) }

// Attachments lists the shelf, newest first, narrowed by a query the same way
// `att find` narrows it.
func (s *Service) Attachments(query string, limit int) ([]AttachView, error) {
	all, err := s.shelf().List()
	if err != nil {
		return nil, err
	}
	matched := attach.Match(all, query)
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	out := make([]AttachView, 0, len(matched))
	for _, e := range matched {
		out = append(out, attachView(e))
	}
	return out, nil
}

// Attach copies files onto the shelf and returns them. The originals are left
// where they are.
func (s *Service) Attach(paths ...string) ([]AttachView, error) {
	store := s.shelf()
	opt := attach.Options{
		Sanitize:  s.Cfg.Attachments.Sanitize,
		Lowercase: s.Cfg.Attachments.Lowercase,
	}
	var out []AttachView
	for _, p := range paths {
		e, err := store.AddWith(p, opt)
		if err != nil {
			return out, fmt.Errorf("%s: %w", p, err)
		}
		out = append(out, attachView(e))
	}
	return out, nil
}

// AttachTo copies files onto the shelf and writes their links into a file as
// one block, which is the whole point: the note and the file arrive together.
func (s *Service) AttachTo(rel string, paths ...string) ([]AttachView, error) {
	added, err := s.Attach(paths...)
	if err != nil {
		return added, err
	}
	if len(added) == 0 {
		return added, nil
	}
	links := make([]string, 0, len(added))
	for _, a := range added {
		links = append(links, a.Link)
	}
	if _, err := s.Add(rel, strings.Join(links, "\n")); err != nil {
		return added, err
	}
	return added, nil
}

// MaxImage is the largest picture taken from the clipboard. A screenshot of a
// large display is a few megabytes; this leaves room to spare while bounding
// what a stray paste can put on the shelf.
const MaxImage = 50 << 20

// imageTypes is what a pasted picture may be, keyed by what its bytes say.
var imageTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// AttachImage puts a picture that is not a file yet — a screenshot on the
// clipboard — on the shelf, named for the moment it arrived, and returns it
// like anything else attached. What it is is read from the bytes rather than
// taken on trust, so a paste of anything else is refused; two in one second
// get two names, because the shelf never overwrites.
func (s *Service) AttachImage(data []byte, now time.Time) (AttachView, error) {
	if len(data) == 0 {
		return AttachView{}, fmt.Errorf("the clipboard holds no picture")
	}
	if len(data) > MaxImage {
		return AttachView{}, fmt.Errorf("the picture is larger than %d MB", MaxImage>>20)
	}
	kind := http.DetectContentType(data)
	ext, ok := imageTypes[kind]
	if !ok {
		return AttachView{}, fmt.Errorf("not a picture tlog can show (%s)", kind)
	}

	// The shelf copies from a path, so the bytes become a file first, under
	// the name they should have; the copy on the shelf is the one that stays.
	tmp, err := os.MkdirTemp("", "tlog-paste-")
	if err != nil {
		return AttachView{}, err
	}
	defer os.RemoveAll(tmp)
	src := filepath.Join(tmp, "screenshot-"+now.Format("2006-01-02-150405")+ext)
	if err := os.WriteFile(src, data, 0o600); err != nil {
		return AttachView{}, err
	}
	added, err := s.Attach(src)
	if err != nil {
		return AttachView{}, err
	}
	return added[0], nil
}
