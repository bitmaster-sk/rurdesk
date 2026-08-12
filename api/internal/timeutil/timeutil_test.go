package timeutil

import (
	"testing"
	"time"
)

func TestTruncateClock(t *testing.T) {
	cases := []struct {
		in   time.Time
		want time.Time
	}{
		{
			time.Date(2026, 8, 10, 23, 59, 59, 999, time.UTC),
			time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, c := range cases {
		if got := TruncateClock(c.in); !got.Equal(c.want) {
			t.Fatalf("TruncateClock(%v): got %v, want %v", c.in, got, c.want)
		}
	}
}

func TestTruncateClockKeepsLocation(t *testing.T) {
	zone := time.FixedZone("UTC+2", 2*60*60)
	got := TruncateClock(time.Date(2026, 8, 10, 17, 30, 0, 0, zone))
	if got.Location() != zone {
		t.Fatalf("location: got %v, want %v", got.Location(), zone)
	}
	if got.Hour() != 0 || got.Day() != 10 {
		t.Fatalf("got %v, want 2026-08-10 00:00 +02", got)
	}
}

func TestTruncateClockOnUtcGivesTheUtcDay(t *testing.T) {
	local := time.Date(2026, 8, 10, 0, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))

	want := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	if got := TruncateClock(local.UTC()); !got.Equal(want) {
		t.Fatalf("TruncateClock(t.UTC()): got %v, want %v", got, want)
	}

	if got := TruncateClock(local).UTC(); got.Equal(want) {
		t.Fatalf("TruncateClock(t).UTC() must not equal the UTC day, got %v", got)
	}
}
