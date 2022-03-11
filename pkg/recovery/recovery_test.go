package recovery

import (
	"errors"
	"testing"
)

func TestRecovery(t *testing.T) {
	defer Recovery()
	for i := 0; i < 10; i++ {
		if i == 5 {
			panic("dump")
		}
	}
}

func TestPrintInfo(t *testing.T) {
	printInfo("test", func() (interface{}, error) {
		return nil, errors.New("make error")
	})
}
