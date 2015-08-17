package countmin

import (
	"bufio"
	"fmt"
	"os"
	"testing"
)

func TestCountMin(t *testing.T) {

	log, err := NewDefaultLog()
	if err != nil {
		t.Error("Expected no error, go ", err)
	}
	sk := NewSketch(2718282, 7)

	for i := 0; i < 1; i++ {
		fd, err := os.Open("/usr/share/dict/web2")
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		scanner := bufio.NewScanner(fd)

		for scanner.Scan() {
			s := scanner.Text()
			log.Update(s)
			sk.Increment(s)
		}
	}
	topK := log.GetTopK()
	for k := range topK {
		fmt.Println(k, log.Query(k), sk.Count(k))
	}
	log.Update("seif")
}
