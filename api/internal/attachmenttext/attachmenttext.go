package attachmenttext

import (
	"regexp"

	"github.com/bitmaster-sk/rurdesk/api/internal/markdowntext"
	"github.com/google/uuid"
)

var linkPattern = regexp.MustCompile(`!?\[([^\]\n]*)\]\(attachment:([0-9a-fA-F-]{36})\)`)

func ParseIds(body string) []uuid.UUID {
	var ids []uuid.UUID
	seen := make(map[uuid.UUID]bool)

	for _, part := range markdowntext.OutsideCode(body) {
		for _, match := range linkPattern.FindAllStringSubmatch(part, -1) {
			idAttachment, err := uuid.Parse(match[2])
			if err != nil || seen[idAttachment] {
				continue
			}
			seen[idAttachment] = true
			ids = append(ids, idAttachment)
		}
	}
	return ids
}

func StripLinks(text string) string {
	return markdowntext.ReplaceOutsideCode(text, func(part string) string {
		return linkPattern.ReplaceAllString(part, "[$1]")
	})
}
