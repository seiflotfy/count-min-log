package cml

import (
	"errors"
	"math"
)

func value16(c uint16, exp float64) float64 {
	if c == 0 {
		return 0.0
	}
	return math.Pow(exp, float64(c-1))
}

func fullValue16(c uint16, exp float64) float64 {
	if c <= 1 {
		return value16(c, exp)
	}
	return (1.0 - value16(c+1, exp)) / (1.0 - exp)
}

/*
Sketch16 is a Count-Min-Log sketch 16-bit registers
*/
type Sketch16 struct {
	w            uint
	k            uint
	conservative bool
	exp          float64
	maxSample    bool
	progressive  bool
	nBits        uint

	store      [][]uint16
	totalCount uint
	cMax       float64
}

/*
NewSketch16 returns a new Count-Min-Log sketch with 16-bit registers
*/
func NewSketch16(w uint, k uint, conservative bool, exp float64,
	maxSample bool, progressive bool, nBits uint) (*Sketch16, error) {
	store := make([][]uint16, k, k)
	for i := uint(0); i < k; i++ {
		store[i] = make([]uint16, w, w)
	}
	cMax := math.Pow(2.0, float64(nBits)) - 1.0
	if cMax > math.MaxUint16 {
		return nil,
			errors.New("using 16 bit registers allows a max nBits value of 16")
	}
	return &Sketch16{
		w:            w,
		k:            k,
		conservative: conservative,
		exp:          exp,
		maxSample:    maxSample,
		progressive:  progressive,
		nBits:        nBits,
		store:        store,
		totalCount:   0.0,
		cMax:         cMax,
	}, nil
}

/*
NewSketch16ForEpsilonDelta ...
*/
func NewSketch16ForEpsilonDelta(epsilon, delta float64) (*Sketch16, error) {
	var (
		width = uint(math.Ceil(math.E / epsilon))
		depth = uint(math.Ceil(math.Log(1 / delta)))
	)
	return NewSketch16(width, depth, true, 1.00026, true, true, 16)
}

/*
NewDefaultSketch16 returns a new Count-Min-Log sketch with 16-bit registers and default settings
*/
func NewDefaultSketch16() (*Sketch16, error) {
	return NewSketch16(1000000, 7, true, 1.00026, true, true, 16)
}

/*
NewForCapacity16 returns a new Count-Min-Log sketch with 16-bit registers optimized for a given max capacity and expected error rate
*/
func NewForCapacity16(capacity uint64, e float64) (*Sketch16, error) {
	// e = 2n/w    ==>    w = 2n/e
	if !(e >= 0.001 && e < 1.0) {
		return nil, errors.New("e needs to be >= 0.001 and < 1.0")
	}
	w := float64(2*capacity) / e
	return NewSketch16(uint(w), 1, true, 1.00026, true, true, 16)
}

func (sk *Sketch16) randomLog(c uint16) bool {
	pIncrease := 1.0 / (fullValue16(c+1, sk.getExp(c+1)) - fullValue16(c, sk.getExp(c)))
	return randFloat() < pIncrease
}

func (sk *Sketch16) getExp(c uint16) float64 {
	if sk.progressive == true {
		return 1.0 + ((sk.exp - 1.0) * (float64(c) - 1.0) / sk.cMax)
	}
	return sk.exp
}

/*
GetFillRate ...
*/
func (sk *Sketch16) GetFillRate() float64 {
	occs := 0.0
	size := sk.w * sk.k
	for _, row := range sk.store {
		for _, col := range row {
			if col > 0 {
				occs++
			}
		}
	}
	return 100 * occs / float64(size)
}

/*
Reset the Sketch to a fresh state (all counters set to 0)
*/
func (sk *Sketch16) Reset() {
	sk.store = make([][]uint16, sk.k, sk.k)
	for i := 0; i < len(sk.store); i++ {
		sk.store[i] = make([]uint16, sk.w, sk.w)
	}
	sk.totalCount = 0
}

/*
IncreaseCount increases the count of `s` by one, return true if added and the current count of `s`
*/
func (sk *Sketch16) IncreaseCount(s []byte) (bool, float64) {
	sk.totalCount++
	v := make([]uint16, sk.k, sk.k)
	vmin := uint16(math.MaxUint16)
	vmax := uint16(0)
	for i := range v {
		v[i] = sk.store[i][hash(s, uint(i), sk.w)]
		if v[i] < vmin {
			vmin = v[i]
		}
		if v[i] > vmax {
			vmax = v[i]
		}
	}

	var c uint16
	if sk.maxSample {
		c = vmax
	} else {
		c = vmin
	}

	if float64(c) > sk.cMax {
		return false, 0.0
	}

	increase := sk.randomLog(c)
	if increase {
		for i := uint(0); i < sk.k; i++ {
			nc := v[i]
			if !sk.conservative || vmin == nc {
				sk.store[i][hash(s, i, sk.w)] = nc + 1
			}
		}
		return increase, fullValue16(vmin+1, sk.getExp(vmin+1))
	}
	return false, fullValue16(vmin, sk.getExp(vmin))
}

/*
GetCount returns the count of `s`
*/
func (sk *Sketch16) GetCount(s []byte) float64 {
	clmin := uint16(math.MaxUint16)
	for i := uint(0); i < sk.k; i++ {
		cl := sk.store[i][hash(s, i, sk.w)]
		if cl < clmin {
			clmin = cl
		}
	}
	c := clmin
	return fullValue16(c, sk.getExp(c))
}

/*
GetProbability returns the error probability of `s`
*/
func (sk *Sketch16) GetProbability(s []byte) float64 {
	v := sk.GetCount(s)
	if v > 0 {
		return v / float64(sk.totalCount)
	}
	return 0
}
