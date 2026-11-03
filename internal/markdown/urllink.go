package markdown

import (
	"regexp"
	"strings"
)

// URLLink is an ordinary markdown link — [label](url) or ![alt](url) — or a
// bare address written straight into the text. These are not part of the
// dialect: they are standard markdown inside a block's text, and they are
// parsed here only so that every surface shows and opens them the same way.
// `att` and `tlog attach` write exactly this form for a file on the shelf.
type URLLink struct {
	Label string // the link text, an image's alt, or the address itself when bare
	URL   string
	Image bool
	Bare  bool
	Start int // byte span of the whole link in the text it was found in
	End   int
}

// An image before a link, so ![alt](src) is not read as a link; a bare address
// only at the start of a word, so the url inside a [label](url) is not found
// twice.
var urlLinkRe = regexp.MustCompile(
	`(!?)\[([^\[\]]*)\]\(([^()\s]+)\)` +
		`|(^|\s)((?:https?|file|mailto):[^\s<>]+)`)

// URLLinks finds every link in s, in order.
func URLLinks(s string) []URLLink {
	var out []URLLink
	for _, m := range urlLinkRe.FindAllStringSubmatchIndex(s, -1) {
		if m[6] >= 0 { // [label](url), with or without the !
			// A [[wikilink]] followed by a parenthesis is not a markdown link.
			if m[0] > 0 && s[m[0]-1] == '[' {
				continue
			}
			out = append(out, URLLink{
				Label: s[m[4]:m[5]],
				URL:   s[m[6]:m[7]],
				Image: m[3] > m[2],
				Start: m[0],
				End:   m[1],
			})
			continue
		}
		url := strings.TrimRight(s[m[10]:m[11]], ".,;:!?'\")]")
		out = append(out, URLLink{
			Label: url,
			URL:   url,
			Bare:  true,
			Start: m[10],
			End:   m[10] + len(url),
		})
	}
	return out
}

// URLLinks returns the links in the block's own text, leaving out anything
// inside a fenced code sample.
func (b *Block) URLLinks() []URLLink { return URLLinks(stripFences(b.Text)) }
