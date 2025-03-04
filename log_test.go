package cml

import (
	"fmt"
	"math"
	"testing"
)

// evalWithinPercentage checks if the actual value is within a given percentage of the expected value
func evalWithinPercentage(t *testing.T, got float64, expected uint, percentage float64) bool {
	t.Helper()
	if expected == 0 {
		return got < 1.0 // For zero expected, just ensure the value is very small
	}
	diff := math.Abs(float64(expected) - got)
	maxDiff := float64(expected) * (percentage / 100.0)
	return diff <= maxDiff
}

// TestSketchCreation verifies that sketches can be created with various parameters
func TestSketchCreation(t *testing.T) {
	t.Run("NewSketch", func(t *testing.T) {
		sketch, err := NewSketch[uint16](10, 5, 1.5)
		if err != nil {
			t.Fatalf("Failed to create sketch: %v", err)
		}
		if sketch.w != 10 {
			t.Errorf("Expected width 10, got %d", sketch.w)
		}
		if sketch.d != 5 {
			t.Errorf("Expected depth 5, got %d", sketch.d)
		}
		if sketch.exp != 1.5 {
			t.Errorf("Expected exp 1.5, got %f", sketch.exp)
		}
	})

	t.Run("NewSketchInvalidExp", func(t *testing.T) {
		_, err := NewSketch[uint16](10, 5, 0.5)
		if err == nil {
			t.Error("Expected error for exp <= 1.0, got nil")
		}
	})

	t.Run("NewSketchForEpsilonDelta", func(t *testing.T) {
		sketch, err := NewSketchForEpsilonDelta[uint16](0.01, 0.01)
		if err != nil {
			t.Fatalf("Failed to create sketch: %v", err)
		}
		if sketch.w == 0 || sketch.d == 0 {
			t.Errorf("Invalid dimensions: w=%d, d=%d", sketch.w, sketch.d)
		}
	})

	t.Run("NewForCapacity16", func(t *testing.T) {
		sketch, err := NewForCapacity[uint16](10000000, 0.01)
		if err != nil {
			t.Fatalf("Failed to create sketch: %v", err)
		}
		if sketch.w == 0 || sketch.d == 0 {
			t.Errorf("Invalid dimensions: w=%d, d=%d", sketch.w, sketch.d)
		}
	})
}

// TestUpdate tests the Update method with various inputs
func TestUpdate(t *testing.T) {
	sketch, err := NewForCapacity[uint16](10000000, 0.01)
	if err != nil {
		t.Fatalf("Failed to create sketch: %v", err)
	}

	t.Run("SingleUpdate", func(t *testing.T) {
		key := []byte("test-key")
		sketch.Update(key)
		count := sketch.Query(key)

		if count < 0.5 || count > 1.5 {
			t.Errorf("Expected count ~1 after single update, got %f", count)
		}

		// Update again and verify count increases
		sketch.Update(key)
		newCount := sketch.Query(key)
		if newCount <= count {
			t.Errorf("Count did not increase after update: %f -> %f", count, newCount)
		}
	})

	t.Run("MultipleUpdates", func(t *testing.T) {
		key := []byte("multi-key")
		updates := 10

		for range updates {
			sketch.Update(key)
		}

		count := sketch.Query(key)
		if !evalWithinPercentage(t, count, uint(updates), 1) {
			t.Errorf("Expected count ~%d after %d updates, got %f", updates, updates, count)
		}
	})
}

