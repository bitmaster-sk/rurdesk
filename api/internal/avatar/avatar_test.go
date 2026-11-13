package avatar

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomColor_IsAHexColor(t *testing.T) {
	assert.Regexp(t, regexp.MustCompile(`^#[0-9a-f]{6}$`), RandomColor())
}
