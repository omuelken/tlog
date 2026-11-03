package app

import (
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/tilman-schieber/tlog/internal/fuzzy"
)

// PersonTag is the tag that makes a page a person. It is matched without
// regard to case, so the "tags: Person" a Logseq import writes and a
// [[Ada #person]] typed here mean the same thing.
const PersonTag = "person"

// Mention is one entry in the @ menu: a person, a date, another page, or a
// person who does not exist yet. Whatever it is, accepting it writes an
// ordinary link — people are special in how they are found, not in how they
// are stored.
type Mention struct {
	Kind   string `json:"kind"`   // "person", "date", "page" or "new"
	Name   string `json:"name"`   // the page, or what the date is called
	Insert string `json:"insert"` // what accepting writes in place of the @…
	Detail string `json:"detail"` // why it is here: "12 mentions", "Fr 07.11.  in 4 days"
	ISO    string `json:"iso,omitempty"`
	State  string `json:"state,omitempty"`
}

// usage is how much a page is talked about: how often, and when last — the
// day of the newest journal that mentions it.
type usage struct {
	count int
	last  time.Time
}

func (l *Lookup) usage(name string) usage {
	var u usage
	for _, r := range l.g.Backlinks(name) {
		u.count++
		if r.From.IsJournal && r.From.Day.After(u.last) {
			u.last = r.From.Day
		}
	}
	return u
}

// byUse orders names by who you are writing about: most recently mentioned
// first, then most often.
func (l *Lookup) byUse(names []string) []string {
	us := make(map[string]usage, len(names))
	for _, n := range names {
		us[n] = l.usage(n)
	}
	out := append([]string(nil), names...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := us[out[i]], us[out[j]]
		if !a.last.Equal(b.last) {
			return a.last.After(b.last)
		}
		return a.count > b.count
	})
	return out
}

// match is how well a name answers a query. A name that starts with it — or
// one of its words does, or its initials are it, as "jr" is Julia
// Rausenberger's — is a strong match; any other subsequence is a loose one.
type match struct {
	name   string
	exact  bool // the name, or an alias, is what was typed
	strong bool
	score  int
}

func (l *Lookup) matchName(name, query string) (match, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	m := match{name: name}
	ok := false
	for _, n := range append([]string{name}, l.g.Aliases(name)...) {
		s, hit := fuzzy.Match(n, q)
		if !hit {
			continue
		}
		ok = true
		if strings.EqualFold(n, q) {
			m.exact = true
		}
		if s > m.score {
			m.score = s
		}
		if strongMatch(strings.ToLower(n), q) {
			m.strong = true
		}
	}
	return m, ok
}

func strongMatch(name, q string) bool {
	if strings.HasPrefix(name, q) {
		return true
	}
	var initials strings.Builder
	for _, w := range strings.Fields(name) {
		if strings.HasPrefix(w, q) {
			return true
		}
		initials.WriteString(string([]rune(w)[:1]))
	}
	return strings.HasPrefix(initials.String(), q) && len([]rune(q)) > 1
}

// rankNames orders names against a query: strong matches first, then by score,
// ties broken by use, so of two Julias the one you write about wins. With no
// query it is use alone.
func (l *Lookup) rankNames(names []string, query string) []match {
	ordered := l.byUse(names)
	if strings.TrimSpace(query) == "" {
		out := make([]match, len(ordered))
		for i, n := range ordered {
			out[i] = match{name: n, strong: true}
		}
		return out
	}
	var hits []match
	for _, n := range ordered {
		if m, ok := l.matchName(n, query); ok {
			hits = append(hits, m)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].exact != hits[j].exact {
			return hits[i].exact
		}
		if hits[i].strong != hits[j].strong {
			return hits[i].strong
		}
		return hits[i].score > hits[j].score
	})
	return hits
}

// People is every page tagged as a person.
func (l *Lookup) People() []string { return l.g.PagesWithTag(PersonTag) }

