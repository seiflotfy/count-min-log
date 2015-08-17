package countmin

import (
	"fmt"
	"testing"
)

func TestCountMin(t *testing.T) {

	log, err := NewDefaultLog()
	if err != nil {
		t.Error("Expected no error, go ", err)
	}

	for i := 0; i < 1000000; i++ {
		log.Update("seif")
	}
	topK := log.GetTopK()
	for k := range topK {
		fmt.Println(k, log.Query(k))
	}
	log.Update("seif")
}
