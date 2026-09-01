package service

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

type fakeStore struct {
	data     map[string]string
	loads    int
	upserted map[string]string
}

func (f *fakeStore) LoadAll(ctx context.Context) (map[string]string, error) {
	f.loads++
	return f.data, nil
}

func (f *fakeStore) Upsert(ctx context.Context, values map[string]string) error {
	f.upserted = values
	for k, v := range values {
		f.data[k] = v
	}
	return nil
}

func TestAppSettings_DefaultWhenKeyAbsent(t *testing.T) {
	svc := NewAppSettingsService(&fakeStore{data: map[string]string{}})
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := constants.KnownAppNumericSettings[constants.SettingTablePageSize].Default
	if got := svc.TablePageSize(); got != want {
		t.Fatalf("want default %d, got %d", want, got)
	}
}

func TestAppSettings_ReadsFromMemoryNotStore(t *testing.T) {
	store := &fakeStore{data: map[string]string{constants.SettingTablePageSize: "75"}}
	svc := NewAppSettingsService(store)
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	loadsAfterBoot := store.loads
	for i := 0; i < 100; i++ {
		_ = svc.TablePageSize()
	}
	if store.loads != loadsAfterBoot {
		t.Fatalf("getter hit the store: loads went %d -> %d", loadsAfterBoot, store.loads)
	}
	if svc.TablePageSize() != 75 {
		t.Fatalf("want 75 from store, got %d", svc.TablePageSize())
	}
}

func TestAppSettings_UpdateValidatesAndSwaps(t *testing.T) {
	svc := NewAppSettingsService(&fakeStore{data: map[string]string{}})
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := svc.Update(context.Background(), map[string]int{constants.SettingTablePageSize: 9999}, nil); err == nil {
		t.Fatal("expected out-of-range error")
	}
	if err := svc.Update(context.Background(), map[string]int{constants.SettingTablePageSize: 100}, nil); err != nil {
		t.Fatal(err)
	}
	if svc.TablePageSize() != 100 {
		t.Fatalf("snapshot not swapped, got %d", svc.TablePageSize())
	}
}

func TestAppSettings_UserApiKeyLimit(t *testing.T) {
	svc := NewAppSettingsService(&fakeStore{data: map[string]string{}})
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := svc.UserApiKeyLimit(); got != 10 {
		t.Fatalf("want default 10, got %d", got)
	}

	if err := svc.Update(context.Background(), map[string]int{constants.SettingUserApiKeyLimit: 25}, nil); err != nil {
		t.Fatal(err)
	}
	if got := svc.UserApiKeyLimit(); got != 25 {
		t.Fatalf("want 25 after update, got %d", got)
	}

	if err := svc.Update(context.Background(), map[string]int{constants.SettingUserApiKeyLimit: 0}, nil); err == nil {
		t.Fatal("expected out-of-range error for 0")
	}
	if err := svc.Update(context.Background(), map[string]int{constants.SettingUserApiKeyLimit: 101}, nil); err == nil {
		t.Fatal("expected out-of-range error for 101")
	}
}

// A boolean setting round-trips as a JSON boolean, so the value in the column
// reads the way an admin would expect and no layer translates 1 into true.
func TestAppSettings_BoolSettingRoundTripsAsBoolean(t *testing.T) {
	store := &fakeStore{data: map[string]string{}}
	svc := NewAppSettingsService(store)
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !svc.IsAgentThinkingPersisted() {
		t.Fatal("want the setting on by default")
	}

	boolChanges := map[string]bool{constants.SettingIsAgentThinkingPersisted: false}
	if err := svc.Update(context.Background(), nil, boolChanges); err != nil {
		t.Fatal(err)
	}

	if got := store.upserted[constants.SettingIsAgentThinkingPersisted]; got != "false" {
		t.Fatalf("want %q persisted, got %q", "false", got)
	}
	if svc.IsAgentThinkingPersisted() {
		t.Fatal("want the setting off after update")
	}
}

func TestAppSettings_BoolSettingFallsBackToDefaultWhenStoredValueIsGarbage(t *testing.T) {
	store := &fakeStore{data: map[string]string{constants.SettingIsAgentThinkingPersisted: "\"maybe\""}}
	svc := NewAppSettingsService(store)
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !svc.IsAgentThinkingPersisted() {
		t.Fatal("want the default when the stored value does not parse")
	}
}

func TestAppSettings_UpdateRejectsUnknownBoolSetting(t *testing.T) {
	svc := NewAppSettingsService(&fakeStore{data: map[string]string{}})
	if err := svc.Update(context.Background(), nil, map[string]bool{"agent.bogus": true}); err == nil {
		t.Fatal("expected unknown-key error")
	}
}

func TestAppSettings_UpdateRejectsUnknownKey(t *testing.T) {
	svc := NewAppSettingsService(&fakeStore{data: map[string]string{}})
	if err := svc.Update(context.Background(), map[string]int{"pagination.bogus": 10}, nil); err == nil {
		t.Fatal("expected unknown-key error")
	}
}
