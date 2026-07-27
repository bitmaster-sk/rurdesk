package service

import (
	"testing"
	"time"
)

func TestNextName(t *testing.T) {
	if got := NextName(11); got != "Sprint 12" {
		t.Fatalf("got %q, want Sprint 12", got)
	}
	if got := NextName(0); got != "Sprint 1" {
		t.Fatalf("got %q, want Sprint 1", got)
	}
}

func TestDefaultWindow(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	latest := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	start, end := DefaultWindow(&latest, now)
	if !start.Equal(latest) || end.Sub(start) != 14*24*time.Hour {
		t.Fatalf("future latest end: expected latest+14d, got %v..%v", start, end)
	}

	start, end = DefaultWindow(nil, now)
	if !start.Equal(now) || end.Sub(start) != 14*24*time.Hour {
		t.Fatalf("no prior sprint: expected now+14d, got %v..%v", start, end)
	}

	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	start, _ = DefaultWindow(&past, now)
	if !start.Equal(now) {
		t.Fatalf("past latest end must not push start into the past, got %v", start)
	}
}
