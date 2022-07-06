package recovery

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCrashLog(t *testing.T) {
	require := require.New(t)
	heapdumpDir := t.TempDir()
	require.NoError(SetCrashlogDir(heapdumpDir))

	t.Run("index out of range", func(t *testing.T) {
		defer Recover()
		strs := make([]string, 2)
		strs[0] = "a"
		strs[1] = "b"
		strs[2] = "c"
	})
	t.Run("invaled memory address or nil pointer", func(t *testing.T) {
		defer Recover()
		var i *int
		*i = 1
	})
	t.Run("divide by zero", func(t *testing.T) {
		defer Recover()
		a, b := 10, 0
		a = a / b
	})
}

func TestPrintInfo(t *testing.T) {
	printInfo("test", func() (interface{}, error) {
		return nil, errors.New("make error")
	})
}
