package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tapp "github.com/tilman-schieber/tlog/internal/app"
)

// The API is a bridge and nothing more, so these tests check the bridging:
// that an edit comes back with a page the frontend can draw and an offset it
// can address next, and that a stale hash is refused rather than applied.

func newAPI(t *testing.T) *API {
	t.Helper()
	svc, err := tapp.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &API{svc: svc}
}

func TestTodayStartsEmptyAndTakesText(t *testing.T) {
	a := newAPI(t)
	p, err := a.Today()
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsJournal || len(p.Blocks) != 0 {
		t.Fatalf("a fresh journal should be empty: %+v", p)
	}

	e, err := a.AppendBlock(p.Rel, p.Hash, "first thought")
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Page.Blocks) != 1 || e.Page.Blocks[0].Text != "first thought" {
		t.Fatalf("blocks: %+v", e.Page.Blocks)
	}
	if e.Page.Hash == p.Hash {
		t.Fatal("the hash must move when the file changes")
	}
}

// The frontend chains operations using only what came back from the last one.
// If the returned offset or hash were wrong, the second call would fail.
func TestEditsChainWithoutReloading(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()

	e, err := a.AppendBlock(p.Rel, p.Hash, "parent")
	if err != nil {
		t.Fatal(err)
	}
	e, err = a.NewBlock(e.Page.Rel, e.Offset, e.Page.Hash, "child", false)
	if err != nil {
		t.Fatal(err)
	}
	e, err = a.Indent(e.Page.Rel, e.Offset, e.Page.Hash)
	if err != nil {
		t.Fatal(err)
	}
	e, err = a.SetText(e.Page.Rel, e.Offset, e.Page.Hash, "child, edited")
	if err != nil {
		t.Fatal(err)
	}
	e, err = a.ToggleTask(e.Page.Rel, e.Offset, e.Page.Hash)
	if err != nil {
		t.Fatal(err)
	}

	if len(e.Page.Blocks) != 2 {
		t.Fatalf("blocks: %+v", e.Page.Blocks)
	}
	b := e.Page.Blocks[1]
	if b.Depth != 1 || b.Task != "open" || !strings.Contains(b.Text, "child, edited") {
		t.Fatalf("second block: %+v", b)
	}
	if !e.Page.Blocks[0].HasChildren {
		t.Fatal("the parent should report children")
	}
}

func TestStaleHashIsRefused(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()
	e, _ := a.AppendBlock(p.Rel, p.Hash, "mine")

	// Something else writes: nvim, the TUI, an agent.
	if err := a.svc.Store.Write(e.Page.Rel, []byte("- theirs\n"), ""); err != nil {
		t.Fatal(err)
	}

	if _, err := a.SetText(e.Page.Rel, e.Offset, e.Page.Hash, "clobbered"); err == nil {
		t.Fatal("a stale edit was accepted")
	}
	f, _ := a.svc.Store.Read(e.Page.Rel)
	if string(f.Data) != "- theirs\n" {
		t.Fatalf("the other writer's work was destroyed: %q", f.Data)
	}
}

func TestFollowingALinkAndTheTagPage(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()
	if _, err := a.AppendBlock(p.Rel, p.Hash, "met [[Ada Lovelace #person]] about the plan"); err != nil {
		t.Fatal(err)
	}

	ada, err := a.OpenPage("Ada Lovelace")
	if err != nil {
		t.Fatal(err)
	}
	if len(ada.Tags) != 1 || ada.Tags[0] != "person" {
		t.Fatalf("tags: %v", ada.Tags)
	}
	if len(ada.Refs) != 1 || !strings.Contains(ada.Refs[0].Text, "the plan") {
		t.Fatalf("refs: %+v", ada.Refs)
	}

	person, err := a.OpenPage("person")
	if err != nil {
		t.Fatal(err)
	}
	if len(person.Tagged) != 1 || person.Tagged[0] != "Ada Lovelace" {
		t.Fatalf("tagged: %v", person.Tagged)
	}
}

