package dates

import (
	"regexp"
	"strings"
	"time"
)

// Suggestion is one entry in a date quick pick: what to type for it, and the
// day it means.
type Suggestion struct {
	Phrase string
	Day    time.Time
}

// phrases is what a quick pick offers to complete, in the order it offers
// them. Every one of them goes through Parse, so a suggestion can never mean
// something different from typing it out.
var phrases = []string{
	"today", "tomorrow",
	"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday",
	"next week", "in 2 weeks", "next month", "end of week", "end of month", "end of year",
	"heute", "morgen", "übermorgen",
	"montag", "dienstag", "mittwoch", "donnerstag", "freitag", "samstag", "sonntag",
	"nächste woche", "nächsten monat", "monatsende", "jahresende",
}

var countRe = regexp.MustCompile(`^(?:in\s*)?(\d{1,3})\s*$`)

// Suggest is what a date quick pick shows for what has been typed so far.
//
// With nothing typed it is the handful of days a deadline usually lands on:
// today, tomorrow, the next few working days, next week, the end of the month.
// Typing narrows it: "fr" offers Friday, "in 3" offers three days, weeks and
// months, "next" the next week and month, and anything Parse understands is
// offered as itself, first. Days are not repeated: "fri" and "freitag" are one
// suggestion.
func Suggest(input string, now time.Time, limit int) []Suggestion {
	s := strings.ToLower(strings.TrimSpace(input))
	today := day(now)
	var out []Suggestion
	seen := map[string]bool{}
	add := func(phrase string) {
		if len(out) >= limit {
			return
		}
		d, ok := Parse(phrase, now)
		if !ok || seen[Format(d)] {
			return
		}
		seen[Format(d)] = true
		out = append(out, Suggestion{Phrase: phrase, Day: d})
	}

	if s == "" {
		add("today")
		add("tomorrow")
		// The working days after tomorrow, until the week is out.
		for d := today.AddDate(0, 0, 2); d.Weekday() != time.Saturday && d.Weekday() != time.Sunday; d = d.AddDate(0, 0, 1) {
			add(strings.ToLower(d.Weekday().String()))
		}
		add("next week")
		add("in 2 weeks")
		add("end of month")
		return out
	}

	if m := countRe.FindStringSubmatch(s); m != nil {
		for _, unit := range []string{"days", "weeks", "months"} {
			add("in " + m[1] + " " + unit)
		}
	}
	// The written-out phrase before what was typed, so "fr" is offered as
	// "friday": the same day, said so it can be read back.
	for _, p := range phrases {
		if strings.HasPrefix(p, s) {
			add(p)
		}
	}
	add(s) // what was typed, if it is a date no phrase covers: "7 nov", "+3d"
	return out
}
