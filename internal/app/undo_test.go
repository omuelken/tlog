package app

import (
	"os"
	"testing"
)

func fileSays(t *testing.T, s *Service, rel string) string {
	t.Helper()
	f, err := s.Store.Read(rel)
	if err != nil {
		t.Fatal(err)
	}
	return string(f.Data)
}

func TestUndoAndRedoAStep(t *testing.T) {
	s := newSvc(t)
	rel, _ := s.AddToday("one")
	s.Checkpoint()
	if _, err := s.AddToday("two"); err != nil {
		t.Fatal(err)
	}
	s.Checkpoint()

	done, err := s.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if got := fileSays(t, s, rel); got != "- one\n" || done.Rel != rel {
		t.Fatalf("after undo: %q (%+v)", got, done)
	}
	if _, err := s.Redo(); err != nil {
		t.Fatal(err)
	}
	if got := fileSays(t, s, rel); got != "- one\n\n- two\n" {
		t.Fatalf("after redo: %q", got)
	}
}

func TestOneActionIsOneUndo(t *testing.T) {
	s := newSvc(t)
	rel, _ := s.AddToday("parent")
	s.Checkpoint()
	// Two writes with no checkpoint between them — the text, then a move —
	// are one thing a person did.
	s.AddToday("child")
	d, _ := s.Load(rel)
	if _, err := s.Indent(Addr{Rel: rel, Offset: d.Doc.Blocks[1].Start, Hash: d.Hash}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Undo(); err != nil {
		t.Fatal(err)
	}
	if got := fileSays(t, s, rel); got != "- parent\n" {
		t.Fatalf("got %q", got)
	}
}

func TestUndoNeverOverwritesSomeoneElsesEdit(t *testing.T) {
	s := newSvc(t)
	rel, _ := s.AddToday("mine")
	s.Checkpoint()
	if err := s.Store.Write(rel, []byte("- theirs\n"), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Undo(); err == nil {
		t.Fatal("undo wrote over an edit made elsewhere")
	}
	if got := fileSays(t, s, rel); got != "- theirs\n" {
		t.Fatalf("got %q", got)
	}
	if !s.CanUndo() {
		t.Fatal("a refused undo was thrown away instead of kept")
	}
}

func TestUndoingTheFirstWriteRemovesTheFile(t *testing.T) {
	s := newSvc(t)
	rel, _ := s.AddToday("a new day")
	s.Checkpoint()
	if _, err := s.Undo(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Store.Abs(rel)); !os.IsNotExist(err) {
		t.Fatalf("the day is still there: %v", err)
	}
}

func TestANewChangeEndsRedo(t *testing.T) {
	s := newSvc(t)
	s.AddToday("one")
	s.Checkpoint()
	s.Undo()
	s.AddToday("other")
	if s.CanRedo() {
		t.Fatal("redo survived a new change")
	}
	if _, err := s.Redo(); !Refused(err) {
		t.Fatalf("nothing to redo should be a refusal, got %v", err)
	}
	s2 := newSvc(t)
	if _, err := s2.Undo(); !Refused(err) {
		t.Fatalf("nothing to undo should be a refusal, got %v", err)
	}
}