func TestBrowsingDaysCreatesNothing(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()
	for i := 0; i < 3; i++ {
		var err error
		p, err = a.Journal(p.Rel, -1)
		if err != nil {
			t.Fatal(err)
		}
	}
	files, err := a.svc.Store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("browsing wrote files: %v", files)
	}
}

func TestIndexSearchAndCompletion(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()
	if _, err := a.AppendBlock(p.Rel, p.Hash, "use sqlite as a cache for [[Project Foo #project]]"); err != nil {
		t.Fatal(err)
	}

	idx, err := a.Index()
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Journals) != 1 || len(idx.Tags) != 1 {
		t.Fatalf("index: %+v", idx)
	}

	hits, err := a.Search("sqlite cache")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Offset != 0 && hits[0].Hash == "" {
		t.Fatalf("hits: %+v", hits)
	}

	pages, err := a.CompletePages("Pro")
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 || pages[0] != "Project Foo" {
		t.Fatalf("completion: %v", pages)
	}
	tags, err := a.CompleteTags("pro")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "project" {
		t.Fatalf("tag completion: %v", tags)
	}
}

func TestDeleteAndMove(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()
	e, _ := a.AppendBlock(p.Rel, p.Hash, "one")
	e, _ = a.NewBlock(e.Page.Rel, e.Offset, e.Page.Hash, "two", false)

	e, err := a.Move(e.Page.Rel, e.Offset, e.Page.Hash, -1)
	if err != nil {
		t.Fatal(err)
	}
	if e.Page.Blocks[0].Text != "two" {
		t.Fatalf("move: %+v", e.Page.Blocks)
	}

	e, err = a.DeleteBlock(e.Page.Rel, e.Offset, e.Page.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Page.Blocks) != 1 || e.Page.Blocks[0].Text != "one" {
		t.Fatalf("delete: %+v", e.Page.Blocks)
	}
}

// Clicking a journal in the sidebar, or following [[2026-09-17]], used to
// resolve into pages/ and show an empty file while creating junk.
func TestJournalsOpenByNameNotAsPages(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()
	e, err := a.AppendBlock(p.Rel, p.Hash, "the real day")
	if err != nil {
		t.Fatal(err)
	}
	name := e.Page.Title

	got, err := a.OpenPage(name)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsJournal || len(got.Blocks) != 1 || got.Blocks[0].Text != "the real day" {
		t.Fatalf("opening %q gave %q with %d blocks", name, got.Rel, len(got.Blocks))
	}

	files, err := a.svc.Store.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if !strings.HasPrefix(f, "journals/") {
			t.Fatalf("a junk page was created: %q", f)
		}
	}
}

// --- slash commands ---------------------------------------------------------

func TestCommandMenuAndDatePreview(t *testing.T) {
	a := newAPI(t)

	if len(a.Commands("")) == 0 {
		t.Fatal("a bare slash should offer the whole menu")
	}
	if got := a.Commands("dl"); len(got) == 0 || got[0].Name != "deadline" {
		t.Fatalf("alias not matched: %+v", got)
	}

	// The menu shows what a shorthand means before anything is written.
	p := a.DatePreview("morgen")
	if p.ISO != time.Now().AddDate(0, 0, 1).Format("2006-01-02") {
		t.Fatalf("preview: %+v", p)
	}
	if p.Label == "" || p.Short == "" {
		t.Fatalf("preview should be readable: %+v", p)
	}
	// Halfway through typing, there is simply no date yet.
	if got := a.DatePreview("mor"); got.ISO != "" {
		t.Fatalf("half-typed text should not resolve: %+v", got)
	}
}

