package cml

import (
	"encoding/binary"
	"errors"
	"math"
	"math/rand/v2"
	"unsafe"

	"github.com/dgryski/go-farm"
)

// Register is the type of the register in the sketch.
type Register interface {
	uint8 | uint16 | uint32
}

/*
Sketch is a Count-Min-Log Sketch with generic register size. Not safe for concurrent use.
*/
type Sketch[T Register] struct {
	w      uint
	d      uint
	exp    float64
	logExp float64 // precomputed ln(exp) to optimize math operations
	rng    *rand.Rand
	store  []T
	idxs   []uint
}

/*
NewSketch returns a new Count-Min-Log Sketch with specified register size.

Parameters:
  - w: Width of the sketch matrix. Controls the size of each row in the sketch.
    Larger values reduce collisions but increase memory usage.
  - d: Depth of the sketch matrix (number of hash functions). Controls the number
    of independent estimators. Higher values improve accuracy at the cost of
    more memory and computation.
  - exp: Base for logarithmic counting (must be > 1.0). Controls the compression
    rate of counters. Higher values save memory but reduce precision for large
    counts. Typical values range from 1.1 to 2.0.

Returns a pointer to the initialized Sketch and any error encountered during creation.
*/
func NewSketch[T Register](w, d uint, exp float64) (*Sketch[T], error) {
	if exp <= 1.0 {
		return nil, errors.New("exp must be > 1.0")
	}

	store := make([]T, w*d)
	idxs := make([]uint, 0, d)

	return &Sketch[T]{
		w:      w,
		d:      d,
		exp:    exp,
		logExp: math.Log(exp),
		rng:    rand.New(rand.NewPCG(uint64(w), uint64(d))),
		store:  store,
		idxs:   idxs,
	}, nil
}

/*
NewSketchForEpsilonDelta creates a new Count-Min-Log Sketch optimized for given error bounds.

Parameters:
  - epsilon: The maximum relative error rate (ε) for frequency estimates. A smaller
    epsilon gives more accurate counts but requires more memory. Typical values
    range from 0.01 to 0.1.
  - delta: The probability (δ) that the relative error exceeds epsilon. A smaller
    delta provides higher confidence but requires more hash functions. Typical
    values range from 0.01 to 0.1.

The width of the sketch is set to ⌈e/ε⌉ and depth to ⌈ln(1/δ)⌉, where e is Euler's
number. The base for logarithmic counting (exp) is automatically tuned based on the
register size T.

Returns a pointer to the initialized Sketch and any error encountered during creation.
*/
func NewSketchForEpsilonDelta[T Register](epsilon, delta float64) (*Sketch[T], error) {
	var (
		width = uint(math.Ceil(math.E / epsilon))
		depth = uint(math.Ceil(math.Log(1 / delta)))
		alpha = math.Pow(float64(^T(0)), 1.0/(float64(^T(0))-1))
	)
	return NewSketch[T](width, depth, alpha)
}

/*
NewForCapacity creates a new Count-Min-Log Sketch optimized for a given capacity and error rate.

Parameters:
  - capacity: The expected maximum number of unique items to be counted. The sketch
    will be sized to efficiently handle up to this many items.
  - e: The desired error rate (between 0.001 and 1.0) that determines the sketch's
    accuracy. A smaller error rate provides more accurate counts but requires more
    memory. For example, e=0.01 means counts will be within ±1% of true values
    with high probability.

The sketch dimensions are automatically optimized based on these parameters to minimize
memory usage while maintaining the desired error bounds. The base for logarithmic
counting is tuned based on the register size T to optimize the space-accuracy tradeoff.

Returns a pointer to the initialized Sketch and an error if the error rate is invalid
(must be between 0.001 and 1.0).
*/
func NewForCapacity[T Register](capacity uint64, e float64) (*Sketch[T], error) {
	if !(e >= 0.001 && e < 1.0) {
		return nil, errors.New("e needs to be >= 0.001 and < 1.0")
	}

	m := max(1, math.Ceil((float64(capacity)*math.Log(e))/log05))
	w := max(1, math.Ceil(log2*m/float64(capacity)))

	maxVal := float64(^T(0))
	alpha := math.Pow(maxVal, 1.0/(maxVal-1))

	return NewSketch[T](uint(m/w), uint(w), alpha)
}

var (
	log05 = math.Log(0.5)
	log2  = math.Log(2.0)
)

func (cml *Sketch[T]) increaseDecision(c T) bool {
	return cml.rng.Float64() < math.Exp(-float64(c)*cml.logExp)
}

// Update increases the count of `s` by one, return true if added and the current count of `s`
func (cml *Sketch[T]) Update(e []byte) bool {
	minVal := ^T(0) // max value for type T
	cml.idxs = cml.idxs[:0]

	hs := farm.Hash64(e)
	h1 := uint32(hs & 0xffffffff)
	h2 := uint32((hs >> 32) & 0xffffffff)

	for i := uint(0); i < cml.d; i++ {
		saltedHash := uint(h1 + uint32(i)*h2)
		idx := (i * cml.w) + (saltedHash % cml.w)
		val := cml.store[idx]

		if val < minVal {
			minVal = val
			cml.idxs = cml.idxs[:1]
			cml.idxs[0] = idx
		} else if val == minVal {
			cml.idxs = append(cml.idxs, idx)
		}
	}

	// Check for overflow before incrementing
	if minVal >= ^T(0) {
		return false // Register saturated
	}

	if !cml.increaseDecision(minVal) {
		return false
	}

	minVal++
	for _, idx := range cml.idxs {
		cml.store[idx] = minVal
	}

	return true
}

