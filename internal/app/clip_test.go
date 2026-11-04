package app

import (
	"os"
	"strings"
	"testing"
)

// page writes a file and returns its relative path, hash and the offset of
// each block by its first line.
func page(t *testing.T, s *Service, body string) (string, string, map[string]int) {
	t.Helper()
	rel := "pages/Clip.md"
	if err := os.WriteFile(s.Store.Abs(rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := s.Load(rel)
	if err != nil {
		t.Fatal(err)
	}
	at := map[string]int{}
	for _, b := range d.Doc.Flatten() {
		at[b.FirstLine()] = b.Start
	}
	return rel, d.Hash, at
}

// onDiskNow is the file as it stands, without the blank lines a setting may
// put between top-level blocks: the shape is what these tests are about.
func onDiskNow(t *testing.T, s *Service, rel string) string {
	t.Helper()
	b, err := os.ReadFile(s.Store.Abs(rel))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\n\n", "\n")
}

func TestAFragmentIsTheOutlineAsStored(t *testing.T) {
	s := newSvc(t)
	rel, hash, at := page(t, s, "- a\n  - a1\n    - a11\n  - a2\n- b\n")

	got, err := s.Fragment(rel, hash, []int{at["a1"]})
	if err != nil {
		t.Fatal(err)
	}
	if got != "- a1\n  - a11\n" {
		t.Fatalf("a block brings what is under it, at the top level: %q", got)
	}

	// a1 is inside a, so it is not taken twice; b keeps its place after a.
	got, _ = s.Fragment(rel, hash, []int{at["a"], at["a1"], at["b"]})
	if got != "- a\n  - a1\n    - a11\n  - a2\n- b\n" {
		t.Fatalf("got %q", got)
	}

	// Different depths keep their distance from the shallowest.
	got, _ = s.Fragment(rel, hash, []int{at["a2"], at["a11"]})
	if got != "  - a11\n- a2\n" {
		t.Fatalf("got %q", got)
	}
	if onDiskNow(t, s, rel) != "- a\n  - a1\n    - a11\n  - a2\n- b\n" {
		t.Fatal("copying wrote to the file")
	}
}

func TestCuttingTakesTheBlocksAndSaysWhereToLand(t *testing.T) {
	s := newSvc(t)
	rel, hash, at := page(t, s, "- a\n  - a1\n- b\n- c\n")

	res, text, err := s.CutBlocks(rel, hash, []int{at["a"], at["b"]})
	if err != nil {
		t.Fatal(err)
	}
	if text != "- a\n  - a1\n- b\n" {
		t.Fatalf("cut %q", text)
	}
	if got := onDiskNow(t, s, rel); got != "- c\n" {
		t.Fatalf("left %q", got)
	}
	if d, _ := s.Load(rel); d.Doc.FindByOffset(res.Offset).Text != "c" || res.Hash != d.Hash {
		t.Fatalf("the cursor should land on c: %+v", res)
	}

	if _, _, err := s.CutBlocks(rel, hash, []int{0}); !Stale(err) && err == nil {
		t.Fatal("a cut from a stale read went through")
	}
}

func TestPastingPutsTheOutlineBackAsBlocks(t *testing.T) {
	s := newSvc(t)
	rel, hash, at := page(t, s, "- a\n  - a1\n- b\n")

	res, err := s.PasteBlocks(Addr{Rel: rel, Offset: at["a"], Hash: hash}, "- x\n  - x1\n- y\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := onDiskNow(t, s, rel); got != "- a\n  - a1\n- x\n  - x1\n- y\n- b\n" {
		t.Fatalf("after: %q", got)
	}
	if d, _ := s.Load(rel); d.Doc.FindByOffset(res.Offset).Text != "x" {
		t.Fatal("the result should be the first block pasted")
	}

	// Before, and at the level of the block it is pasted next to.
	d, _ := s.Load(rel)
	a1 := d.Doc.Blocks[0].Children[0]
	if _, err := s.PasteBlocks(Addr{Rel: rel, Offset: a1.Start, Hash: d.Hash}, "- z\n", true); err != nil {
		t.Fatal(err)
	}
	if got := onDiskNow(t, s, rel); got != "- a\n  - z\n  - a1\n- x\n  - x1\n- y\n- b\n" {
		t.Fatalf("before: %q", got)
	}
}

func TestPastingPlainTextMakesOneBlock(t *testing.T) {
	s := newSvc(t)
	rel, hash, at := page(t, s, "- a\n")
	if _, err := s.PasteBlocks(Addr{Rel: rel, Offset: at["a"], Hash: hash}, "just words\nand a second line", false); err != nil {
		t.Fatal(err)
	}
	if got := onDiskNow(t, s, rel); got != "- a\n- just words\n  and a second line\n" {
		t.Fatalf("got %q", got)
	}
}

func TestPastingIntoABlankBulletReplacesIt(t *testing.T) {
	s := newSvc(t)
	rel, hash, at := page(t, s, "- \n")
	if _, err := s.PasteBlocks(Addr{Rel: rel, Offset: at[""], Hash: hash}, "- x\n", false); err != nil {
		t.Fatal(err)
	}
	if got := onDiskNow(t, s, rel); got != "- x\n" {
		t.Fatalf("got %q", got)
	}
}

func TestACopyDoesNotTakeAnAnchorThePageAlreadyHas(t *testing.T) {
	s := newSvc(t)
	rel, hash, at := page(t, s, "- named ^k3f9q2\n")
	text, _ := s.Fragment(rel, hash, []int{at["named"]})
	if _, err := s.PasteBlocks(Addr{Rel: rel, Offset: at["named"], Hash: hash}, text, false); err != nil {
		t.Fatal(err)
	}
	if got := onDiskNow(t, s, rel); got != "- named ^k3f9q2\n- named\n" {
		t.Fatalf("got %q", got)
	}

	// Cut and pasted back, the block is still the one links point at.
	rel, hash, at = page(t, s, "- named ^k3f9q2\n- other\n")
	res, text, err := s.CutBlocks(rel, hash, []int{at["named"]})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PasteBlocks(res.Addr, text, false); err != nil {
		t.Fatal(err)
	}
	if got := onDiskNow(t, s, rel); got != "- other\n- named ^k3f9q2\n" {
		t.Fatalf("got %q", got)
	}
}

func TestNothingToPasteIsRefusedQuietly(t *testing.T) {
	s := newSvc(t)
	rel, hash, at := page(t, s, "- a\n")
	if _, err := s.PasteBlocks(Addr{Rel: rel, Offset: at["a"], Hash: hash}, "  \n", false); !Refused(err) {
		t.Fatalf("got %v", err)
	}
}
