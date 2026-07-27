package lexorank

import (
	"strings"
	"testing"
)

func TestBetweenStrictlyOrdered(t *testing.T) {
	tests := []struct{ prev, next string }{
		{"", ""}, {"", "m"}, {"m", ""}, {"a", "b"}, {"a", "z"},
		{"az", "b"}, {"aaa", "aab"}, {"gn", "na"},
	}
	for _, tc := range tests {
		got := Between(tc.prev, tc.next)
		if tc.prev != "" && !(tc.prev < got) {
			t.Fatalf("Between(%q,%q)=%q not > prev", tc.prev, tc.next, got)
		}
		if tc.next != "" && !(got < tc.next) {
			t.Fatalf("Between(%q,%q)=%q not < next", tc.prev, tc.next, got)
		}
	}
}

// Regression: inserting before a low key must NOT yield the bare floor "a".
func TestBetweenNeverReturnsBareFloor(t *testing.T) {
	if got := Between("", "c"); got == "a" {
		t.Fatalf(`Between("","c") returned bare floor "a"`)
	}
	// Repeatedly insert at the very top; must stay strictly ordered and never be "a".
	next := "n"
	for i := 0; i < 50; i++ {
		got := Between("", next)
		if !(got < next) {
			t.Fatalf("iter %d: %q not < %q", i, got, next)
		}
		if got == "a" || got == "" {
			t.Fatalf("iter %d: produced unusable floor key %q", i, got)
		}
		next = got
	}
}

func TestSeedRanksStrictlyIncreasing(t *testing.T) {
	// 675 = 26*26-1 is the two-char capacity boundary; test across the width transition.
	for _, n := range []int{1, 2, 3, 10, 50, 675, 676, 677, 2000} {
		ranks := SeedRanks(n)
		if len(ranks) != n {
			t.Fatalf("SeedRanks(%d) len=%d", n, len(ranks))
		}
		for i := 0; i < n; i++ {
			if strings.Trim(ranks[i], "a") == "" {
				t.Fatalf("SeedRanks(%d) produced all-'a' floor key %q at %d", n, ranks[i], i)
			}
			if i > 0 && !(ranks[i-1] < ranks[i]) {
				t.Fatalf("SeedRanks(%d) not increasing at %d: %q !< %q", n, i, ranks[i-1], ranks[i])
			}
		}
	}
}
