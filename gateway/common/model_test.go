package common

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseModelCatalog_GooseSplitsProviderOnFirstSlash(t *testing.T) {
	catalog, err := parseModelCatalog("goose", "openai/kimi-k2.7-code, ollama/qwen3:8b", "")
	if err != nil {
		t.Fatalf("parseModelCatalog: %v", err)
	}
	want := []Model{
		{Provider: "openai", Name: "kimi-k2.7-code"},
		{Provider: "ollama", Name: "qwen3:8b"},
	}
	if !reflect.DeepEqual(catalog.Models, want) {
		t.Errorf("Models = %+v, want %+v", catalog.Models, want)
	}
	if catalog.Default != want[0] {
		t.Errorf("Default = %+v, want the first entry %+v", catalog.Default, want[0])
	}
}

func TestParseModelCatalog_GooseKeepsSlashesInsideTheModelName(t *testing.T) {
	catalog, err := parseModelCatalog("goose", "openrouter/moonshotai/kimi-k2", "")
	if err != nil {
		t.Fatalf("parseModelCatalog: %v", err)
	}
	if got := catalog.Models[0]; got != (Model{Provider: "openrouter", Name: "moonshotai/kimi-k2"}) {
		t.Errorf("Models[0] = %+v", got)
	}
}

func TestParseModelCatalog_ClaudeCodeTakesBareNames(t *testing.T) {
	catalog, err := parseModelCatalog("claude-code", "sonnet,opus", "opus")
	if err != nil {
		t.Fatalf("parseModelCatalog: %v", err)
	}
	if !reflect.DeepEqual(catalog.Ids(), []string{"sonnet", "opus"}) {
		t.Errorf("Ids = %v", catalog.Ids())
	}
	if catalog.Default.Id() != "opus" {
		t.Errorf("Default = %q, want opus", catalog.Default.Id())
	}
}

func TestParseModelCatalog_RejectsABadConfig(t *testing.T) {
	tests := []struct {
		name, adapter, models, modelDefault, wantInErr string
	}{
		{"empty list", "goose", " , ", "", "MODELS"},
		{"goose entry without provider", "goose", "kimi-k2", "", "kimi-k2"},
		{"goose entry with empty model", "goose", "openai/", "", "openai/"},
		{"duplicate entry", "claude-code", "sonnet,sonnet", "", "sonnet"},
		{"default outside the list", "goose", "openai/kimi", "openai/glm", "MODEL_DEFAULT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseModelCatalog(tt.adapter, tt.models, tt.modelDefault)
			if err == nil || !strings.Contains(err.Error(), tt.wantInErr) {
				t.Fatalf("err = %v, want it to mention %q", err, tt.wantInErr)
			}
		})
	}
}

func TestModelCatalogResolve(t *testing.T) {
	catalog, err := parseModelCatalog("goose", "openai/a,openai/b", "openai/a")
	if err != nil {
		t.Fatalf("parseModelCatalog: %v", err)
	}

	if got, err := catalog.Resolve("openai/b"); err != nil || got.Id() != "openai/b" {
		t.Errorf("Resolve(openai/b) = %+v, %v", got, err)
	}
	if got, err := catalog.Resolve(""); err != nil || got.Id() != "openai/a" {
		t.Errorf("Resolve(\"\") = %+v, %v, want the default", got, err)
	}

	_, err = catalog.Resolve("openai/c")
	var agentErr *AgentError
	if !errors.As(err, &agentErr) || agentErr.Code != ErrCodeModelNotAllowed {
		t.Fatalf("Resolve(openai/c) err = %v, want %s", err, ErrCodeModelNotAllowed)
	}
	for _, want := range []string{"openai/c", "openai/a", "openai/b"} {
		if !strings.Contains(agentErr.Detail, want) {
			t.Errorf("detail %q does not mention %q", agentErr.Detail, want)
		}
	}
}
