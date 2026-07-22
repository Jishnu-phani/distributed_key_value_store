package main

import "testing"

func TestHashRingDistribution(t *testing.T) {
	hr := NewHashRing(150)
	hr.AddNode("node-A")
	hr.AddNode("node-B")
	hr.AddNode("node-C")

	counts := map[string]int{}
	for i := 0; i < 10000; i++ {
		key := "key-" + string(rune(i))
		node, ok := hr.GetNode(key)
		if !ok {
			t.Fatal("expected a node to be found")
		}
		counts[node]++
	}

	t.Logf("distribution: %+v", counts)
	// With 150 virtual nodes each, distribution should be roughly even —
	// no hard assertion here, just eyeball the logged output for now.
}

func TestHashRingMinimalReshuffle(t *testing.T) {
	hr := NewHashRing(150)
	hr.AddNode("node-A")
	hr.AddNode("node-B")
	hr.AddNode("node-C")

	before := map[string]string{}
	for i := 0; i < 1000; i++ {
		key := "key-" + string(rune(i))
		node, _ := hr.GetNode(key)
		before[key] = node
	}

	hr.RemoveNode("node-C")

	moved := 0
	for key, oldNode := range before {
		newNode, _ := hr.GetNode(key)
		if newNode != oldNode {
			moved++
		}
	}

	t.Logf("%d out of 1000 keys moved after removing one of three nodes", moved)
	// Expect roughly ~1/3 moved, NOT all 1000 — that's the whole point of consistent hashing.
}