func TestRunCommandFromTheApp(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()
	e, err := a.AppendBlock(p.Rel, p.Hash, "Bot testen /deadline morgen")
	if err != nil {
		t.Fatal(err)
	}

	// The frontend passes the rune offsets of the "/name arg" run.
	text := "Bot testen /deadline morgen"
	e2, err := a.RunCommand(e.Page.Rel, e.Offset, e.Page.Hash,
		"deadline", "morgen", text, 11, len([]rune(text)))
	if err != nil {
		t.Fatal(err)
	}

	b := e2.Page.Blocks[0]
	if b.Due != time.Now().AddDate(0, 0, 1).Format("2006-01-02") {
		t.Fatalf("due: %+v", b)
	}
	if b.DueState == "" || b.DueLabel == "" {
		t.Fatalf("the app needs the resolved state to colour it: %+v", b)
	}
	if strings.Contains(b.Text, "/deadline") || strings.Contains(b.Text, "morgen") {
		t.Fatalf("the command survived: %q", b.Text)
	}
}

func TestDueListsWhatIsOpen(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()

	e, _ := a.AppendBlock(p.Rel, p.Hash, "[ ] offen")
	e, _ = a.RunCommand(e.Page.Rel, e.Offset, e.Page.Hash, "deadline", "morgen", "[ ] offen", 9, 9)
	e2, _ := a.AppendBlock(e.Page.Rel, e.Page.Hash, "[x] erledigt")
	_, _ = a.RunCommand(e2.Page.Rel, e2.Offset, e2.Page.Hash, "deadline", "morgen", "[x] erledigt", 12, 12)

	open, err := a.Due(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].Text != "offen" {
		t.Fatalf("open: %+v", open)
	}
	all, _ := a.Due(true)
	if len(all) != 2 {
		t.Fatalf("all: %+v", all)
	}
	// Finished work is never overdue, however long ago it was due.
	for _, it := range all {
		if it.Done && it.State != "" {
			t.Fatalf("a done item was given a state: %+v", it)
		}
	}
}

// The agenda ticks a line off by its address alone: an open task and a block
// that only has a deadline are both closed, and /todo opens one again.
func TestAgendaClosesWhatIsDue(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()

	e, _ := a.AppendBlock(p.Rel, p.Hash, "[ ] offen")
	e, _ = a.RunCommand(e.Page.Rel, e.Offset, e.Page.Hash, "deadline", "morgen", "[ ] offen", 9, 9)
	e2, _ := a.AppendBlock(e.Page.Rel, e.Page.Hash, "nur ein Termin")
	_, _ = a.RunCommand(e2.Page.Rel, e2.Offset, e2.Page.Hash, "deadline", "morgen", "nur ein Termin", 14, 14)

	for range 2 {
		due, _ := a.Due(false)
		if len(due) == 0 {
			t.Fatal("nothing left to close")
		}
		it := due[0]
		if _, err := a.ApplyCommand(it.Rel, it.Offset, it.Hash, "done", ""); err != nil {
			t.Fatal(err)
		}
	}
	if open, _ := a.Due(false); len(open) != 0 {
		t.Fatalf("still open: %+v", open)
	}
	all, _ := a.Due(true)
	if len(all) != 2 || !all[0].Done || !all[1].Done {
		t.Fatalf("both should be done, the plain one made a task: %+v", all)
	}

	it := all[0]
	e3, err := a.ApplyCommand(it.Rel, it.Offset, it.Hash, "todo", "")
	if err != nil {
		t.Fatal(err)
	}
	if open, _ := a.Due(false); len(open) != 1 {
		t.Fatalf("/todo should open it again: %+v", open)
	}
	// A stale address is refused, never written through.
	if _, err := a.ApplyCommand(it.Rel, it.Offset, it.Hash, "done", ""); err == nil {
		t.Fatalf("a stale hash was accepted; the file is at %s", e3.Page.Hash)
	}
}

