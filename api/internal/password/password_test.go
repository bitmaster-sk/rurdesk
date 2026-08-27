package password

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestCost(t *testing.T) {
	tests := []struct {
		name     string
		env      any
		expected int
	}{
		{name: "unset falls back to the bcrypt default", env: nil, expected: bcrypt.DefaultCost},
		{name: "below the bcrypt minimum falls back", env: 3, expected: bcrypt.DefaultCost},
		{name: "above the bcrypt maximum falls back", env: 32, expected: bcrypt.DefaultCost},
		{name: "unparseable value falls back", env: "cheap", expected: bcrypt.DefaultCost},
		{name: "valid value is honoured", env: bcrypt.MinCost, expected: bcrypt.MinCost},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			viper.Set("BCRYPT_COST", test.env)
			t.Cleanup(func() { viper.Set("BCRYPT_COST", nil) })

			require.Equal(t, test.expected, Cost())
		})
	}
}

func TestHashUsesTheConfiguredCost(t *testing.T) {
	viper.Set("BCRYPT_COST", MinCost)
	t.Cleanup(func() { viper.Set("BCRYPT_COST", nil) })

	hash, err := Hash("correct-horse")
	require.NoError(t, err)

	cost, err := bcrypt.Cost([]byte(hash))
	require.NoError(t, err)
	require.Equal(t, MinCost, cost)
}

func TestCompare(t *testing.T) {
	viper.Set("BCRYPT_COST", MinCost)
	t.Cleanup(func() { viper.Set("BCRYPT_COST", nil) })

	hash, err := Hash("correct-horse")
	require.NoError(t, err)

	require.NoError(t, Compare(hash, "correct-horse"))
	require.Error(t, Compare(hash, "wrong"))
	require.Error(t, Compare("not-a-bcrypt-hash", "correct-horse"))
}

func TestCompareAcceptsAHashMadeAtAnotherCost(t *testing.T) {
	viper.Set("BCRYPT_COST", 6)
	t.Cleanup(func() { viper.Set("BCRYPT_COST", nil) })
	hash, err := Hash("correct-horse")
	require.NoError(t, err)

	viper.Set("BCRYPT_COST", MinCost)

	require.NoError(t, Compare(hash, "correct-horse"),
		"stored hashes must keep verifying after the cost changes")
}

func TestDummyCarriesTheConfiguredCost(t *testing.T) {
	viper.Set("BCRYPT_COST", MinCost)
	t.Cleanup(func() { viper.Set("BCRYPT_COST", nil) })

	cost, err := bcrypt.Cost([]byte(Dummy()))
	require.NoError(t, err)
	require.Equal(t, MinCost, cost, "an unknown-email login must do the same work as a real one")

	viper.Set("BCRYPT_COST", 5)
	cost, err = bcrypt.Cost([]byte(Dummy()))
	require.NoError(t, err)
	require.Equal(t, 5, cost, "the dummy hash must follow a cost change, not stay pinned to the first one")
}
