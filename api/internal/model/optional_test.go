package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOptionalUnmarshal_DistinguishesAbsentFromNull(t *testing.T) {
	type payload struct {
		Field Optional[string] `json:"field,omitzero"`
	}

	tests := []struct {
		name          string
		body          string
		wantIsDefined bool
		wantValue     *string
	}{
		{name: "absent key", body: `{}`, wantIsDefined: false, wantValue: nil},
		{name: "explicit null", body: `{"field":null}`, wantIsDefined: true, wantValue: nil},
		{name: "value", body: `{"field":"x"}`, wantIsDefined: true, wantValue: ptr("x")},
		{name: "empty string", body: `{"field":""}`, wantIsDefined: true, wantValue: ptr("")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got payload
			require.Nil(t, json.Unmarshal([]byte(tc.body), &got))
			require.Equal(t, tc.wantIsDefined, got.Field.IsDefined)
			require.Equal(t, tc.wantValue, got.Field.Value)
		})
	}
}

func TestOptionalMarshal_UndefinedIsOmittedNotNulled(t *testing.T) {
	type payload struct {
		Absent  Optional[string] `json:"absent,omitzero"`
		Cleared Optional[string] `json:"cleared,omitzero"`
		Set     Optional[string] `json:"set,omitzero"`
	}

	body, err := json.Marshal(payload{
		Cleared: NewOptionalNull[string](),
		Set:     NewOptional("x"),
	})

	require.Nil(t, err)
	require.JSONEq(t, `{"cleared":null,"set":"x"}`, string(body))
}

func TestOptionalOrElse(t *testing.T) {
	require.Equal(t, "current", Optional[string]{}.OrElse("current"))
	require.Equal(t, "", NewOptionalNull[string]().OrElse("current"))
	require.Equal(t, "new", NewOptional("new").OrElse("current"))
}

func TestOptionalPtrOrElse(t *testing.T) {
	current := ptr("current")

	require.Equal(t, current, Optional[string]{}.PtrOrElse(current))
	require.Nil(t, NewOptionalNull[string]().PtrOrElse(current))
	require.Equal(t, ptr("new"), NewOptional("new").PtrOrElse(current))
}

func ptr[T any](value T) *T { return &value }
