package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The pre-run cache flush must never hit Redis DB 0: that is the dev cache,
// and flushing it deletes every dev user's session (instant logout). The DB
// index therefore has to be configured explicitly and must be non-zero.
func TestCacheDBFromEnvRequiresExplicitNonZeroDB(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "unset", value: "", wantErr: true},
		{name: "dev db zero", value: "0", wantErr: true},
		{name: "not a number", value: "abc", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "db one", value: "1", want: 1},
		{name: "db two", value: "2", want: 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TEST_CACHE_DB", tc.value)

			got, err := testCacheDBFromEnv()

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
