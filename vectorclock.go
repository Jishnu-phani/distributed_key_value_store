package main

type VectorClock map[string]int64

func (vc VectorClock) Copy() VectorClock {
	newVC := make(VectorClock, len(vc))
	for k, v := range vc {
		newVC[k] = v
	}
	return newVC
}

// Increment returns a NEW VectorClock with nodeID's counter bumped by 1.
// It never mutates the receiver — vector clocks are treated as immutable
// values once attached to a stored version.
func (vc VectorClock) Increment(nodeID string) VectorClock {
	newVC := vc.Copy()
	newVC[nodeID]++
	return newVC
}

// Merge returns the component-wise maximum of vc and other — used when a
// write needs to account for every version it has "seen."
func (vc VectorClock) Merge(other VectorClock) VectorClock {
	merged := vc.Copy()
	for k, v := range other {
		if v > merged[k] {
			merged[k] = v
		}
	}
	return merged
}

type Ordering int

const (
	Equal Ordering = iota
	Before
	After
	Concurrent
)

func (vc VectorClock) Compare(other VectorClock) Ordering {

	keys := make(map[string]bool)

	for k := range vc {
		keys[k] = true
	}
	for k := range other {
		keys[k] = true
	}

	vcStatus := Equal
	// otherStatus := Equal

	for k := range keys {

		if vc[k] < other[k] {

			// if (vcStatus == After || otherStatus == Before){
			if vcStatus == After {
				return Concurrent
			} else {
				vcStatus = Before
				// otherStatus = After
			}

		} else if vc[k] > other[k] {

			// if (vcStatus == Before || otherStatus == After) {
			if vcStatus == Before {
				return Concurrent
			} else {
				vcStatus = After
				// otherStatus = Before
			}

		}

	}

	return vcStatus
}
