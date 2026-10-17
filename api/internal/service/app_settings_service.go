package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

// appSettingsStorer is the persistence contract the service depends on (small interface — DIP).
type appSettingsStorer interface {
	LoadAll(ctx context.Context) (map[string]string, error)
	Upsert(ctx context.Context, values map[string]string) error
}

// snapshot is an immutable resolved view, swapped atomically on update.
type snapshot struct {
	numericValues map[string]int
	boolValues    map[string]bool
	stringValues  map[string]string
}

type AppSettingsService struct {
	store appSettingsStorer
	snap  atomic.Pointer[snapshot]
}

func NewAppSettingsService(store appSettingsStorer) *AppSettingsService {
	return &AppSettingsService{store: store}
}

// Load hydrates the in-memory snapshot from the store. Call once at boot.
func (s *AppSettingsService) Load(ctx context.Context) error {
	raw, err := s.store.LoadAll(ctx)
	if err != nil {
		return err
	}
	numericValues := make(map[string]int, len(constants.KnownAppNumericSettings))
	for key, spec := range constants.KnownAppNumericSettings {
		numericValues[key] = spec.Default
		if v, ok := raw[key]; ok {
			if n, err := strconv.Atoi(v); err == nil && n >= spec.Min && n <= spec.Max {
				numericValues[key] = n
			}
		}
	}
	boolValues := make(map[string]bool, len(constants.KnownAppBoolSettings))
	for key, defaultValue := range constants.KnownAppBoolSettings {
		boolValues[key] = defaultValue
		if v, ok := raw[key]; ok {
			if isEnabled, err := strconv.ParseBool(v); err == nil {
				boolValues[key] = isEnabled
			}
		}
	}
	stringValues := make(map[string]string, len(constants.KnownAppStringSettings))
	for key, defaultValue := range constants.KnownAppStringSettings {
		stringValues[key] = defaultValue
		if v, ok := raw[key]; ok {
			// LoadAll returns value::text — raw JSON — so a string arrives with
			// its quotes; keep the default when it does not parse as JSON.
			var parsed string
			if err := json.Unmarshal([]byte(v), &parsed); err == nil {
				stringValues[key] = parsed
			}
		}
	}
	s.snap.Store(&snapshot{numericValues: numericValues, boolValues: boolValues, stringValues: stringValues})
	return nil
}

func (s *AppSettingsService) getNumeric(key string) int {
	snap := s.snap.Load()
	if snap == nil {
		return constants.KnownAppNumericSettings[key].Default
	}
	return snap.numericValues[key]
}

func (s *AppSettingsService) getBool(key string) bool {
	snap := s.snap.Load()
	if snap == nil {
		return constants.KnownAppBoolSettings[key]
	}
	return snap.boolValues[key]
}

func (s *AppSettingsService) getString(key string) string {
	snap := s.snap.Load()
	if snap == nil {
		return constants.KnownAppStringSettings[key]
	}
	return snap.stringValues[key]
}

func (s *AppSettingsService) TablePageSize() int {
	return s.getNumeric(constants.SettingTablePageSize)
}

func (s *AppSettingsService) KanbanPageSize() int {
	return s.getNumeric(constants.SettingKanbanPageSize)
}

func (s *AppSettingsService) GanttBacklogPageSize() int {
	return s.getNumeric(constants.SettingGanttBacklogPageSize)
}

func (s *AppSettingsService) SprintVelocityLimit() int {
	return s.getNumeric(constants.SettingSprintVelocityLimit)
}

func (s *AppSettingsService) UserApiKeyLimit() int {
	return s.getNumeric(constants.SettingUserApiKeyLimit)
}

func (s *AppSettingsService) AttachmentMaxSizeMb() int {
	return s.getNumeric(constants.SettingAttachmentMaxSizeMb)
}

func (s *AppSettingsService) IsAgentThinkingPersisted() bool {
	return s.getBool(constants.SettingIsAgentThinkingPersisted)
}

func (s *AppSettingsService) AgentThinkingMaxKb() int {
	return s.getNumeric(constants.SettingAgentThinkingMaxKb)
}

func (s *AppSettingsService) PlantUmlServerUrl() string {
	return s.getString(constants.SettingPlantUmlServerUrl)
}

// Update validates every key, persists, then reloads the snapshot.
func (s *AppSettingsService) Update(ctx context.Context, numericChanges map[string]int, boolChanges map[string]bool, stringChanges map[string]string) error {
	persist := make(map[string]string, len(numericChanges)+len(boolChanges)+len(stringChanges))
	for key, n := range numericChanges {
		spec, ok := constants.KnownAppNumericSettings[key]
		if !ok {
			return fmt.Errorf("unknown setting %q", key)
		}
		if n < spec.Min || n > spec.Max {
			return fmt.Errorf("setting %q out of range [%d,%d]: %d", key, spec.Min, spec.Max, n)
		}
		persist[key] = strconv.Itoa(n)
	}
	for key, isEnabled := range boolChanges {
		if _, ok := constants.KnownAppBoolSettings[key]; !ok {
			return fmt.Errorf("unknown setting %q", key)
		}
		persist[key] = strconv.FormatBool(isEnabled)
	}
	for key, value := range stringChanges {
		if _, ok := constants.KnownAppStringSettings[key]; !ok {
			return fmt.Errorf("unknown setting %q", key)
		}
		if key == constants.SettingPlantUmlServerUrl {
			normalized, err := normalizePlantUmlServerUrl(value)
			if err != nil {
				return err
			}
			value = normalized
		}
		// Upsert casts to jsonb — persisting the bare string would break the cast.
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("encoding setting %q: %w", key, err)
		}
		persist[key] = string(encoded)
	}
	if err := s.store.Upsert(ctx, persist); err != nil {
		return err
	}
	return s.Load(ctx)
}

func normalizePlantUmlServerUrl(value string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > 500 {
		return "", fmt.Errorf("setting %q exceeds 500 characters", constants.SettingPlantUmlServerUrl)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("setting %q must be an http(s) URL with a host", constants.SettingPlantUmlServerUrl)
	}
	return trimmed, nil
}
