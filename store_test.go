package main

import "testing"

func TestStorePutGet(t *testing.T) {
	s := NewStore()
	s.Put("foo", "bar")

	val, ok := s.Get("foo")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val != "bar" {
		t.Fatalf("expected 'bar', got '%s'", val)
	}
}

func TestStoreDelete(t *testing.T) {
	s := NewStore()
	s.Put("foo", "bar")
	s.Delete("foo")

	_, ok := s.Get("foo")
	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestStoreGetMissing(t *testing.T) {
	s := NewStore()
	_, ok := s.Get("nonexistent")
	if ok {
		t.Fatal("expected key to not exist")
	}
}