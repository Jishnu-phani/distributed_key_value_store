package main

import "sync"

type VersionedValue struct {
	Value     string `json:"value"`
	Timestamp int64  `json:"timestamp"`
}

type Store struct {
	mu   sync.RWMutex
	data map[string]VersionedValue
}

func NewStore() *Store {
	return &Store{data: make(map[string]VersionedValue)}
}

func (s *Store) Get(key string) (VersionedValue, bool) {
	s.mu.RLock()
	value, ok := s.data[key]
	s.mu.RUnlock()
	return value, ok
}

func (s *Store) Put(key string, value VersionedValue) {
	s.mu.Lock()
	s.data[key] = value
	s.mu.Unlock()
}

func (s *Store) Delete(key string) {
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
}