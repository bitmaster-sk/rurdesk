package skills

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

//go:embed builtin/*.md
var builtinFS embed.FS

type BuiltinSkill struct {
	Key           string
	Name          string
	Description   string
	Content       string
	DefaultStages []string
}

var catalog = mustLoad()

func Builtins() []BuiltinSkill {
	out := make([]BuiltinSkill, len(catalog))
	copy(out, catalog)
	return out
}

func BuiltinByKey(key string) (BuiltinSkill, bool) {
	for _, builtin := range catalog {
		if builtin.Key == key {
			return builtin, true
		}
	}
	return BuiltinSkill{}, false
}

func mustLoad() []BuiltinSkill {
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		panic(fmt.Sprintf("skills: reading embedded builtin dir: %v", err))
	}
	var out []BuiltinSkill
	for _, entry := range entries {
		raw, err := builtinFS.ReadFile(path.Join("builtin", entry.Name()))
		if err != nil {
			panic(fmt.Sprintf("skills: reading %s: %v", entry.Name(), err))
		}
		skill, err := parse(strings.TrimSuffix(entry.Name(), ".md"), string(raw))
		if err != nil {
			panic(fmt.Sprintf("skills: parsing %s: %v", entry.Name(), err))
		}
		out = append(out, skill)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Not a YAML parser: three known frontmatter keys, nothing else.
func parse(key, raw string) (BuiltinSkill, error) {
	const delim = "---"
	rest, found := strings.CutPrefix(raw, delim+"\n")
	if !found {
		return BuiltinSkill{}, fmt.Errorf("missing frontmatter open")
	}
	front, content, found := strings.Cut(rest, "\n"+delim+"\n")
	if !found {
		return BuiltinSkill{}, fmt.Errorf("missing frontmatter close")
	}
	skill := BuiltinSkill{Key: key, Content: strings.TrimSpace(content)}
	for _, line := range strings.Split(front, "\n") {
		fieldKey, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(fieldKey) {
		case "name":
			skill.Name = strings.TrimSpace(value)
		case "description":
			skill.Description = strings.TrimSpace(value)
		case "stages":
			stages, err := parseStages(value)
			if err != nil {
				return BuiltinSkill{}, err
			}
			skill.DefaultStages = stages
		}
	}
	if skill.Name == "" || skill.Description == "" || skill.Content == "" {
		return BuiltinSkill{}, fmt.Errorf("name, description and content are required")
	}
	return skill, nil
}

func parseStages(value string) ([]string, error) {
	var stages []string
	for _, part := range strings.Split(value, ",") {
		stage := strings.TrimSpace(part)
		if stage == "" {
			continue
		}
		if !constants.IsSkillStage(stage) {
			return nil, fmt.Errorf("unknown default stage %q", stage)
		}
		stages = append(stages, stage)
	}
	return stages, nil
}
