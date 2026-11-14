package email

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalize(t *testing.T) {
	assert.Equal(t, "jan.novak@firma.sk", Normalize("  Jan.Novak@Firma.SK "))
	assert.Equal(t, "", Normalize("   "))
}
