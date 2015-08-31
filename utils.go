package cml

import (
	"math/rand"
	"strconv"

	"code.google.com/p/gofarmhash"
)

func randFloat() float64 {
	return rand.Float64()
}

func hash(s []byte, i, w uint) uint {
	str := strconv.Itoa(int(i)) + string(s)
	return uint(farmhash.Hash64([]byte(str))) % w
}
