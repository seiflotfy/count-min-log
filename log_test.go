package cml

import (
	"fmt"
	"testing"
)

func TestCountMinLog8(t *testing.T) {
	log8, err := NewDefaultSketch8()
	if err != nil {
		t.Error("Expected no error, go ", err)
	}

	for i := 0; i < 1000000; i++ {
		log8.IncreaseCount("seif")
	}

	fmt.Println(log8.GetCount("seif"))
	//fmt.Println(math.Abs(1000000 - log8.GetCount("seif")))
}

func TestCountMinLog16(t *testing.T) {
	log16, err := NewDefaultSketch16()
	if err != nil {
		t.Error("Expected no error, go ", err)
	}

	for i := 0; i < 1000000; i++ {
		log16.IncreaseCount("seif")
	}

	fmt.Println(log16.GetCount("seif"))
	//fmt.Println(log16.GetProbability("seif"))
}
