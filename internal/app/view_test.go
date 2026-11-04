package app

import "testing"

func TestPlacesRanksEverywhereYouCouldGo(t *testing.T) {
	s := newSvc(t)
	if _, err := s.AddToPage("Ada Lovelace", "the first programmer [[engine #project]]"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddToPage("Grace Hopper", "the first compiler"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddToday("a note"); err != nil {
		t.Fatal(err)
	}

	all, err := s.Places("", 0)
	if err != nil {
		t.Fatal(err)
	}
	var kinds = map[string]int{}
	for _, p := range all {
		kinds[p.Kind]++
	}
	if kinds["journal"] == 0 || kinds["page"] == 0 || kinds["tag"] == 0 {
		t.Fatalf("not everywhere: %+v", kinds)
	}
	// A journal is what is most often being looked for, so it comes first.
	if all[0].Kind != "journal" {
		t.Fatalf("got %+v", all[0])
	}

	// The same fuzzy ranking the [[link]] completion uses, so a name that
	// completes one way is found the other way too.
	hits, err := s.Places("lov", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].Name != "Ada Lovelace" {
		t.Fatalf("got %+v", hits)
	}
	if hits[0].Rel != "pages/Ada Lovelace.md" {
		t.Fatalf("a page must carry its path so opening it creates nothing: %+v", hits[0])
	}

	// A tag may have no file of its own, and that is not an error.
	for _, p := range all {
		if p.Kind == "tag" && p.Rel != "" {
			t.Fatalf("a tag should carry no path: %+v", p)
		}
	}
}

// Open items come first, soonest first: what is left to do. What is done
// follows, newest first — the last things finished are the ones worth seeing,
// and the ones most likely to have been ticked by mistake. No date of
// finishing is stored, so newest means latest deadline.
func TestDueListsOpenSoonestThenDoneNewest(t *testing.T) {
	s := newSvc(t)
	add := func(text, due string, done bool) {
		t.Helper()
		if !done {
			if _, err := s.AddTodoTo("pages/Plan.md", text, due); err != nil {
				t.Fatal(err)
			}
			return
		}
		res, err := s.AddTodoTo("pages/Plan.md", text, due)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.ToggleTask(res.Addr); err != nil {
			t.Fatal(err)
		}
	}
	add("done long ago", "2026-01-10", true)
	add("open later", "2026-03-01", false)
	add("done lately", "2026-02-20", true)
	add("open soon", "2026-02-01", false)
	add("done in between", "2026-02-01", true)

	all, err := s.Due(true)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range all {
		got = append(got, it.Text)
	}
	want := []string{"open soon", "open later", "done lately", "done in between", "done long ago"}
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %q, want %q", got, want)
		}
	}

	open, _ := s.Due(false)
	if len(open) != 2 || open[0].Text != "open soon" {
		t.Fatalf("open only: %+v", open)
	}
}
