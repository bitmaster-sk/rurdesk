package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// A builtin whose live fields still hash to the checksum stored on it counts as
// untouched, which is what lets the startup sync update it in place.
func Checksum(name, description, content string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{name, description, content}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func (skill BuiltinSkill) Checksum() string {
	return Checksum(skill.Name, skill.Description, skill.Content)
}
