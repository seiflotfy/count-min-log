package countmin

import (
	"bufio"
	"fmt"
	"os"
	"testing"
)

func TestCountMin(t *testing.T) {

	sk, err := NewSketch(0.99, 0.0000001, 10)
	if err != nil {
		t.Error("Expected no error, go ", err)
	}

	fd, err := os.Open("/usr/share/dict/web2")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	scanner := bufio.NewScanner(fd)

	i := 0
	for scanner.Scan() {
		sk.Add(scanner.Text())
		i++
		if i == 1000000 {
			break
		}
	}

	fmt.Println(sk.topk)
	fmt.Println(sk.Get("unhealthily"))
}