func TestMonthIsLaidOutMondayFirst(t *testing.T) {
	a := newAPI(t)
	cal := a.Month("2026-09-24")
	if cal.Title != "September 2026" {
		t.Fatalf("title: %q", cal.Title)
	}
	// September 2026 starts on a Tuesday, so exactly one blank leads it.
	if cal.Days[0].Day != 0 || cal.Days[1].Day != 1 {
		t.Fatalf("leading blanks wrong: %+v", cal.Days[:4])
	}
	var sel, days int
	for _, d := range cal.Days {
		if d.Day > 0 {
			days++
		}
		if d.Sel {
			sel = d.Day
		}
	}
	if days != 30 || sel != 24 {
		t.Fatalf("days=%d selected=%d", days, sel)
	}
	// An unparseable date still yields a usable month rather than nothing.
	if got := a.Month("nonsense"); len(got.Days) == 0 {
		t.Fatal("a bad date should still draw a calendar")
	}
}

// --- attachments ------------------------------------------------------------

func TestAttachFromTheApp(t *testing.T) {
	src := filepath.Join(t.TempDir(), "Quartalsbericht.pdf")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := newAPI(t)
	p, _ := a.Today()

	// What a drop on the window means: the file into assets/, the link into
	// the page being read.
	e, err := a.AttachFiles(p.Rel, []string{src})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Page.Blocks) != 1 || e.Page.Blocks[0].Text != "[Quartalsbericht.pdf](../assets/Quartalsbericht.pdf)" {
		t.Fatalf("blocks: %+v", e.Page.Blocks)
	}
	if !strings.Contains(e.Page.Blocks[0].Text, "Quartalsbericht.pdf") {
		t.Fatalf("link: %q", e.Page.Blocks[0].Text)
	}

	// The original stays where it was.
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("the original was moved: %v", err)
	}
	// And the file is in the notes.
	if _, err := os.Stat(filepath.Join(a.AttachDir(), "Quartalsbericht.pdf")); err != nil || filepath.Base(a.AttachDir()) != "assets" {
		t.Fatalf("not in the notes' assets/: %q %v", a.AttachDir(), err)
	}

	found, err := a.Attachments("quartal")
	if err != nil || len(found) != 1 {
		t.Fatalf("lookup: %+v %v", found, err)
	}
	if found[0].Size == "" || found[0].When == "" || found[0].Link == "" {
		t.Fatalf("an attachment should be showable: %+v", found[0])
	}
}