/*
BulkUpdate increases the count of `s` by the given frequency, optimized to reduce redundant work.
It computes the current minimum among the d registers for the item, then for each frequency,
if the random decision passes, it increments all registers that equal the minimum.
*/
func (cml *Sketch[T]) BulkUpdate(e []byte, freq uint) bool {
	// Initialize minVal to the maximum possible value for type T.
	minVal := ^T(0)
	// Reset the slice that stores indices.
	cml.idxs = cml.idxs[:0]

	// Hash the input to get two independent 32-bit values.
	hs := farm.Hash64(e)
	h1 := uint32(hs & 0xffffffff)
	h2 := uint32((hs >> 32) & 0xffffffff)

	// Compute indices for each row and determine the overall minimum value.
	for i := uint(0); i < cml.d; i++ {
		saltedHash := uint(h1 + uint32(i)*h2)
		idx := (i * cml.w) + (saltedHash % cml.w)
		cml.idxs = append(cml.idxs, idx)
		val := cml.store[idx]
		if val < minVal {
			minVal = val
		}
	}

	anyUpdated := false
	maxVal := ^T(0)

	// Process freq increments.
	for i := uint(0); i < freq; i++ {
		// Stop if we've reached the maximum representable value
		if minVal >= maxVal {
			break
		}

		// Decide whether to increase based on the current minVal.
		if !cml.increaseDecision(minVal) {
			continue
		}
		updatedVal := minVal + 1
		updated := false

		// Increment all registers that equal the current minVal.
		for _, idx := range cml.idxs {
			if cml.store[idx] == minVal {
				cml.store[idx] = updatedVal
				updated = true
			}
		}

		if updated {
			anyUpdated = true
			// All registers with the old min have been incremented.
			// Therefore, the new min among these rows is now updatedVal.
			minVal = updatedVal
		}
	}

	return anyUpdated
}

func (cml *Sketch[T]) pointValue(c T) float64 {
	if c == 0 {
		return 0
	}
	return math.Exp(float64(c-1) * cml.logExp)
}

func (cml *Sketch[T]) value(c T) float64 {
	if c <= 1 {
		return cml.pointValue(c)
	}
	if c < ^T(0) { // Avoid overflow
		c++
	}
	return (1 - cml.pointValue(c)) / (1 - cml.exp)
}

/*
Query returns the count of `e`
*/
func (cml *Sketch[T]) Query(e []byte) float64 {
	c := ^T(0) // max value for type T
	hs := farm.Hash64(e)
	h1 := uint32(hs & 0xffffffff)
	h2 := uint32((hs >> 32) & 0xffffffff)

	for i := uint(0); i < cml.d; i++ {
		saltedHash := uint(h1 + uint32(i)*h2)
		idx := (i * cml.w) + (saltedHash % cml.w)
		if val := cml.store[idx]; val < c {
			c = val
		}
	}
	return cml.value(c)
}

/*
Merge combines two sketches by taking the maximum value at each position.
Returns an error if the sketches have different dimensions.
*/
func (cml *Sketch[T]) Merge(other *Sketch[T]) error {
	if cml.w != other.w || cml.d != other.d || cml.exp != other.exp || len(cml.store) != len(other.store) {
		return errors.New("sketches must have same dimensions and exp value")
	}

	for i := range cml.store {
		if other.store[i] > cml.store[i] {
			cml.store[i] = other.store[i]
		}
	}
	return nil
}

/*
MarshalBinary implements the encoding.BinaryMarshaler interface.
Returns a binary representation of the sketch.
*/
func (cml *Sketch[T]) MarshalBinary() ([]byte, error) {
	// Header: w(4) + d(4) + exp(8) = 16 bytes
	// Data: w * d * sizeof(T) bytes
	size := 16 + int(cml.w)*int(cml.d)*int(unsafe.Sizeof(T(0)))
	data := make([]byte, size)

	// Write header
	binary.LittleEndian.PutUint32(data[0:], uint32(cml.w))
	binary.LittleEndian.PutUint32(data[4:], uint32(cml.d))
	binary.LittleEndian.PutUint64(data[8:], math.Float64bits(cml.exp))

	// Write data
	offset := 16
	for _, val := range cml.store {
		switch any(val).(type) {
		case uint8:
			data[offset] = uint8(val)
			offset++
		case uint16:
			binary.LittleEndian.PutUint16(data[offset:], uint16(val))
			offset += 2
		case uint32:
			binary.LittleEndian.PutUint32(data[offset:], uint32(val))
			offset += 4
		}
	}
	return data, nil
}

/*
UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
Reconstructs a sketch from its binary representation.
*/
func (cml *Sketch[T]) UnmarshalBinary(data []byte) error {
	if len(data) < 16 {
		return errors.New("data too short")
	}

	// Read header
	w := uint(binary.LittleEndian.Uint32(data[0:]))
	d := uint(binary.LittleEndian.Uint32(data[4:]))
	exp := math.Float64frombits(binary.LittleEndian.Uint64(data[8:]))

	// Validate data size
	expectedSize := 16 + int(w)*int(d)*int(unsafe.Sizeof(T(0)))
	if len(data) != expectedSize {
		return errors.New("data size mismatch")
	}

	// Create new sketch
	sketch, err := NewSketch[T](w, d, exp)
	if err != nil {
		return err
	}

	// Read data
	offset := 16
	switch any(T(0)).(type) {
	case uint8:
		for i := range sketch.store {
			sketch.store[i] = T(data[offset])
			offset++
		}
	case uint16:
		for i := range sketch.store {
			sketch.store[i] = T(binary.LittleEndian.Uint16(data[offset:]))
			offset += 2
		}
	case uint32:
		for i := range sketch.store {
			sketch.store[i] = T(binary.LittleEndian.Uint32(data[offset:]))
			offset += 4
		}
	}

	// Update the receiver
	*cml = *sketch
	return nil
}
