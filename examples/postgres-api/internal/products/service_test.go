package products

import (
	"context"
	"errors"
	"postgres-api/internal/api"
	"sync"
	"testing"
)

func sampleInput() Input {
	value0 := string("example")
	value1 := float64(1.5)
	value2 := bool(true)
	return Input{Name: &value0, Price: &value1, Active: &value2}
}
func zeroInput() Input {
	value0 := string("updated")
	value1 := float64(0)
	value2 := bool(false)
	return Input{Name: &value0, Price: &value1, Active: &value2}
}
func TestServiceCRUD(t *testing.T) {
	ctx := context.Background()
	s := NewService(NewMemoryStore())
	input := sampleInput()
	v, err := s.Create(ctx, input)
	if err != nil || v.ID < 1 {
		t.Fatalf("create: %v %v", v, err)
	}
	records, err := s.List(ctx, 20, 0)
	if err != nil || len(records) != 1 {
		t.Fatalf("list: %v %v", records, err)
	}
	got, err := s.Get(ctx, v.ID)
	if err != nil || got != v {
		t.Fatalf("get: %v %v", got, err)
	}
	got, err = s.Update(ctx, v.ID, zeroInput())
	if err != nil {
		t.Fatal(err)
	}
	expected := zeroInput().Record()
	expected.ID = v.ID
	if got != expected {
		t.Fatalf("zero-value update: got %v expected %v", got, expected)
	}
	if err = s.Delete(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(ctx, v.ID); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if err = s.Delete(ctx, v.ID); !errors.Is(err, api.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, Input{}); err == nil {
		t.Fatal("accepted missing fields")
	}
	if _, err = s.List(ctx, 101, 0); err == nil {
		t.Fatal("accepted invalid pagination")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.List(canceled, 20, 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

type failingStore struct{ *MemoryStore }

var failure = errors.New("storage unavailable")

func (failingStore) Create(context.Context, Record) (Record, error)   { return Record{}, failure }
func (failingStore) List(context.Context, int, int) ([]Record, error) { return nil, failure }
func TestStorageErrors(t *testing.T) {
	s := NewService(failingStore{NewMemoryStore()})
	if _, err := s.Create(context.Background(), sampleInput()); !errors.Is(err, failure) {
		t.Fatal("lost create error", err)
	}
	if _, err := s.List(context.Background(), 20, 0); !errors.Is(err, failure) {
		t.Fatal("lost list error", err)
	}
}

func TestConcurrentMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	var workers sync.WaitGroup
	for i := 0; i < 50; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if _, err := store.Create(context.Background(), sampleInput().Record()); err != nil {
				t.Error(err)
			}
		}()
	}
	workers.Wait()
	records, err := store.List(context.Background(), 100, 0)
	if err != nil || len(records) != 50 {
		t.Fatalf("concurrent creates: %d %v", len(records), err)
	}
}
