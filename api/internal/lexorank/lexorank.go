// Package lexorank produces short, orderable string keys for manual sorting.
// A new position is the lexicographic midpoint between its neighbours, so a
// single reorder writes a single row and never renumbers siblings.
package lexorank

const (
	loChar = byte('a')
	hiChar = byte('z')
)

// Between returns a key k with prev < k < next under byte-lexicographic order.
// prev == "" means "before every key"; next == "" means "after every key".
//
// It never returns the bare floor key ("a"), so there is always room to insert
// before the smallest key (keys descend as "an","ag","ad",…).
func Between(prev, next string) string {
	var out []byte
	for i := 0; ; i++ {
		da := loChar // exhausted/empty prev digit == the low bound
		if i < len(prev) {
			da = prev[i]
		}
		db := hiChar + 1 // exhausted next digit == just above the high bound
		if i < len(next) {
			db = next[i]
		}
		if da == db {
			out = append(out, da) // shared digit: carry and descend
			continue
		}
		mid := (da + db) / 2
		if mid == da {
			out = append(out, da) // adjacent: no digit between; descend a level
			continue
		}
		out = append(out, mid)
		return string(out)
	}
}

// SeedRanks returns n strictly increasing, evenly spread interior keys. Key width
// grows so n keys always fit strictly inside the range, for any n.
func SeedRanks(n int) []string {
	length, span := 2, 26*26
	for n > span-1 { // ensure span >= n+1 so v>=1 and no duplicates
		length++
		span *= 26
	}
	ranks := make([]string, n)
	for i := 0; i < n; i++ {
		v := (i + 1) * span / (n + 1)
		b := make([]byte, length)
		for j := length - 1; j >= 0; j-- {
			b[j] = loChar + byte(v%26)
			v /= 26
		}
		ranks[i] = string(b)
	}
	return ranks
}