// TestBulkUpdate tests the BulkUpdate method with various frequencies
func TestBulkUpdate(t *testing.T) {
	sketch, err := NewForCapacity[uint16](10000000, 0.01)
	if err != nil {
		t.Fatalf("Failed to create sketch: %v", err)
	}

	t.Run("SmallBulkUpdate", func(t *testing.T) {
		key := []byte("small-bulk")
		freq := uint(50)

		sketch.BulkUpdate(key, freq)
		count := sketch.Query(key)

		if !evalWithinPercentage(t, count, freq, 2) {
			t.Errorf("Expected count ~%d after bulk update, got %f", freq, count)
		}
	})

	t.Run("LargeBulkUpdate", func(t *testing.T) {
		key := []byte("large-bulk")
		freq := uint(100000)

		sketch.BulkUpdate(key, freq)
		count := sketch.Query(key)

		if !evalWithinPercentage(t, count, freq, 1) {
			t.Errorf("Expected count ~%d after large bulk update, got %f", freq, count)
		}
	})
}

// TestLogAddAndCount is a comprehensive test of Update, BulkUpdate and Query operations
func TestLogAddAndCount(t *testing.T) {
	sketch, err := NewForCapacity[uint16](10000000, 0.01)
	if err != nil {
		t.Fatalf("Failed to create sketch: %v", err)
	}

	// Test data with expected counts
	testData := []struct {
		key      string
		updates  int
		bulkSize uint
		expected uint
	}{
		{"a", 0, 1000000, 1000000},
		{"b", 2, 0, 2},
		{"c", 1, 0, 1},
		{"d", 1, 0, 1},
		{"e", 0, 0, 0}, // Should have near-zero count
	}

	// Perform updates
	for _, data := range testData {
		for i := 0; i < data.updates; i++ {
			sketch.Update([]byte(data.key))
		}
		if data.bulkSize > 0 {
			sketch.BulkUpdate([]byte(data.key), data.bulkSize)
		}
	}

	// Verify counts
	for _, data := range testData {
		t.Run("Key_"+data.key, func(t *testing.T) {
			count := sketch.Query([]byte(data.key))

			errorMargin := 1.0
			if data.expected > 100000 { // Large counts have a higher error margin
				errorMargin = 2.0
			}

			if !evalWithinPercentage(t, count, data.expected, errorMargin) {
				t.Errorf("For key %s: expected ~%d, got %f (outside %f%% margin)",
					data.key, data.expected, count, errorMargin)
			}
		})
	}

	// Test collision behavior - this key should have a small but non-zero count due to hash collisions
	t.Run("CollisionBehavior", func(t *testing.T) {
		collisionKey := []byte("collision-test")
		count := sketch.Query(collisionKey)
		if count > 5.0 {
			t.Errorf("Unexpected high count for unused key: %f", count)
		}
	})
}

// TestMerge tests the Merge functionality with various scenarios
func TestMerge(t *testing.T) {
	t.Run("MergeSameDimensions", func(t *testing.T) {
		// Create two sketches with dimensions appropriate for our test counts
		sketch1, _ := NewForCapacity[uint16](10000, 0.01)
		sketch2, _ := NewForCapacity[uint16](10000, 0.01)

		// Add different items to each sketch
		sketch1.BulkUpdate([]byte("item1"), 1000)
		sketch2.BulkUpdate([]byte("item2"), 2000)

		// Merge sketch2 into sketch1
		err := sketch1.Merge(sketch2)
		if err != nil {
			t.Fatalf("Failed to merge sketches: %v", err)
		}

		// Verify counts after merge
		count1 := sketch1.Query([]byte("item1"))
		count2 := sketch1.Query([]byte("item2"))

		if !evalWithinPercentage(t, count1, 1000, 1.0) {
			t.Errorf("Expected ~1000 for item1 after merge, got %f", count1)
		}
		if !evalWithinPercentage(t, count2, 2000, 1.0) {
			t.Errorf("Expected ~2000 for item2 after merge, got %f", count2)
		}
	})

	t.Run("MergeDifferentDimensions", func(t *testing.T) {
		// Use NewSketch directly to ensure different dimensions
		sketch1, _ := NewSketch[uint16](10, 5, 1.5)
		sketch2, _ := NewSketch[uint16](20, 5, 1.5)

		err := sketch1.Merge(sketch2)
		if err == nil {
			t.Error("Expected error when merging sketches with different dimensions")
		}
	})

	t.Run("MergeDifferentExp", func(t *testing.T) {
		sketch1, _ := NewSketch[uint16](10, 5, 1.5)
		sketch2, _ := NewSketch[uint16](10, 5, 1.6)

		err := sketch1.Merge(sketch2)
		if err == nil {
			t.Error("Expected error when merging sketches with different exp values")
		}
	})

	t.Run("MergeWithOverlappingItems", func(t *testing.T) {
		sketch1, _ := NewForCapacity[uint16](10000, 0.01)
		sketch2, _ := NewForCapacity[uint16](10000, 0.01)

		// Add same item to both sketches
		key := []byte("common-item")
		sketch1.BulkUpdate(key, 1000)
		sketch2.BulkUpdate(key, 2000)

		err := sketch1.Merge(sketch2)
		if err != nil {
			t.Fatalf("Failed to merge sketches: %v", err)
		}

		// After merge, count should be at least the maximum of both counts
		count := sketch1.Query(key)
		if !evalWithinPercentage(t, count, 2000, 1.0) {
			t.Errorf("Expected ~2000 after merging overlapping items, got %f", count)
		}
	})
}

