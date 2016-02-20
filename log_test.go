package cml

import (
	"math"
	"testing"
)

func eval(got uint, expected uint) bool {
	return uint(math.Abs(float64(expected-got))) <= (expected / 100)
}

// Ensures that Add adds to the set and Count returns the correct
// approximation.
func TestLogAddAndCount(t *testing.T) {
	log, _ := NewForCapacity16(10000000, 0.01)

	log.Update([]byte("b"))
	log.Update([]byte("c"))
	log.Update([]byte("b"))
	log.Update([]byte("d"))
	log.BulkUpdate([]byte("a"), 1000000)

	if count := log.Query([]byte("a")); !eval(uint(count), 1000000) {
		t.Errorf("expected 1000000, got %d", uint(count))
	}

	if count := log.Query([]byte("b")); !eval(uint(count), 2) {
		t.Errorf("expected 2, got %d", uint(count))
	}

	if count := log.Query([]byte("c")); !eval(uint(count), 1) {
		t.Errorf("expected 1, got %d", uint(count))
	}

	if count := log.Query([]byte("d")); !eval(uint(count), 1) {
		t.Errorf("expected 1, got %d", uint(count))
	}

	if count := log.Query([]byte("x")); !eval(uint(count), 0) {
		t.Errorf("expected 0, got %d", uint(count))
	}
}
