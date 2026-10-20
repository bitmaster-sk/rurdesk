package common

import (
	"fmt"
	"strings"
)

type Model struct {
	Provider string
	Name     string
}

func (m Model) Id() string {
	if m.Provider == "" {
		return m.Name
	}
	return m.Provider + "/" + m.Name
}

type ModelCatalog struct {
	Models  []Model
	Default Model
}

func parseModelCatalog(adapterType, rawModels, rawDefault string) (ModelCatalog, error) {
	var catalog ModelCatalog
	seen := map[string]bool{}
	for _, entry := range strings.Split(rawModels, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		model, err := parseModel(adapterType, entry)
		if err != nil {
			return ModelCatalog{}, err
		}
		if seen[model.Id()] {
			return ModelCatalog{}, fmt.Errorf("MODELS lists %q twice", entry)
		}
		seen[model.Id()] = true
		catalog.Models = append(catalog.Models, model)
	}
	if len(catalog.Models) == 0 {
		return ModelCatalog{}, fmt.Errorf("MODELS must list at least one model")
	}

	catalog.Default = catalog.Models[0]
	if rawDefault = strings.TrimSpace(rawDefault); rawDefault != "" {
		model, ok := catalog.find(rawDefault)
		if !ok {
			return ModelCatalog{}, fmt.Errorf("MODEL_DEFAULT %q is not listed in MODELS", rawDefault)
		}
		catalog.Default = model
	}
	return catalog, nil
}

func parseModel(adapterType, entry string) (Model, error) {
	if adapterType != "goose" {
		return Model{Name: entry}, nil
	}
	provider, name, ok := strings.Cut(entry, "/")
	if !ok || provider == "" || name == "" {
		return Model{}, fmt.Errorf("MODELS entry %q must be provider/model", entry)
	}
	return Model{Provider: provider, Name: name}, nil
}

func (c ModelCatalog) find(id string) (Model, bool) {
	for _, model := range c.Models {
		if model.Id() == id {
			return model, true
		}
	}
	return Model{}, false
}

func (c ModelCatalog) Ids() []string {
	ids := make([]string, len(c.Models))
	for i, model := range c.Models {
		ids[i] = model.Id()
	}
	return ids
}

// Resolve never falls back to the default for an unknown id: the tracker
// owns that fallback and records it on the run.
func (c ModelCatalog) Resolve(id string) (Model, error) {
	if id == "" {
		return c.Default, nil
	}
	if model, ok := c.find(id); ok {
		return model, nil
	}
	return Model{}, &AgentError{
		Code:   ErrCodeModelNotAllowed,
		Detail: fmt.Sprintf("model %q is not allowed on this gateway (allowed: %s)", id, strings.Join(c.Ids(), ", ")),
	}
}
