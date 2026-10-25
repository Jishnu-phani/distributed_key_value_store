package main

import (
	"sync"
)

type VersionedValue struct {
	Value     string      `json:"value"`
	Timestamp int64       `json:"timestamp"`
	Vclock    VectorClock `json:"vclock"`
}

type Store struct {
	mu   sync.RWMutex
	data map[string][]VersionedValue
}

func NewStore() *Store {
	return &Store{data: make(map[string][]VersionedValue)}
}

func (s *Store) Get(key string) ([]VersionedValue, bool) {
	s.mu.RLock()
	value, ok := s.data[key]
	s.mu.RUnlock()
	return value, ok
}

func (s *Store) Put(key string, value VersionedValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existingValues := s.data[key]

	s.data[key] = reconcile(existingValues, value)
}

func (s *Store) Delete(key string) {
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
}

func reconcile(existingValues []VersionedValue, value VersionedValue) []VersionedValue {
	var survivors []VersionedValue
	dominated := false

	for _, existing := range existingValues {
		switch value.Vclock.Compare(existing.Vclock) {
		case Before:
			survivors = append(survivors, existing)
			dominated = true
		case After:
		case Equal:
		case Concurrent:
			survivors = append(survivors, existing)
		}
	}

	if !dominated {
		survivors = append(survivors, value)
	}

	return survivors
}
