package repository

import (
	"testing"
	"time"
)

func TestSortValTimeRoundTripPreservesSubSecond(t *testing.T) {
	ts := time.Date(2026, 6, 17, 5, 44, 19, 123456000, time.UTC)
	norm := normalizeSortVal(ts) // -> RFC3339Nano string (as it would be JSON-encoded in the cursor)
	parsed, err := argTime(norm)
	if err != nil {
		t.Fatal(err)
	}
	got := parsed.(time.Time)
	if !got.Equal(ts) {
		t.Fatalf("sub-second precision lost: got %v want %v", got, ts)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	in := issueCursor{Col: "updateAt", Dir: "desc", Val: "2026-06-16T10:00:00Z", Id: 42}
	s, err := encodeCursor(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := decodeCursor(s)
	if err != nil {
		t.Fatal(err)
	}
	if out.Col != in.Col || out.Dir != in.Dir || out.Id != in.Id || out.Val != in.Val {
		t.Fatalf("round trip mismatch: %+v vs %+v", *out, in)
	}
}

func TestCursorDecodeGarbage(t *testing.T) {
	if _, err := decodeCursor("!!!not-base64!!!"); err == nil {
		t.Fatal("expected error for garbage cursor")
	}
}

func TestKeysetPredicate_NotNullDesc(t *testing.T) {
	cur := &issueCursor{Col: "updateAt", Dir: "desc", Val: "2026-06-16T10:00:00Z", Id: 42}
	pred, args, idx, err := buildKeysetPredicate(cur, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := "(iss.update_at < $1 OR (iss.update_at = $1 AND iss.id_issue < $2))"
	if pred != want {
		t.Fatalf("pred = %q want %q", pred, want)
	}
	if len(args) != 2 || idx != 3 {
		t.Fatalf("args=%v idx=%d", args, idx)
	}
}

func TestKeysetPredicate_NullableDescIncludesNulls(t *testing.T) {
	cur := &issueCursor{Col: "severity", Dir: "desc", Val: float64(5), Id: 7}
	pred, _, _, _ := buildKeysetPredicate(cur, 1)
	want := "(pis.order_rank < $1 OR (pis.order_rank = $1 AND iss.id_issue < $2) OR pis.order_rank IS NULL)"
	if pred != want {
		t.Fatalf("pred = %q want %q", pred, want)
	}
}

func TestKeysetPredicate_NullCursorValue(t *testing.T) {
	cur := &issueCursor{Col: "severity", Dir: "desc", Val: nil, Id: 9}
	pred, args, _, _ := buildKeysetPredicate(cur, 1)
	want := "(pis.order_rank IS NULL AND iss.id_issue < $1)"
	if pred != want || len(args) != 1 {
		t.Fatalf("pred = %q args=%v", pred, args)
	}
}

// Saved-view validation gates on this, so an unknown key must not pass as
// sortable just because sortColumnFor would silently fall back to updateAt.
func TestIsSortColumn(t *testing.T) {
	for _, key := range []string{"updateAt", "createAt", "severity", "assignedToName"} {
		if !IsSortColumn(key) {
			t.Fatalf("IsSortColumn(%q) = false, want true", key)
		}
	}
	for _, key := range []string{"", "dropTables", "UpdateAt", "update_at"} {
		if IsSortColumn(key) {
			t.Fatalf("IsSortColumn(%q) = true, want false", key)
		}
	}
}

func TestKeysetPredicate_Asc(t *testing.T) {
	cur := &issueCursor{Col: "title", Dir: "asc", Val: "abc", Id: 3}
	pred, _, _, _ := buildKeysetPredicate(cur, 1)
	want := "(iss.title > $1 OR (iss.title = $1 AND iss.id_issue > $2))"
	if pred != want {
		t.Fatalf("pred = %q want %q", pred, want)
	}
}
