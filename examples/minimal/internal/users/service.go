package users

import (
	"context"
	"fmt"
	"minimal/internal/api"
	"sort"
	"sync"
)

// Store is a domain-specific seam: repositories and unit tests implement it directly.
type Store interface {
	List(context.Context, int, int) ([]Record, error)
	Get(context.Context, int64) (Record, error)
	Create(context.Context, Record) (Record, error)
	Update(context.Context, int64, Record) (Record, error)
	Delete(context.Context, int64) error
}
type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }
func (s *Service) List(ctx context.Context, limit, offset int) ([]Record, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, fmt.Errorf("invalid pagination")
	}
	return s.store.List(ctx, limit, offset)
}
func (s *Service) Get(ctx context.Context, id int64) (Record, error) { return s.store.Get(ctx, id) }
func (s *Service) Create(ctx context.Context, input Input) (Record, error) {
	if err := input.Validate(); err != nil {
		return Record{}, err
	}
	return s.store.Create(ctx, input.Record())
}
func (s *Service) Update(ctx context.Context, id int64, input Input) (Record, error) {
	if err := input.Validate(); err != nil {
		return Record{}, err
	}
	return s.store.Update(ctx, id, input.Record())
}
func (s *Service) Delete(ctx context.Context, id int64) error { return s.store.Delete(ctx, id) }

// MemoryStore is bounded demonstration storage; data resets when the process stops.
type MemoryStore struct {
	mu      sync.RWMutex
	next    int64
	records map[int64]Record
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{records: map[int64]Record{}} }
func (s *MemoryStore) List(ctx context.Context, limit, offset int) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]int64, 0, len(s.records))
	for id := range s.records {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := []Record{}
	for i := offset; i < len(ids) && len(out) < limit; i++ {
		out = append(out, s.records[ids[i]])
	}
	return out, nil
}
func (s *MemoryStore) Get(ctx context.Context, id int64) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.records[id]
	if !ok {
		return v, api.ErrNotFound
	}
	return v, nil
}
func (s *MemoryStore) Create(ctx context.Context, v Record) (Record, error) {
	if err := ctx.Err(); err != nil {
		return v, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.records) >= 10000 {
		return v, fmt.Errorf("example memory store capacity exceeded")
	}
	s.next++
	v.ID = s.next
	s.records[v.ID] = v
	return v, nil
}
func (s *MemoryStore) Update(ctx context.Context, id int64, v Record) (Record, error) {
	if err := ctx.Err(); err != nil {
		return v, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[id]; !ok {
		return v, api.ErrNotFound
	}
	v.ID = id
	s.records[id] = v
	return v, nil
}
func (s *MemoryStore) Delete(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[id]; !ok {
		return api.ErrNotFound
	}
	delete(s.records, id)
	return nil
}
