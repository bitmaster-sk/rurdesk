package email

import "strings"

func Normalize(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}
