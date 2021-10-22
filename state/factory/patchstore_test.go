package factory

import (
	"encoding/csv"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/testutil"
)

func TestPatchStore(t *testing.T) {
	require := require.New(t)
	patchTest := [][]string{
		{"1", "DELETE", "test", "74657374"},
		{"2", "PUT", "test", "74657374", "7465737432"},
	}

	filePath, _ := testutil.PathOfTempFile("test")
	f, err := os.Create(filePath)
	defer func() {
		require.NoError(f.Close())
		require.NoError(os.RemoveAll(filePath))
	}()
	require.NoError(err)

	w := csv.NewWriter(f)
	err = w.WriteAll(patchTest)
	require.NoError(err)
	patch, _ := newPatchStore(filePath)
	require.Equal(patch.Get(1)[0].Namespace, "test")
}