func TestAttachingNothingIsRefused(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()
	if _, err := a.AttachFiles(p.Rel, nil); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestSettingsFromTheApp(t *testing.T) {
	t.Setenv("TLOG_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	a := newAPI(t)

	items := a.Settings()
	if len(items) == 0 {
		t.Fatal("no settings offered")
	}
	// Every setting must say what it is, or the panel is a list of riddles.
	for _, s := range items {
		if s.Key == "" || s.Kind == "" || s.Hint == "" {
			t.Fatalf("incomplete: %+v", s)
		}
	}

	res, err := a.SetSetting("attachments.sanitize", "true")
	if err != nil {
		t.Fatal(err)
	}
	if !a.svc.Cfg.Attachments.Sanitize {
		t.Fatal("not applied")
	}
	if res.Note != "" {
		t.Fatalf("this one takes effect at once: %q", res.Note)
	}

	res, err = a.SetSetting("notes", "~/zettel")
	if err != nil {
		t.Fatal(err)
	}
	if res.Note == "" {
		t.Fatal("a setting that needs a restart should say so")
	}

	if _, err := a.SetSetting("git.debounce", "banana"); err == nil {
		t.Fatal("nonsense should be refused")
	}
	if _, err := a.SetSetting("nope", "x"); err == nil {
		t.Fatal("an unknown key should be refused")
	}
}

// Enter in the window is one call now. It used to be SetText followed by
// NewBlock, so the frontend has to be able to chain from what Split hands back.
func TestSplitAndMergeChainLikeTheFrontendDoes(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()
	e, err := a.AppendBlock(p.Rel, p.Hash, "onetwo")
	if err != nil {
		t.Fatal(err)
	}

	e, err = a.SplitBlock(e.Page.Rel, e.Offset, e.Page.Hash, "one", "two", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Page.Blocks) != 2 {
		t.Fatalf("blocks: %+v", e.Page.Blocks)
	}
	// The offset must address the new block, because that is where the caret
	// goes and what the next keystroke edits.
	if b := blockAt(e.Page.Blocks, e.Offset); b == nil || b.Text != "two" {
		t.Fatalf("split returned %d, blocks %+v", e.Offset, e.Page.Blocks)
	}

	e, err = a.MergeIntoPrevious(e.Page.Rel, e.Offset, e.Page.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Page.Blocks) != 1 || e.Page.Blocks[0].Text != "onetwo" {
		t.Fatalf("merge: %+v", e.Page.Blocks)
	}
	if b := blockAt(e.Page.Blocks, e.Offset); b == nil {
		t.Fatalf("merge returned %d, which is no block", e.Offset)
	}
}

func TestSplitRefusesAStaleHash(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()
	e, _ := a.AppendBlock(p.Rel, p.Hash, "mine")
	stale := e.Page.Hash
	if _, err := a.SetText(e.Page.Rel, e.Offset, stale, "theirs"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SplitBlock(e.Page.Rel, e.Offset, stale, "mi", "ne", false); err == nil {
		t.Fatal("the window overwrote an edit it had not seen")
	}
}

func blockAt(bs []tapp.BlockView, off int) *tapp.BlockView {
	for i := range bs {
		if bs[i].Offset == off {
			return &bs[i]
		}
	}
	return nil
}

func TestPastingAScreenshotFromTheApp(t *testing.T) {
	a := newAPI(t)

	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	link, err := a.PasteImage(base64.StdEncoding.EncodeToString(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(link, "![screenshot-") || !strings.Contains(link, "](../assets/screenshot-") || !strings.HasSuffix(link, ".png)") {
		t.Fatalf("link: %q", link)
	}
	entries, _ := os.ReadDir(a.AttachDir())
	if len(entries) != 1 {
		t.Fatalf("assets/ holds %d files", len(entries))
	}

	if _, err := a.PasteImage(base64.StdEncoding.EncodeToString([]byte("just words"))); err == nil {
		t.Fatal("text was accepted as a picture")
	}
	if _, err := a.PasteImage("not base64 at all!"); err == nil {
		t.Fatal("garbage was accepted")
	}
}

// The window keeps the same recent list as the outliner, in the same file, and
// names the journals page so that it can be shown and gone to.
func TestRecentIsSharedAndNamesTheJournalsPage(t *testing.T) {
	t.Setenv("TLOG_STATE", filepath.Join(t.TempDir(), "recent.json"))
	a := newAPI(t)
	if _, err := a.OpenPage("Ada"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.svc.AppendBlock("pages/Ada.md", "", "x"); err != nil {
		t.Fatal(err)
	}

	r := a.Visited("pages/Ada.md")
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	if len(r.Places) == 0 || r.Places[0].Rel != "pages/Ada.md" || r.Places[0].Title != "Ada" {
		t.Fatalf("visited: %+v", r.Places)
	}
	last := r.Places[len(r.Places)-1]
	if last.Rel != a.JournalsRel() || last.Title != "Journals" {
		t.Fatalf("the journals page should always be there, last: %+v", r.Places)
	}

	again := a.Recent()
	if len(again.Places) != len(r.Places) || again.Places[0].Rel != "pages/Ada.md" {
		t.Fatalf("not remembered: %+v", again.Places)
	}

	// The outliner reads the same file, so what the window visited is there.
	if got := a.svc.Recent(); got[0] != "pages/Ada.md" {
		t.Fatalf("the core does not see it: %v", got)
	}
}

// A list that could not be written is still a list.
func TestVisitedSaysWhyItCouldNotRemember(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TLOG_STATE", filepath.Join(blocker, "recent.json"))
	a := newAPI(t)
	r := a.Visited(a.JournalsRel())
	if r.Error == "" {
		t.Fatal("an unwritable state file went unmentioned")
	}
	if len(r.Places) == 0 || r.Places[0].Rel != a.JournalsRel() {
		t.Fatalf("the list was lost with the write: %+v", r.Places)
	}
}

// Every day on the journals page is its own page: an edit made there goes to
// that day, through the same calls as on the day's own page.
func TestJournalsPageEditsEachDayOnItsOwn(t *testing.T) {
	a := newAPI(t)
	today := a.svc.TodayRel()
	yesterday := a.svc.JournalRel(time.Now().AddDate(0, 0, -1))
	if _, err := a.svc.AppendBlock(yesterday, "", "yesterday's note"); err != nil {
		t.Fatal(err)
	}

	rels, err := a.JournalDays()
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 2 || rels[0] != today || rels[1] != yesterday {
		t.Fatalf("days: %v", rels)
	}

	days, err := a.OpenDays(rels)
	if err != nil {
		t.Fatal(err)
	}
	if !days[0].Today || days[1].Today || days[0].Weekday == "" || days[1].Ahead {
		t.Fatalf("headings: %+v %+v", days[0], days[1])
	}
	if len(days[0].Page.Blocks) != 0 {
		t.Fatal("today has no file yet and should be empty")
	}
	if _, err := os.Stat(a.svc.Store.Abs(today)); err == nil {
		t.Fatal("showing the journals page wrote today's file")
	}

	y := days[1].Page
	e, err := a.NewBlock(y.Rel, y.Blocks[0].Offset, y.Hash, "and another", false)
	if err != nil {
		t.Fatal(err)
	}
	if e.Page.Rel != yesterday || len(e.Page.Blocks) != 2 {
		t.Fatalf("the edit went to %s: %+v", e.Page.Rel, e.Page.Blocks)
	}

	// A day's edge is a wall: the first block of a day cannot be indented
	// under the last block of the day above it.
	if _, err := a.Indent(e.Page.Rel, e.Page.Blocks[0].Offset, e.Page.Hash); err == nil {
		t.Fatal("the first block of a day was indented across the date")
	}
}

func TestALinkedTextFileIsShownNotHandedOver(t *testing.T) {
	a := newAPI(t)
	p, _ := a.Today()
	var launched []string
	a.svc.Launch = func(target string) error { launched = append(launched, target); return nil }

	dir := filepath.Join(a.Root(), "assets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"Regeln.md": "# Regeln\n", "run.py": "def f(): pass\n", "plot.png": "\x89PNG\x00"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Relative to the note it is written in, as the outliner resolves it.
	f, err := a.OpenURL(p.Rel, "../assets/Regeln.md")
	if err != nil || f == nil || f.Name != "Regeln.md" || f.Text != "# Regeln\n" || len(f.Tokens) != 0 {
		t.Fatalf("markdown: %+v %v", f, err)
	}
	if f, err := a.OpenURL(p.Rel, "../assets/run.py"); err != nil || f == nil || len(f.Tokens) == 0 {
		t.Fatalf("code should come with its colours: %+v %v", f, err)
	}
	if len(launched) != 0 {
		t.Fatalf("a text file went to the desktop: %v", launched)
	}
	if f, err := a.OpenURL(p.Rel, "../assets/plot.png"); err != nil || f != nil || len(launched) != 1 {
		t.Fatalf("a picture should go to the desktop: %+v %v %v", f, err, launched)
	}
	if err := a.OpenOutside(filepath.Join(dir, "Regeln.md")); err != nil || len(launched) != 2 {
		t.Fatalf("open outside: %v %v", err, launched)
	}
}
