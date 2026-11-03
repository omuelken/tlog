package app

import (
	"strings"
	"testing"
)

func mentionNames(ms []Mention) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Kind+":"+m.Name)
	}
	return out
}

func peopleService(t *testing.T) *Lookup {
	t.Helper()
	s := newSvc(t)
	for _, p := range []string{"Julia Rausenberger", "Juliane Reck", "Oliver Mülken"} {
		if err := s.Store.Write("pages/"+p+".md", []byte("---\ntags: Person\n---\n\n- about them\n"), ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Store.Write("pages/Elke.md", []byte("---\naliases: EH\n---\n\n- x\n"), ""); err != nil {
		t.Fatal(err)
	}
	// Julia is written about most, so she comes first among the Jul… people.
	for _, line := range []string{"met [[Julia Rausenberger]]", "and [[Julia Rausenberger]] again", "[[Juliane Reck]] too", "see [[NOA]]"} {
		if _, err := s.AddToday(line); err != nil {
			t.Fatal(err)
		}
	}
	l, err := s.Lookup()
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestMentionsPutPeopleFirstByUse(t *testing.T) {
	l := peopleService(t)
	got := mentionNames(l.Mentions("", 8))
	if len(got) < 4 || got[0] != "person:Julia Rausenberger" || got[1] != "person:Juliane Reck" {
		t.Fatalf("got %v", got)
	}
	if !strings.HasPrefix(got[len(got)-1], "date:") {
		t.Fatalf("the usual days should follow the people: %v", got)
	}
	if m := l.Mentions("", 8)[0]; m.Insert != "[[Julia Rausenberger]]" || m.Detail != "2 mentions" {
		t.Fatalf("got %+v", m)
	}
}

func TestMentionsMatchInitialsAndFirstNames(t *testing.T) {
	l := peopleService(t)
	for q, want := range map[string]string{"jr": "person:Julia Rausenberger", "oli": "person:Oliver Mülken", "julia": "person:Julia Rausenberger"} {
		if got := mentionNames(l.Mentions(q, 8)); len(got) == 0 || got[0] != want {
			t.Errorf("@%s: %v", q, got)
		}
	}
	// A page by its alias, and a page by its exact name before anything loose.
	if got := mentionNames(l.Mentions("EH", 8)); len(got) == 0 || got[0] != "page:Elke" {
		t.Errorf("@EH: %v", got)
	}
	if got := mentionNames(l.Mentions("noa", 8)); len(got) == 0 || got[0] != "page:NOA" {
		t.Errorf("@noa: %v", got)
	}
}

func TestMentionsStillPickDates(t *testing.T) {
	got := peopleService(t).Mentions("fri", 8)
	if len(got) != 1 || got[0].Kind != "date" || !strings.HasPrefix(got[0].Insert, "[[20") {
		t.Fatalf("got %+v", got)
	}
}

func TestANewPersonIsOfferedOnlyWhenNobodyIsMeant(t *testing.T) {
	l := peopleService(t)
	// Halfway through typing Julia, nobody new is meant.
	for _, m := range l.Mentions("ju", 8) {
		if m.Kind == "new" {
			t.Fatalf("@ju offered a new person: %+v", m)
		}
	}
	got := l.Mentions("Maria Muster", 8)
	last := got[len(got)-1]
	if last.Kind != "new" || last.Insert != "[[Maria Muster #person]]" {
		t.Fatalf("got %+v", got)
	}
	// Someone who exists is not offered again as new.
	for _, m := range l.Mentions("Oliver Mülken", 8) {
		if m.Kind == "new" {
			t.Fatalf("an existing person was offered as new: %+v", m)
		}
	}
}
