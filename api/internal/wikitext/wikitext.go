package wikitext

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const MaxSlugLength = 120

var sharedPrefixes = []string{"shared:", "spolocne:"}

var linkPattern = regexp.MustCompile(`\[\[([^\[\]\n]+?)\]\]`)

var fencePattern = regexp.MustCompile("(?s)```.*?```|`[^`\n]*`")

type Link struct {
	Shared bool
	Slug   string
}

func Slugify(text string) string {
	stripped, _, err := transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC), text)
	if err != nil {
		stripped = text
	}
	var builder strings.Builder
	pendingDash := false
	for _, r := range strings.ToLower(stripped) {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if !isAlnum {
			pendingDash = builder.Len() > 0
			continue
		}
		if pendingDash {
			builder.WriteByte('-')
			pendingDash = false
		}
		builder.WriteRune(r)
	}
	slug := builder.String()
	if len(slug) > MaxSlugLength {
		slug = strings.TrimRight(slug[:MaxSlugLength], "-")
	}
	return slug
}

func ParseLinks(body string) []Link {
	withoutCode := fencePattern.ReplaceAllString(body, "")
	seen := map[Link]bool{}
	links := make([]Link, 0)
	for _, match := range linkPattern.FindAllStringSubmatch(withoutCode, -1) {
		link, ok := ParseTarget(match[1])
		if !ok || seen[link] {
			continue
		}
		seen[link] = true
		links = append(links, link)
	}
	return links
}

func ParseTarget(raw string) (Link, bool) {
	target, _, _ := strings.Cut(raw, "|")
	target, _, _ = strings.Cut(target, "#")
	target = strings.TrimSpace(target)
	shared := false
	lowered := strings.ToLower(target)
	for _, prefix := range sharedPrefixes {
		if strings.HasPrefix(lowered, prefix) {
			shared = true
			target = target[len(prefix):]
			break
		}
	}
	slug := Slugify(target)
	if slug == "" {
		return Link{}, false
	}
	return Link{Shared: shared, Slug: slug}, true
}

func ToPrefixQuery(text string) string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	terms := make([]string, len(words))
	for i, word := range words {
		terms[i] = word + ":*"
	}
	return strings.Join(terms, " & ")
}

func EstimateTokens(text string) int {
	chars := len([]rune(text))
	return (chars + 3) / 4
}
