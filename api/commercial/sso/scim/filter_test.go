package scim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFilter(t *testing.T) {
	tests := []struct {
		raw      string
		expected *Filter
	}{
		{raw: "", expected: nil},
		{raw: `userName eq "Jana@Acme.test"`, expected: &Filter{Attribute: AttributeUserName, Value: "Jana@Acme.test"}},
		{raw: `USERNAME EQ "jana@acme.test"`, expected: &Filter{Attribute: AttributeUserName, Value: "jana@acme.test"}},
		{raw: `externalId eq "a \"quoted\" id"`, expected: &Filter{Attribute: AttributeExternalId, Value: `a "quoted" id`}},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			filter, err := ParseFilter(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, filter)
		})
	}
}

func TestParseFilter_RejectsWhatItCannotAnswer(t *testing.T) {
	for _, raw := range []string{
		`userName co "jana"`,
		`displayName eq "Jana"`,
		`userName eq jana`,
		`userName eq "jana" and active eq true`,
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := ParseFilter(raw)
			var scimErr *Error
			require.ErrorAs(t, err, &scimErr)
			assert.Equal(t, "invalidFilter", scimErr.ScimType)
		})
	}
}

func TestParsePage(t *testing.T) {
	assert.Equal(t, Page{StartIndex: 1, Count: 100}, ParsePage("", ""))
	assert.Equal(t, Page{StartIndex: 11, Count: 5}, ParsePage("11", "5"))
	assert.Equal(t, Page{StartIndex: 1, Count: 0}, ParsePage("0", "0"))
	assert.Equal(t, Page{StartIndex: 1, Count: 100}, ParsePage("-3", "1000"))
}
