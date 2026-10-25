package main

import "testing"

func TestStorePutGet(t *testing.T) {
	s := NewStore()
	s.Put("foo", VersionedValue{
		Value:  "bar",
		Vclock: VectorClock{"node-a": 1},
	})

	vals, ok := s.Get("foo")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if len(vals) != 1 {
		t.Fatalf("expected exactly 1 value, got %d", len(vals))
	}
	if vals[0].Value != "bar" {
		t.Fatalf("expected 'bar', got '%s'", vals[0].Value)
	}
}

func TestStoreDelete(t *testing.T) {
	s := NewStore()
	s.Put("foo", VersionedValue{
		Value:  "bar",
		Vclock: VectorClock{"node-a": 1},
	})
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

func TestStoreSequentialWriteReplaces(t *testing.T) {
	s := NewStore()

	// First write: node-a's clock at {a:1}
	s.Put("foo", VersionedValue{
		Value:  "v1",
		Vclock: VectorClock{"node-a": 1},
	})

	// Second write: builds on the first, {a:2} — strictly newer, should replace.
	s.Put("foo", VersionedValue{
		Value:  "v2",
		Vclock: VectorClock{"node-a": 2},
	})

	vals, ok := s.Get("foo")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if len(vals) != 1 {
		t.Fatalf("expected exactly 1 value after sequential write, got %d: %+v", len(vals), vals)
	}
	if vals[0].Value != "v2" {
		t.Fatalf("expected 'v2' to have replaced 'v1', got '%s'", vals[0].Value)
	}
}

func TestStoreConcurrentWritesProduceSiblings(t *testing.T) {
	s := NewStore()

	// Two genuinely concurrent writes — neither clock dominates the other,
	// same shape as the {a:3} vs {a:2,c:1} example worked through conceptually.
	s.Put("foo", VersionedValue{
		Value:  "from-a",
		Vclock: VectorClock{"node-a": 3},
	})
	s.Put("foo", VersionedValue{
		Value:  "from-c",
		Vclock: VectorClock{"node-a": 2, "node-c": 1},
	})

	vals, ok := s.Get("foo")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if len(vals) != 2 {
		t.Fatalf("expected 2 concurrent siblings, got %d: %+v", len(vals), vals)
	}

	values := map[string]bool{}
	for _, v := range vals {
		values[v.Value] = true
	}
	if !values["from-a"] || !values["from-c"] {
		t.Fatalf("expected both siblings present, got %+v", vals)
	}
}

func TestStoreNewWriteCollapsesSiblings(t *testing.T) {
	s := NewStore()

	// Create the same conflict as above...
	s.Put("foo", VersionedValue{
		Value:  "from-a",
		Vclock: VectorClock{"node-a": 3},
	})
	s.Put("foo", VersionedValue{
		Value:  "from-c",
		Vclock: VectorClock{"node-a": 2, "node-c": 1},
	})

	// ...then a resolving write whose clock dominates BOTH siblings
	// (this is what a coordinator's merged+incremented baseVC would produce).
	s.Put("foo", VersionedValue{
		Value:  "resolved",
		Vclock: VectorClock{"node-a": 3, "node-c": 1}, // dominates {a:3} and {a:2,c:1}
	})

	vals, ok := s.Get("foo")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if len(vals) != 1 {
		t.Fatalf("expected siblings to collapse to 1 value, got %d: %+v", len(vals), vals)
	}
	if vals[0].Value != "resolved" {
		t.Fatalf("expected 'resolved', got '%s'", vals[0].Value)
	}
}
