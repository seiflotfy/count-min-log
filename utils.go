package cml

import "github.com/dgryski/go-pcgr"

var rnd = pcgr.Rand{
	State: 0x0ddc0ffeebadf00d,
	Inc:   0xcafebabe,
}

func randFloat() float64 {
	return float64(rnd.Next()%10e5) / 10e5
}
