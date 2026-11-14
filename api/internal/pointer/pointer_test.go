package pointer

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEqual(t *testing.T) {
	one, otherOne, two := 1, 1, 2

	tests := []struct {
		name     string
		a, b     *int
		expected bool
	}{
		{name: "both nil", a: nil, b: nil, expected: true},
		{name: "nil and value", a: nil, b: &one, expected: false},
		{name: "value and nil", a: &one, b: nil, expected: false},
		{name: "same value at different addresses", a: &one, b: &otherOne, expected: true},
		{name: "different values", a: &one, b: &two, expected: false},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.expected, Equal(testCase.a, testCase.b))
		})
	}
}

func TestValue(t *testing.T) {
	five := int64(5)

	require.Equal(t, int64(5), Value(&five))
	require.Nil(t, Value[int64](nil))

	encoded, err := json.Marshal(Value[int64](nil))
	require.NoError(t, err)
	require.JSONEq(t, `null`, string(encoded))
}