// Mentions is the @ menu for what has been typed after the @.
//
// People come first, because they are most of what gets mentioned; then the
// days the typing could mean, so @fri still works; then other pages. A name
// that starts with what was typed beats a loose match whatever it is, so @noa
// finds the page NOA before a person whose name merely contains n, o and a.
// When nobody matches, or a whole name has been typed, the last entry makes a
// new person: meeting someone and writing them down is the same act. It writes
// [[Name #person]], which is how the dialect already says what a page is.
func (l *Lookup) Mentions(query string, limit int) []Mention {
	q := strings.TrimSpace(query)

	people := l.People()
	isPerson := map[string]bool{}
	for _, p := range people {
		isPerson[strings.ToLower(p)] = true
	}
	personHits := take(l.rankNames(people, q), 5)

	var pageHits []match
	if q != "" {
		var pages []string
		for _, n := range l.targets() {
			if !isPerson[strings.ToLower(n)] {
				pages = append(pages, n)
			}
		}
		pageHits = take(l.rankNames(pages, q), 4)
	}

	person := func(m match) Mention {
		return Mention{Kind: "person", Name: m.name, Insert: "[[" + m.name + "]]", Detail: plural(l.usage(m.name).count, "mention")}
	}
	page := func(m match) Mention {
		return Mention{Kind: "page", Name: m.name, Insert: "[[" + m.name + "]]", Detail: "page"}
	}

	var out []Mention
	// An exact name first, then strong matches of either kind, people first;
	// then the days; then the loose matches.
	if len(pageHits) > 0 && pageHits[0].exact && (len(personHits) == 0 || !personHits[0].exact) {
		out = append(out, page(pageHits[0]))
		pageHits = pageHits[1:]
	}
	for _, m := range personHits {
		if m.strong {
			out = append(out, person(m))
		}
	}
	for _, m := range pageHits {
		if m.strong {
			out = append(out, page(m))
		}
	}
	for _, d := range DateChoices(q, 3) {
		out = append(out, Mention{
			Kind: "date", Name: d.Phrase, Insert: "[[" + d.ISO + "]]",
			Detail: d.Short + "  " + d.Label, ISO: d.ISO, State: d.State,
		})
	}
	for _, m := range personHits {
		if !m.strong {
			out = append(out, person(m))
		}
	}
	for _, m := range pageHits {
		if !m.strong {
			out = append(out, page(m))
		}
	}
	out = take(out, limit)

	strongPerson := len(personHits) > 0 && personHits[0].strong
	if q != "" && !l.exists(q) && looksLikeAName(q) && (!strongPerson || strings.Contains(q, " ")) {
		if len(out) >= limit {
			out = out[:limit-1] // the way to make someone new is never crowded out
		}
		out = append(out, Mention{Kind: "new", Name: q, Insert: "[[" + q + " #" + PersonTag + "]]", Detail: "new person"})
	}
	return out
}

// targets is every page a link could point at, each once, under its real
// name. An alias is not a page of its own: matching looks at a page's aliases,
// so typing one finds the page it belongs to.
func (l *Lookup) targets() []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range l.g.LinkTargets() {
		real, _ := l.g.Canonical(n)
		if k := strings.ToLower(real); !seen[k] {
			seen[k] = true
			out = append(out, real)
		}
	}
	return out
}

// exists reports whether a page or alias already answers to a name.
func (l *Lookup) exists(name string) bool {
	_, ok := l.g.Canonical(name)
	if ok {
		return true
	}
	_, ok = l.g.Page(name)
	return ok
}

// looksLikeAName keeps "@fri", "@in 3" and a stray "@here" from being offered
// as people: a name is capitalised or has a space in it, the way names are
// written.
func looksLikeAName(s string) bool {
	r := []rune(s)
	if len(r) < 2 || !(unicode.IsUpper(r[0]) || strings.Contains(s, " ")) {
		return false
	}
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return false
		}
	}
	return len(DateChoices(s, 1)) == 0 || strings.Contains(s, " ")
}

func take[T any](xs []T, n int) []T {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

func plural(n int, unit string) string {
	switch n {
	case 0:
		return "no " + unit + "s yet"
	case 1:
		return "1 " + unit
	}
	return strconv.Itoa(n) + " " + unit + "s"
}

// rankPages is the [[ completion: the same matching as @, over every name a
// link could point at, so the people and pages you write about come first.
func (l *Lookup) rankPages(prefix string, limit int) []string {
	hits := take(l.rankNames(l.targets(), prefix), limitOr(limit))
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.name
	}
	return out
}

func limitOr(n int) int {
	if n <= 0 {
		return 1 << 30
	}
	return n
}
