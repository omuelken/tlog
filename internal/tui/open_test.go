package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tilman-schieber/tlog/internal/markdown"
)

// outline is every block on disk, depth shown as leading dots, in file order.
func outline(t *testing.T, m *Model) []string {
	t.Helper()
	var out []string
	var walk func(bs []*markdown.Block, depth int)
	walk = func(bs []*markdown.Block, depth int) {
		for _, b := range bs {
			out = append(out, strings.Repeat(".", depth)+b.Text)
			walk(b.Children, depth+1)
		}
	}
	walk(markdown.Parse([]byte(onDisk(t, m))).Blocks, 0)
	return out
}

func TestOOpensABlockBelowAndWritesInIt(t *testing.T) {
	m := treeModel(t, "- a\n\n- b\n")
	send(t, m, k(tea.KeyEsc), runes("o"))
	if m.mode != modeOutline || m.ed == nil {
		t.Fatalf("o should start writing: mode %v", m.mode)
	}
	typeText(t, m, "new")
	send(t, m, k(tea.KeyEsc))
	if got, want := outline(t, m), []string{"a", "new", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestShiftOOpensABlockAbove(t *testing.T) {
	m := treeModel(t, "- a\n\n- b\n")
	send(t, m, k(tea.KeyEsc), k(tea.KeyDown), runes("O"))
	typeText(t, m, "new")
	send(t, m, k(tea.KeyEsc))
	if got, want := outline(t, m), []string{"a", "new", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}

	m = treeModel(t, "- a\n\n- b\n")
	send(t, m, k(tea.KeyEsc), runes("O"))
	typeText(t, m, "first")
	send(t, m, k(tea.KeyEsc))
	if got, want := outline(t, m), []string{"first", "a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestOBelowABlockWithChildrenIsWhatIsDrawnBelow(t *testing.T) {
	// Children showing: the row below is the first child, so that is where.
	m := treeModel(t, "- a\n  - c\n\n- b\n")
	send(t, m, k(tea.KeyEsc), runes("o"))
	typeText(t, m, "new")
	send(t, m, k(tea.KeyEsc))
	if got, want := outline(t, m), []string{"a", ".new", ".c", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}

	// Folded: the row below is the next sibling.
	m = treeModel(t, "- a\n  - c\n\n- b\n")
	send(t, m, k(tea.KeyEsc), runes("z"), runes("o"))
	typeText(t, m, "new")
	send(t, m, k(tea.KeyEsc))
	if got, want := outline(t, m), []string{"a", ".c", "new", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestOOnANestedBlockStaysAtItsDepth(t *testing.T) {
	m := treeModel(t, "- a\n  - c\n")
	send(t, m, k(tea.KeyEsc), k(tea.KeyDown), runes("O"))
	typeText(t, m, "new")
	send(t, m, k(tea.KeyEsc))
	if got, want := outline(t, m), []string{"a", ".new", ".c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestOWhileWritingIsALetter(t *testing.T) {
	m := treeModel(t, "- a\n")
	typeText(t, m, "o")
	send(t, m, k(tea.KeyEsc))
	if got := outline(t, m); !reflect.DeepEqual(got, []string{"ao"}) {
		t.Fatalf("got %q", got)
	}
}
