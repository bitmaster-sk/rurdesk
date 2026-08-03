package urlutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParsePositiveDuration(t *testing.T) {
	const day = 24 * time.Hour

	tests := []struct {
		name  string
		raw   string
		want  time.Duration
		isErr bool
	}{
		{name: "days", raw: "30d", want: 30 * day},
		{name: "one day is exactly 24h", raw: "1d", want: day},
		{name: "fractional days", raw: "1.5d", want: 36 * time.Hour},

		{name: "days hours minutes", raw: "1d8h6m", want: day + 8*time.Hour + 6*time.Minute},
		{name: "ascending order still sums", raw: "6m8h1d", want: day + 8*time.Hour + 6*time.Minute},
		{name: "repeated unit sums", raw: "12h12h", want: day},

		{name: "hours", raw: "2h", want: 2 * time.Hour},
		{name: "minutes", raw: "15m", want: 15 * time.Minute},
		{name: "seconds", raw: "90s", want: 90 * time.Second},
		{name: "fractional hours", raw: "1.5h", want: 90 * time.Minute},
		{name: "milliseconds", raw: "500ms", want: 500 * time.Millisecond},

		{name: "surrounding whitespace", raw: "  30d  ", want: 30 * day},

		// rejected
		{name: "empty", raw: "", isErr: true},
		{name: "whitespace only", raw: "   ", isErr: true},
		{name: "zero", raw: "0d", isErr: true},
		{name: "negative", raw: "-30m", isErr: true},
		{name: "unknown unit", raw: "30x", isErr: true},
		{name: "bare number", raw: "30", isErr: true},
		{name: "not a number", raw: "abc", isErr: true},
		{name: "calendar units are not supported", raw: "1M", isErr: true},
		{name: "weeks are not supported", raw: "2w", isErr: true},
		{name: "overflow", raw: "100000000000d", isErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParsePositiveDuration(tc.raw)
			if tc.isErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
