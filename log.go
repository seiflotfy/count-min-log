package countmin

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"

	"code.google.com/p/gofarmhash"
)

/*
Log ...
*/
type Log struct {
	w     uint
	d     uint
	k     uint
	count [][]uint8
	topk  map[string]uint
	x     float64
}

/*
NewLog returns a new count-min Log with parameters delta,
epsilon and k.

The parameters delta and epsilon control the accuracy of the
estimates of the Log.

Cormode and Muthukrishnan prove that for an item i with count a_i, the
estimate from the Log a_i_hat will satisfy the relation

a_hat_i <= a_i + epsilon * ||a||_1

with probability at least 1 - delta, where a is the the vector of all
all counts and ||x||_1 is the L1 norm of a vector x

Parameters
----------
delta : float64
    A value in the unit interval that sets the precision of the Log
epsilon : float64
    A value in the unit interval that sets the precision of the Log
k : uint
    A positive integer that sets the number of top items counted

Examples
--------
>>> s, err := NewLog(0.0000001, 0.005, 40)

Raises
------
ValueError
    If delta or epsilon are not in the unit interval
*/
func NewLog(delta float64, epsilon float64, k uint) (*Log, error) {
	if delta <= 0 || delta >= 1 {
		return nil, errors.New("delta must be between 0 and 1, exclusive")
	}
	if epsilon <= 0 || epsilon >= 1 {
		return nil, errors.New("epsilon must be between 0 and 1, exclusive")
	}
	if k < 1 {
		return nil, errors.New("k must be an integer > 1")
	}

	d := uint(math.Ceil(math.Log(1 / delta)))
	w := uint(math.Ceil(math.E / epsilon))

	fmt.Println(d, w)

	count := make([][]uint8, d)
	for i := uint(0); i < d; i++ {
		count[i] = make([]uint8, w)
	}

	log := &Log{
		w:     w,
		d:     d,
		k:     k,
		count: count,
		topk:  make(map[string]uint, k),
		x:     1.008,
	}
	return log, nil
}

/*
NewDefaultLog returns a logeth with delta = 0.0000001, epsilon = 0.0001 and k = 10
*/
func NewDefaultLog() (*Log, error) {
	return NewLog(0.001, 0.000001, 5)
}

/*
Reset the Log and clear all fields
*/
func (log *Log) Reset() {
	count := make([][]uint8, log.d)
	for i := uint(0); i < log.d; i++ {
		count[i] = make([]uint8, log.w)
	}

	log.count = count
	log.topk = make(map[string]uint, log.k)
}

func (log *Log) getMin(e string) uint8 {
	value := uint8(math.MaxUint8)
	w := uint(len(log.count[0]))
	d := uint(len(log.count))
	h1, h2 := hashn(e)
	for i := uint(0); i < d; i++ {
		column := (h1 + uint32(i)*h2) % uint32(w)
		value = uint8(math.Min(float64(log.count[i][column]), float64(value)))
	}
	return value
}

func (log *Log) increaseDecision(c uint8) bool {
	return rand.Float64() < math.Pow(float64(log.x), -float64(c))
}

/*
Update a key in the Log
Parameters
----------
key : string
    The item to update the value of in the Log
increment : int
    The amount to update the Log by for the given key

Examples
--------
>>> s, err := Log(0.0000001, 0.005, 40)
>>> s.Update('http://www.cnn.com/', 1)
*/
func (log *Log) Update(e string) {
	c := log.getMin(e)
	if log.increaseDecision(c) == true {
		w := uint(len(log.count[0]))
		d := uint(len(log.count))
		h1, h2 := hashn(e)
		for i := uint(0); i < d; i++ {
			column := (h1 + uint32(i)*h2) % uint32(w)
			if log.count[i][column] == c && c+1 < math.MaxUint8 {
				log.count[i][column] = c + 1
			}
		}
		log.updateTopK(e)
	}
}

func (log *Log) updateTopK(key string) {
	estimate := log.Query(key)
	var minKey string
	minCount := uint(math.MaxUint32)
	for key, count := range log.topk {
		if count < minCount {
			minCount = count
			minKey = key
		}
	}
	if estimate > minCount || uint(len(log.topk)) < log.k {
		if uint(len(log.topk)) >= log.k {
			delete(log.topk, minKey)
		}
		log.topk[key] = estimate
	}
}

/*
Query the Log estimate for the given key

Parameters
----------
key : string
   The item to produce an estimate for

Returns
-------
estimate : uint
   The best estimate of the count for the given key based on the
   Log

Examples
--------
>>> s, err := Log(0.0000001, 0.005, 40)
>>> s.Update('http://www.cnn.com/', 1)
>>> s.Get('http://www.cnn.com/')
1
*/
func (log *Log) Query(key string) uint {
	c := log.getMin(key)
	return uint(log.getValue(c))
}

func (log *Log) getPointValue(c uint8) float64 {
	if c == 0 {
		return 0
	}
	return math.Pow(float64(log.x), float64(c-1))
}

func (log *Log) getValue(c uint8) float64 {
	if c <= 1 {
		return log.getPointValue(c)
	}
	v := log.getPointValue(c + 1)
	return (1 - float64(v)) / (1 - float64(log.x))
}

/*
GetTopK return the top K keys and their counts (unsorted)
*/
func (log *Log) GetTopK() map[string]uint {
	return log.topk
}

func getHash(key string) uint {
	hasher := fnv.New32()
	hasher.Write([]byte(key))
	return uint(hasher.Sum32())
}

func hashn(s string) (h1, h2 uint32) {
	// This construction comes from
	// http://www.eecs.harvard.edu/~michaelm/postscripts/tr-02-05.pdf
	// "Building a Better Bloom Filter", by Kirsch and Mitzenmacher. Their
	// proof that this is allowed for count-min requires the h functions to
	// be from the 2-universal hash family, w be a prime and d be larger
	// than the traditional CM-sketch requirements.

	// Empirically, though, this seems to work "just fine".

	// TODO(dgryski): Switch to something that is actually validated by the literature.

	h1 = farmhash.Hash32([]byte(s))

	// inlined jenkins one-at-a-time hash
	h2 = uint32(0)
	for _, c := range s {
		h2 += uint32(c)
		h2 += h2 << 10
		h2 ^= h2 >> 6
	}
	h2 += (h2 << 3)
	h2 ^= (h2 >> 11)
	h2 += (h2 << 15)

	return h1, h2
}
