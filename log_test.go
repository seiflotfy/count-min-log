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

	for i := 0; i < 1000; i++ {
		fd, err := os.Open("/usr/share/dict/web2")
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		scanner := bufio.NewScanner(fd)

		j := 0
		for scanner.Scan() {
			s := scanner.Text()
			log.Update(s)
			j++
			if j == 100 {
				break
			}
		}
		fd.Close()
	}
	topK := log.GetTopK()
	for k := range topK {
		fmt.Println(k, log.Query(k))
	}
}
