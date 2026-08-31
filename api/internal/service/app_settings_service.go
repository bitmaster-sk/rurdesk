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
	values map[string]int
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
	values := make(map[string]int, len(constants.KnownAppSettings))
	for key, spec := range constants.KnownAppSettings {
		values[key] = spec.Default
		if v, ok := raw[key]; ok {
			if n, err := strconv.Atoi(v); err == nil && n >= spec.Min && n <= spec.Max {
				values[key] = n
			}
		}
	}
	s.snap.Store(&snapshot{values: values})
	return nil
}

func (s *AppSettingsService) get(key string) int {
	snap := s.snap.Load()
	if snap == nil {
		return constants.KnownAppSettings[key].Default
	}
	return snap.values[key]
}

func (s *AppSettingsService) TablePageSize() int {
	return s.get(constants.SettingTablePageSize)
}

func (s *AppSettingsService) KanbanPageSize() int {
	return s.get(constants.SettingKanbanPageSize)
}

func (s *AppSettingsService) GanttBacklogPageSize() int {
	return s.get(constants.SettingGanttBacklogPageSize)
}

func (s *AppSettingsService) SprintVelocityLimit() int {
	return s.get(constants.SettingSprintVelocityLimit)
}

func (s *AppSettingsService) UserApiKeyLimit() int {
	return s.get(constants.SettingUserApiKeyLimit)
}

// Update validates every key, persists, then reloads the snapshot.
func (s *AppSettingsService) Update(ctx context.Context, changes map[string]int) error {
	persist := make(map[string]string, len(changes))
	for key, n := range changes {
		spec, ok := constants.KnownAppSettings[key]
		if !ok {
			return fmt.Errorf("unknown setting %q", key)
		}
		if n < spec.Min || n > spec.Max {
			return fmt.Errorf("setting %q out of range [%d,%d]: %d", key, spec.Min, spec.Max, n)
		}
		persist[key] = strconv.Itoa(n)
	}
	if err := s.store.Upsert(ctx, persist); err != nil {
		return err
	}
	return s.Load(ctx)
}
