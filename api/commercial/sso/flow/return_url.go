package flow

import (
	"strings"
	"unicode"
)

const maxReturnUrlLength = 2000

func SafeReturnUrl(raw string) string {
	if raw == "" ||
		len(raw) > maxReturnUrlLength ||
		!strings.HasPrefix(raw, "/") ||
		strings.Contains(raw, "//") ||
		strings.Contains(raw, `\`) ||
		strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return "/"
	}
	return raw
}
