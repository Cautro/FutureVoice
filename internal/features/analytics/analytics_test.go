package analytics

import (
	"testing"
	"time"
)

func TestParseRangeDefaultsToLast30CalendarDays(t *testing.T) {
	now := time.Date(2026, 11, 11, 13, 30, 0, 0, time.FixedZone("local", 3*60*60))
	from, to, err := ParseRange("", "", now)
	if err != nil {
		t.Fatalf("ParseRange returned error: %v", err)
	}
	if got, want := from.Format("2006-01-02"), "2026-10-13"; got != want {
		t.Fatalf("from = %s, want %s", got, want)
	}
	if got, want := to.Format("2006-01-02"), "2026-11-11"; got != want {
		t.Fatalf("to = %s, want %s", got, want)
	}
}

func TestParseRangeAccepts366InclusiveCalendarDays(t *testing.T) {
	from, to, err := ParseRange("2025-11-11", "2026-11-11", time.Now())
	if err != nil {
		t.Fatalf("ParseRange returned error: %v", err)
	}
	if got := int(to.Sub(from).Hours() / 24); got != 365 {
		t.Fatalf("range difference = %d days, want 365", got)
	}
}

func TestParseRangeRejectsInvalidAndReversedDates(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
	}{
		{name: "invalid format", from: "11-11-2026", to: "2026-11-11"},
		{name: "reversed", from: "2026-11-12", to: "2026-11-11"},
		{name: "too long", from: "2024-11-10", to: "2026-11-11"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := ParseRange(test.from, test.to, time.Now()); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