// TestMarshalUnmarshal tests the binary marshaling and unmarshaling functionality
func TestMarshalUnmarshal(t *testing.T) {
	t.Run("MarshalAndUnmarshalEmpty", func(t *testing.T) {
		original, _ := NewSketch[uint16](10, 5, 1.5)

		// Marshal
		data, err := original.MarshalBinary()
		if err != nil {
			t.Fatalf("Failed to marshal empty sketch: %v", err)
		}

		// Unmarshal into new sketch
		reconstructed := &Sketch[uint16]{}
		err = reconstructed.UnmarshalBinary(data)
		if err != nil {
			t.Fatalf("Failed to unmarshal empty sketch: %v", err)
		}

		// Verify dimensions and parameters
		if original.w != reconstructed.w || original.d != reconstructed.d || original.exp != reconstructed.exp {
			t.Error("Reconstructed sketch parameters don't match original")
		}
	})

	t.Run("MarshalAndUnmarshalWithData", func(t *testing.T) {
		original, _ := NewSketch[uint16](10, 5, 1.5)

		// Add some data
		testItems := []struct {
			key   string
			count uint
		}{
			{"item1", 1000},
			{"item2", 2000},
			{"item3", 3000},
		}

		for _, item := range testItems {
			original.BulkUpdate([]byte(item.key), item.count)
		}

		// Marshal
		data, err := original.MarshalBinary()
		if err != nil {
			t.Fatalf("Failed to marshal sketch with data: %v", err)
		}

		// Unmarshal into new sketch
		reconstructed := &Sketch[uint16]{}
		err = reconstructed.UnmarshalBinary(data)
		if err != nil {
			t.Fatalf("Failed to unmarshal sketch with data: %v", err)
		}

		// Verify counts are preserved
		for _, item := range testItems {
			originalCount := original.Query([]byte(item.key))
			reconstructedCount := reconstructed.Query([]byte(item.key))

			// The counts should be exactly equal since we're using the same data
			if originalCount != reconstructedCount {
				t.Errorf("Count mismatch for %s: original=%f, reconstructed=%f",
					item.key, originalCount, reconstructedCount)
			}
		}
	})

	t.Run("UnmarshalInvalidData", func(t *testing.T) {
		sketch := &Sketch[uint16]{}

		// Test with data that's too short
		err := sketch.UnmarshalBinary([]byte{1, 2, 3})
		if err == nil {
			t.Error("Expected error when unmarshaling too short data")
		}

		// Test with data that has invalid size
		invalidData := make([]byte, 20) // Not enough data for header + store
		err = sketch.UnmarshalBinary(invalidData)
		if err == nil {
			t.Error("Expected error when unmarshaling data with invalid size")
		}
	})

	t.Run("MarshalUnmarshalRoundTrip", func(t *testing.T) {
		// Create a sketch with specific dimensions
		original, _ := NewSketch[uint16](15, 7, 1.5)

		// Add some random data
		for i := 0; i < 1000; i++ {
			key := []byte(string(rune(i)))
			original.BulkUpdate(key, uint(i))
		}

		// Marshal
		data, err := original.MarshalBinary()
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		// Unmarshal
		reconstructed := &Sketch[uint16]{}
		err = reconstructed.UnmarshalBinary(data)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		// Verify dimensions and parameters
		if original.w != reconstructed.w ||
			original.d != reconstructed.d ||
			original.exp != reconstructed.exp {
			t.Error("Reconstructed sketch parameters don't match original")
		}

		// Verify some random keys have same counts
		for i := 0; i < 100; i++ {
			key := []byte(string(rune(i)))
			originalCount := original.Query(key)
			reconstructedCount := reconstructed.Query(key)
			if originalCount != reconstructedCount {
				t.Errorf("Count mismatch for key %d: original=%f, reconstructed=%f",
					i, originalCount, reconstructedCount)
			}
		}
	})
}

