package countmin

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"

	"code.google.com/p/gofarmhash"
)

const bigPrime = 9223372036854775783

func randomParameter() uint {
	return uint(rand.Int63n(bigPrime - 1))
}

/*
Sketch ...
*/
type Sketch struct {
	w         uint
	d         uint
	k         uint
	hashFuncs []func(x uint) uint
	count     [][]uint
	topk      map[string]uint
	total     uint
}

/*
NewSketch returns a new count-min sketch with parameters delta,
epsilon and k.

The parameters delta and epsilon control the accuracy of the
estimates of the sketch.

Cormode and Muthukrishnan prove that for an item i with count a_i, the
estimate from the sketch a_i_hat will satisfy the relation

a_hat_i <= a_i + epsilon * ||a||_1

with probability at least 1 - delta, where a is the the vector of all
all counts and ||x||_1 is the L1 norm of a vector x

Parameters
----------
delta : float64
    A value in the unit interval that sets the precision of the sketch
epsilon : float64
    A value in the unit interval that sets the precision of the sketch
k : uint
    A positive integer that sets the number of top items counted

Examples
--------
>>> s, err := NewSketch(0.0000001, 0.005, 40)

Raises
------
ValueError
    If delta or epsilon are not in the unit interval
*/
func NewSketch(delta float64, epsilon float64, k uint) (*Sketch, error) {
	if delta <= 0 || delta >= 1 {
		return nil, errors.New("delta must be between 0 and 1, exclusive")
	}
	if epsilon <= 0 || epsilon >= 1 {
		return nil, errors.New("epsilon must be between 0 and 1, exclusive")
	}
	if k < 1 {
		return nil, errors.New("k must be an integer > 1")
	}

	w := uint(math.Ceil(math.Exp(1) / epsilon))
	d := uint(math.Ceil(math.Log(1 / delta)))

	fmt.Println(w, d)

	count := make([][]uint, d)
	for i := uint(0); i < d; i++ {
		count[i] = make([]uint, w)
	}

	sketch := &Sketch{
		w:         w,
		d:         d,
		k:         k,
		hashFuncs: generateHashFunctions(d, w),
		count:     count,
		topk:      make(map[string]uint, k),
		total:     0,
	}
	return sketch, nil
}

/*
NewDefaultSketch returns a Sketh with delta = 0.0000001, epsilon = 0.0001 and k = 10
*/
func NewDefaultSketch() (*Sketch, error) {
	return NewSketch(0.99, 0.0000001, 10)
}

/*
Reset the sketch and clear all fields
*/
func (sk *Sketch) Reset() {
	count := make([][]uint, sk.d)
	for i := uint(0); i < sk.d; i++ {
		count[i] = make([]uint, sk.w)
	}

	sk.count = count
	sk.topk = make(map[string]uint, sk.k)
	sk.total = 0
}

/*
Add a key to the sketch
Parameters
----------
key : string
    The item to update the value of in the sketch
increment : int
    The amount to update the sketch by for the given key

Examples
--------
>>> s, err := Sketch(0.0000001, 0.005, 40)
>>> s.Update('http://www.cnn.com/', 1)
*/
func (sk *Sketch) Add(key string) {
	for i, hashFunc := range sk.hashFuncs {
		x := farmhash.Hash64([]byte(key))
		column := hashFunc(uint(math.Abs(float64(x))))
		if sk.count[i][column] < uint(math.MaxUint32) {
			sk.count[i][column]++
			sk.total++
		}
	}
	sk.updateTopK(key)
}

func (sk *Sketch) updateTopK(key string) {
	estimate := sk.Get(key)
	var minKey string
	minCount := uint(math.MaxUint32)
	for key, count := range sk.topk {
		if count < minCount {
			minCount = count
			minKey = key
		}
	}
	if estimate > minCount || uint(len(sk.topk)) < sk.k {
		if uint(len(sk.topk)) >= sk.k {
			delete(sk.topk, minKey)
		}
		sk.topk[key] = estimate
	}
}

/*
Get the sketch estimate for the given key

Parameters
----------
key : string
   The item to produce an estimate for

Returns
-------
estimate : uint
   The best estimate of the count for the given key based on the
   sketch

Examples
--------
>>> s, err := Sketch(0.0000001, 0.005, 40)
>>> s.Update('http://www.cnn.com/', 1)
>>> s.Get('http://www.cnn.com/')
1
*/
func (sk *Sketch) Get(key string) uint {
	//value := uint(math.MaxUint32)
	values := make([]uint, len(sk.hashFuncs), len(sk.hashFuncs))
	for i, hashFunc := range sk.hashFuncs {
		x := farmhash.Hash64([]byte(key))
		column := hashFunc(uint(math.Abs(float64(x))))
		values[i] = uint(float64(sk.count[i][column]))
		err := (sk.total - values[i]) / (sk.w - 1)
		values[i] = values[i] - err
	}
	sorted := uInt(values)
	sort.Sort(sorted)
	return values[sk.d/2]
}

type uInt []uint

func (a uInt) Len() int           { return len(a) }
func (a uInt) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a uInt) Less(i, j int) bool { return a[i] < a[j] }

/*
GetTopK return the top K keys and their counts (unsorted)
*/
func (sk *Sketch) GetTopK() map[string]uint {
	return sk.topk
}

func generateHashFunctions(n uint, w uint) []func(x uint) uint {
	funcs := make([]func(x uint) uint, n, n)
	a, b := randomParameter(), randomParameter()
	for i := uint(0); i < n; i++ {
		funcs[i] = func(x uint) uint {
			return (a*x + b) % uint(bigPrime) % uint(w)
		}
	}
	return funcs
}
