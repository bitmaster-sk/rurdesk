package service

import (
	"context"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

type fakeStore struct {
	data     map[string]string
	loads    int
	upserted map[string]string
}

func (f *fakeStore) LoadAll(_ context.Context) (map[string]string, error) {
	f.loads++
	return f.data, nil
}

func (f *fakeStore) Upsert(_ context.Context, values map[string]string) error {
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

	if err := svc.Update(context.Background(), map[string]int{constants.SettingTablePageSize: 9999}, nil, nil); err == nil {
		t.Fatal("expected out-of-range error")
	}
	if err := svc.Update(context.Background(), map[string]int{constants.SettingTablePageSize: 100}, nil, nil); err != nil {
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

	if err := svc.Update(context.Background(), map[string]int{constants.SettingUserApiKeyLimit: 25}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := svc.UserApiKeyLimit(); got != 25 {
		t.Fatalf("want 25 after update, got %d", got)
	}

	if err := svc.Update(context.Background(), map[string]int{constants.SettingUserApiKeyLimit: 0}, nil, nil); err == nil {
		t.Fatal("expected out-of-range error for 0")
	}
	if err := svc.Update(context.Background(), map[string]int{constants.SettingUserApiKeyLimit: 101}, nil, nil); err == nil {
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
	if err := svc.Update(context.Background(), nil, boolChanges, nil); err != nil {
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

func TestAppSettings_AttachmentMaxSizeMb(t *testing.T) {
	svc := NewAppSettingsService(&fakeStore{data: map[string]string{}})
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := svc.AttachmentMaxSizeMb(); got != 25 {
		t.Fatalf("want default 25, got %d", got)
	}

	if err := svc.Update(context.Background(), map[string]int{constants.SettingAttachmentMaxSizeMb: 40}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := svc.AttachmentMaxSizeMb(); got != 40 {
		t.Fatalf("want 40 after update, got %d", got)
	}

	if err := svc.Update(context.Background(), map[string]int{constants.SettingAttachmentMaxSizeMb: 0}, nil, nil); err == nil {
		t.Fatal("expected out-of-range error for 0")
	}
	if err := svc.Update(context.Background(), map[string]int{constants.SettingAttachmentMaxSizeMb: 101}, nil, nil); err == nil {
		t.Fatal("expected out-of-range error for 101")
	}
}

func TestAppSettings_UpdateRejectsUnknownBoolSetting(t *testing.T) {
	svc := NewAppSettingsService(&fakeStore{data: map[string]string{}})
	if err := svc.Update(context.Background(), nil, map[string]bool{"agent.bogus": true}, nil); err == nil {
		t.Fatal("expected unknown-key error")
	}
}

func TestAppSettings_UpdateRejectsUnknownKey(t *testing.T) {
	svc := NewAppSettingsService(&fakeStore{data: map[string]string{}})
	if err := svc.Update(context.Background(), map[string]int{"pagination.bogus": 10}, nil, nil); err == nil {
		t.Fatal("expected unknown-key error")
	}
}

// A string setting round-trips as a JSON string: the repository hands raw JSON
// (value::text) both ways, so the service must unquote on load and quote on
// save — otherwise the jsonb upsert breaks and the getter returns the quotes.
func TestAppSettings_PlantUmlServerUrlRoundTrips(t *testing.T) {
	store := &fakeStore{data: map[string]string{}}
	svc := NewAppSettingsService(store)
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := svc.PlantUmlServerUrl(); got != "" {
		t.Fatalf("want empty default, got %q", got)
	}
	changes := map[string]string{constants.SettingPlantUmlServerUrl: " http://render.local:8080/ "}
	if err := svc.Update(context.Background(), nil, nil, changes); err != nil {
		t.Fatal(err)
	}
	if got := store.upserted[constants.SettingPlantUmlServerUrl]; got != `"http://render.local:8080"` {
		t.Fatalf("want json-quoted trimmed url persisted, got %q", got)
	}
	if got := svc.PlantUmlServerUrl(); got != "http://render.local:8080" {
		t.Fatalf("want unquoted url after reload, got %q", got)
	}
}

func TestAppSettings_PlantUmlServerUrlFallsBackToDefaultWhenStoredValueIsGarbage(t *testing.T) {
	store := &fakeStore{data: map[string]string{constants.SettingPlantUmlServerUrl: "not-json{"}}
	svc := NewAppSettingsService(store)
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := svc.PlantUmlServerUrl(); got != "" {
		t.Fatalf("want default when the stored value does not parse as JSON, got %q", got)
	}
}

func TestAppSettings_PlantUmlServerUrlValidation(t *testing.T) {
	svc := NewAppSettingsService(&fakeStore{data: map[string]string{}})
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"ftp://x", "nieco", "http://", strings.Repeat("https://a", 56)} {
		changes := map[string]string{constants.SettingPlantUmlServerUrl: bad}
		if err := svc.Update(context.Background(), nil, nil, changes); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
	if err := svc.Update(context.Background(), nil, nil, map[string]string{constants.SettingPlantUmlServerUrl: ""}); err != nil {
		t.Fatalf("empty value must be allowed (plantuml off): %v", err)
	}
}

func TestAppSettings_UpdateRejectsUnknownStringSetting(t *testing.T) {
	svc := NewAppSettingsService(&fakeStore{data: map[string]string{}})
	if err := svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svc.Update(context.Background(), nil, nil, map[string]string{"plantuml.bogus": "x"}); err == nil {
		t.Fatal("expected unknown-key error")
	}
}
