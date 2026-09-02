package service

import (
	"context"
	"fmt"
	"strconv"
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
	s.snap.Store(&snapshot{numericValues: numericValues, boolValues: boolValues})
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

func (s *AppSettingsService) IsAgentThinkingPersisted() bool {
	return s.getBool(constants.SettingIsAgentThinkingPersisted)
}

func (s *AppSettingsService) AgentThinkingMaxKb() int {
	return s.getNumeric(constants.SettingAgentThinkingMaxKb)
}

// Update validates every key, persists, then reloads the snapshot.
func (s *AppSettingsService) Update(ctx context.Context, numericChanges map[string]int, boolChanges map[string]bool) error {
	persist := make(map[string]string, len(numericChanges)+len(boolChanges))
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
	if err := s.store.Upsert(ctx, persist); err != nil {
		return err
	}
	return s.Load(ctx)
}
