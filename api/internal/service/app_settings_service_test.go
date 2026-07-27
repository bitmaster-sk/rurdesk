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
	want := constants.KnownAppSettings[constants.SettingTablePageSize].Default
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

	if err := svc.Update(context.Background(), map[string]int{constants.SettingTablePageSize: 9999}); err == nil {
		t.Fatal("expected out-of-range error")
	}
	if err := svc.Update(context.Background(), map[string]int{constants.SettingTablePageSize: 100}); err != nil {
		t.Fatal(err)
	}
	if svc.TablePageSize() != 100 {
		t.Fatalf("snapshot not swapped, got %d", svc.TablePageSize())
	}
}

func TestAppSettings_UpdateRejectsUnknownKey(t *testing.T) {
	svc := NewAppSettingsService(&fakeStore{data: map[string]string{}})
	if err := svc.Update(context.Background(), map[string]int{"pagination.bogus": 10}); err == nil {
		t.Fatal("expected unknown-key error")
	}
}
