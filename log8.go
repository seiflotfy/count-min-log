package cml

import (
	"errors"
	"math"
)

func value8(c uint8, exp float64) float64 {
	if c == 0 {
		return 0.0
	}
	return math.Pow(exp, float64(c-1))
}

func fullValue8(c uint8, exp float64) float64 {
	if c <= 1 {
		return value8(c, exp)
	}
	return (1.0 - value8(c+1, exp)) / (1.0 - exp)
}

/*
Sketch8 is a Count-Min-Log sketch 8-bit registers
*/
type Sketch8 struct {
	w            uint
	k            uint
	conservative bool
	exp          float64
	maxSample    bool
	progressive  bool
	nBits        uint

	store      [][]uint8
	totalCount uint
	cMax       float64
}

/*
NewSketch8ForEpsilonDelta ...
*/
func NewSketch8ForEpsilonDelta(epsilon, delta float64) (*Sketch8, error) {
	var (
		width = uint(math.Ceil(math.E / epsilon))
		depth = uint(math.Ceil(math.Log(1 / delta)))
	)
	return NewSketch8(width, depth, true, 1.00026, true, true, 16)
}

/*
NewSketch8 returns a new Count-Min-Log sketch with 8-bit registers
*/
func NewSketch8(w uint, k uint, conservative bool, exp float64,
	maxSample bool, progressive bool, nBits uint) (*Sketch8, error) {
	store := make([][]uint8, k, k)
	for i := uint(0); i < k; i++ {
		store[i] = make([]uint8, w, w)
	}
	cMax := math.Pow(2.0, float64(nBits)) - 1.0
	if cMax > math.MaxUint8 {
		return nil,
			errors.New("using 8 bit registers allows a max nBits value of 8")
	}
	return &Sketch8{
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
NewDefaultSketch8 returns a new Count-Min-Log sketch with 8-bit registers and default settings
*/
func NewDefaultSketch8() (*Sketch8, error) {
	return NewSketch8(1000000, 7, true, 1.5, true, true, 8)
}

/*
NewForCapacity8 returns a new Count-Min-Log sketch with 8-bit registers optimized for a given max capacity and expected error rate
*/
func NewForCapacity8(capacity uint64, e float64) (*Sketch8, error) {
	// e = 2n/w    ==>    w = 2n/e
	if !(e >= 0.001 && e < 1.0) {
		return nil, errors.New("e needs to be >= 0.001 and < 1.0")
	}
	w := float64(2*capacity) / e
	return NewSketch8(uint(w), 7, true, 1.5, true, true, 8)
}

func (sk *Sketch8) randomLog(c uint8) bool {
	pIncrease := 1.0 / (fullValue8(c+1, sk.getExp(c+1)) - fullValue8(c, sk.getExp(c)))
	return randFloat() < pIncrease
}

func (sk *Sketch8) getExp(c uint8) float64 {
	if sk.progressive == true {
		return 1.0 + ((sk.exp - 1.0) * (float64(c) - 1.0) / sk.cMax)
	}
	return sk.exp
}

/*
Reset the Sketch to a fresh state (all counters set to 0)
*/
func (sk *Sketch8) Reset() {
	sk.store = make([][]uint8, sk.k, sk.k)
	for i := uint(0); i < sk.k; i++ {
		sk.store[i] = make([]uint8, sk.w, sk.w)
	}
	sk.totalCount = 0
}

/*
IncreaseCount increases the count of `s` by one, return true if added and the current count of `s`
*/
func (sk *Sketch8) IncreaseCount(s []byte) (bool, float64) {
	sk.totalCount++
	v := make([]uint8, sk.k, sk.k)
	vmin := uint8(math.MaxUint8)
	vmax := uint8(0)
	for i := range v {
		v[i] = sk.store[i][hash(s, uint(i), sk.w)]
		if v[i] < vmin {
			vmin = v[i]
		} else if v[i] > vmax {
			vmax = v[i]
		}
	}

	var c uint8
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
		return increase, fullValue8(vmin+1, sk.getExp(vmin+1))
	}
	return false, fullValue8(vmin, sk.getExp(vmin))
}

/*
GetCount returns the count of `s`
*/
func (sk *Sketch8) GetCount(s []byte) float64 {
	clmin := uint8(math.MaxUint8)
	for i := uint(0); i < sk.k; i++ {
		cl := sk.store[i][hash(s, i, sk.w)]
		if cl < clmin {
			clmin = cl
		}
	}
	c := clmin
	return fullValue8(c, sk.getExp(c))
}

/*
GetProbability returns the error probability of `s`
*/
func (sk *Sketch8) GetProbability(s []byte) float64 {
	v := sk.GetCount(s)
	if v > 0 {
		return v / float64(sk.totalCount)
	}
	return 0
}
