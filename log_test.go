package cml

import "testing"

// Ensures that Add adds to the set and Count returns the correct
// approximation.
func TestLogAddAndCount(t *testing.T) {
	log, _ := NewForCapacity16(10000000, 0.01)

	log.Update([]byte("b"))
	log.Update([]byte("c"))
	log.Update([]byte("b"))
	log.Update([]byte("d"))
	log.BulkUpdate([]byte("a"), 1000000)

	if count := log.Query([]byte("a")); uint(count) != 0 {
		t.Errorf("expected 3, got %d", uint(count))
	}

	if count := log.Query([]byte("b")); uint(count) != 0 {
		t.Errorf("expected 2, got %d", uint(count))
	}

	if count := log.Query([]byte("c")); uint(count) != 0 {
		t.Errorf("expected 1, got %d", uint(count))
	}

	if count := log.Query([]byte("d")); uint(count) != 0 {
		t.Errorf("expected 1, got %d", uint(count))
	}

	if count := log.Query([]byte("x")); uint(count) != 0 {
		t.Errorf("expected 0, got %d", uint(count))
	}
}
