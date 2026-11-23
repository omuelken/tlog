package markdown

import (
	"strings"
)

// IndentUnit is the canonical indentation of one outline level.
const IndentUnit = "  "

// blankLineBetweenTopLevel controls whether canonical output separates
// top-level blocks with a blank line. The line goes only beside a block that
// has children — a run of one-line blocks stays a tight list, and an outline
// with sub-blocks stands apart from its neighbours (see Gap). It makes files
// easier to read and git hunks easier to follow, and it is a setting rather
// than a constant because it is a preference — see SetBlankLines.
var blankLineBetweenTopLevel = true

// SetBlankLines changes the canonical form. Each file is reformatted the next
// time that file is written; content is untouched, but the diff will be large.
func SetBlankLines(on bool) { blankLineBetweenTopLevel = on }

// Render produces the canonical form of a document. tlog owns the formatting
// of files it writes: the first write normalises a file, and from then on a
// full re-render is byte-identical except where something actually changed,
// which is what keeps diffs small without a source-preserving patcher.
func Render(d *Document) []byte {
	var sb strings.Builder

	if len(d.Frontmatter) > 0 {
		sb.WriteString("---\n")
		for _, p := range d.Frontmatter {
			sb.WriteString(p.Key)
			sb.WriteString(": ")
			sb.WriteString(p.Value)
			sb.WriteByte('\n')
		}
		sb.WriteString("---\n\n")
	}

	for i, b := range d.Blocks {
		if i > 0 && blankLineBetweenTopLevel && Gap(d.Blocks[i-1], b) {
			sb.WriteByte('\n')
		}
		writeBlock(&sb, b, 0)
	}

	out := sb.String()
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return nil
	}
	return []byte(out + "\n")
}

// Gap reports whether a blank line separates two adjacent top-level blocks:
// it does when either of them has children. The outliner spaces the screen
// with the same rule, so it lives here once.
func Gap(prev, next *Block) bool {
	return len(prev.Children) > 0 || len(next.Children) > 0
}

// RenderBlock renders a single block and its descendants at the given depth,
// as it would appear inside a file. Used when splicing a subtree.
func RenderBlock(b *Block, depth int) []byte {
	var sb strings.Builder
	writeBlock(&sb, b, depth)
	return []byte(sb.String())
}

func writeBlock(sb *strings.Builder, b *Block, depth int) {
	indent := strings.Repeat(IndentUnit, depth)
	cont := indent + IndentUnit

	lines := strings.Split(b.Text, "\n")
	first := lines[0]

	sb.WriteString(indent)
	sb.WriteString("-")
	if first != "" {
		sb.WriteString(" ")
		sb.WriteString(first)
	}
	if b.Anchor != "" {
		sb.WriteString(" ^")
		sb.WriteString(b.Anchor)
	}
	sb.WriteByte('\n')

	for _, l := range lines[1:] {
		if l == "" {
			sb.WriteByte('\n')
			continue
		}
		sb.WriteString(cont)
		sb.WriteString(l)
		sb.WriteByte('\n')
	}

	for _, p := range b.Props {
		sb.WriteString(cont)
		sb.WriteString(p.Key)
		sb.WriteString(":: ")
		sb.WriteString(p.Value)
		sb.WriteByte('\n')
	}

	for _, c := range b.Children {
		writeBlock(sb, c, depth+1)
	}
}

// IsCanonical reports whether a file is already in canonical form, i.e.
// whether writing it back would be a no-op.
func IsCanonical(src []byte) bool {
	return string(Render(Parse(src))) == string(src)
}