// TestOverflowProtection tests the overflow protection and saturation handling
func TestOverflowProtection(t *testing.T) {
	sketch, err := NewSketchForEpsilonDelta[uint8](0.01, 0.01)
	if err != nil {
		t.Fatalf("Failed to create sketch: %v", err)
	}

	key := []byte("test-key")
	maxVal := ^uint8(0) // 255

	// Test 1: Verify normal operation
	if !sketch.Update(key) {
		t.Fatal("First update should succeed")
	}
	count := sketch.Query(key)
	if count <= 0 {
		t.Errorf("Expected positive count, got %f", count)
	}

	// Test 2: Verify saturation behavior
	for i := range sketch.store {
		sketch.store[i] = maxVal
	}
	if sketch.Update(key) {
		t.Fatal("Update should fail when registers are saturated")
	}
	if sketch.BulkUpdate(key, 10) {
		t.Fatal("BulkUpdate should fail when registers are saturated")
	}

	// Test 3: Verify no overflow occurred
	for i, val := range sketch.store {
		if val > maxVal {
			t.Errorf("Register %d exceeded max value: got %d, max %d", i, val, maxVal)
		}
	}

	// Test 4: Verify count remains valid after saturation
	saturatedCount := sketch.Query(key)
	if saturatedCount <= count {
		t.Errorf("Saturated count %f should be greater than initial count %f", saturatedCount, count)
	}
}

func BenchmarkSketch(b *testing.B) {
	// Test different sketch sizes
	sizes := []struct {
		capacity uint64
		error    float64
	}{
		{1e6, 0.01},   // Small sketch
		{10e6, 0.01},  // Medium sketch
		{100e6, 0.01}, // Large sketch
	}

	for _, size := range sizes {
		sketch, err := NewForCapacity[uint16](size.capacity, size.error)
		if err != nil {
			b.Fatalf("Failed to create sketch: %v", err)
		}

		name := fmt.Sprintf("fn=Update/cap=%d/err=%.3f", size.capacity, size.error)
		b.Run(name, func(b *testing.B) {
			key := []byte("test-key")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sketch.Update(key)
			}
		})

		// Test BulkUpdate with different frequencies
		freqs := []uint{10, 100, 1000, 10000}
		for _, freq := range freqs {
			name := fmt.Sprintf("fn=BulkUpdate/cap=%d/err=%.3f/freq=%d", size.capacity, size.error, freq)
			b.Run(name, func(b *testing.B) {
				key := []byte("test-key")
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					sketch.BulkUpdate(key, freq)
				}
			})
		}

		// Benchmark Query operation
		b.Run(fmt.Sprintf("fn=Query/cap=%d/err=%.3f", size.capacity, size.error), func(b *testing.B) {
			key := []byte("test-key")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sketch.Query(key)
			}
		})
	}
}
