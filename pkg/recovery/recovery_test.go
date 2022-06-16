package recovery

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestCrashLog(t *testing.T) {
	require := require.New(t)
	heapdumpDir, err := os.MkdirTemp(os.TempDir(), "heapdump")
	require.NoError(err)
	defer testutil.CleanupPath(heapdumpDir)
	var logCfg log.GlobalConfig
	logCfg.StderrRedirectFile = &heapdumpDir

	t.Run("index out of range", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				CrashLog(r, *logCfg.StderrRedirectFile)
			}
		}()
		strs := make([]string, 2)
		strs[0] = "a"
		strs[1] = "b"
		strs[2] = "c"
	})
	t.Run("invaled memory address or nil pointer", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				CrashLog(r, *logCfg.StderrRedirectFile)
			}
		}()
		var i *int
		*i = 1
	})
	t.Run("divide by zero", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				CrashLog(r, *logCfg.StderrRedirectFile)
			}
		}()
		a, b := 10, 0
		a = a / b
	})
}

func TestPrintInfo(t *testing.T) {
	printInfo("test", func() (interface{}, error) {
		return nil, errors.New("make error")
	})
}
