package cml

import (
	"bytes"
	"encoding/binary"
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
	maxSample    bool
	progressive  bool
	conservative bool
	w            uint
	k            uint
	nBits        uint
	totalCount   uint
	exp          float64
	cMax         float64

	store [][]uint16
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
Frequency returns the count of `s`
*/
func (sk *Sketch16) Frequency(s []byte) float64 {
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
Probability returns the error probability of `s`
*/
func (sk *Sketch16) Probability(s []byte) float64 {
	v := sk.Frequency(s)
	if v > 0 {
		return v / float64(sk.totalCount)
	}
	return 0
}

/*
TotalCount returns total count of samples
*/
func (sk *Sketch16) TotalCount() uint {
	return sk.totalCount
}

/*
Marshall returns a serialized byte array representing the structure
*/
func (sk *Sketch16) Marshall() ([]byte, error) {
	buf := new(bytes.Buffer)

	maxSample := uint8(0)
	if sk.maxSample {
		maxSample = 1
	}
	progressive := uint8(0)
	if sk.progressive {
		progressive = 1
	}
	conservative := uint8(0)
	if sk.conservative {
		conservative = 1
	}
	if err := binary.Write(buf, binary.LittleEndian, maxSample); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, progressive); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, conservative); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint64(sk.w)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint64(sk.k)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint64(sk.nBits)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint64(sk.totalCount)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, float64(sk.exp)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, float64(sk.cMax)); err != nil {
		return nil, err
	}

	bytes := make([]byte, sk.k*sk.w*2, sk.k*sk.w*2)
	for i := range sk.store {
		for j, value := range sk.store[i] {
			d := make([]byte, 2)
			pos := uint(i)*sk.w*2 + uint(j)*2
			binary.LittleEndian.PutUint16(d, value)
			bytes[pos] = d[0]
			bytes[pos+1] = d[1]
		}
	}
	data := append(buf.Bytes(), bytes...)
	return data, nil
}

/*
Unmarshall16 returns a Sketch16 from an serialized byte array
*/
func Unmarshall16(b []byte) (*Sketch16, error) {
	imaxSample := uint8(0)
	iprogressive := uint8(0)
	iconservative := uint8(0)
	w := uint64(0)
	k := uint64(0)
	nBits := uint64(0)
	totalCount := uint64(0)
	exp := float64(0)
	cMax := float64(0)
	buf := bytes.NewReader(b[0:1])

	if err := binary.Read(buf, binary.LittleEndian, &imaxSample); err != nil {
		return nil, err
	}
	buf = bytes.NewReader(b[1:2])
	if err := binary.Read(buf, binary.LittleEndian, &iprogressive); err != nil {
		return nil, err
	}
	buf = bytes.NewReader(b[2:3])
	if err := binary.Read(buf, binary.LittleEndian, &iconservative); err != nil {
		return nil, err
	}

	maxSample := false
	if imaxSample > 0 {
		maxSample = true
	}
	progressive := false
	if iprogressive > 0 {
		progressive = true
	}
	conservative := false
	if iconservative > 0 {
		conservative = true
	}

	buf = bytes.NewReader(b[3:11])
	if err := binary.Read(buf, binary.LittleEndian, &w); err != nil {
		return nil, err
	}
	buf = bytes.NewReader(b[11:19])
	if err := binary.Read(buf, binary.LittleEndian, &k); err != nil {
		return nil, err
	}
	buf = bytes.NewReader(b[19:27])
	if err := binary.Read(buf, binary.LittleEndian, &nBits); err != nil {
		return nil, err
	}
	buf = bytes.NewReader(b[27:35])
	if err := binary.Read(buf, binary.LittleEndian, &totalCount); err != nil {
		return nil, err
	}
	buf = bytes.NewReader(b[35:43])
	if err := binary.Read(buf, binary.LittleEndian, &exp); err != nil {
		return nil, err
	}
	buf = bytes.NewReader(b[43:51])
	if err := binary.Read(buf, binary.LittleEndian, &cMax); err != nil {
		return nil, err
	}

	store := make([][]uint16, k, k)
	for i := range store {
		store[i] = make([]uint16, w, w)
		for j := range store[i] {
			pos := 51 + uint(i)*uint(w)*2 + uint(j)*2
			value := binary.LittleEndian.Uint16(b[pos : pos+2])
			store[i][j] = value
		}
	}

	sketch16 := &Sketch16{maxSample: maxSample, progressive: progressive, conservative: conservative,
		w: uint(w), k: uint(k), nBits: uint(nBits), totalCount: uint(totalCount),
		exp: exp, cMax: cMax, store: store}

	return sketch16, nil
}
