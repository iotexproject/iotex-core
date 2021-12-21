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
	require.NoError(err)

	w := csv.NewWriter(f)
	err = w.WriteAll(patchTest)
	require.NoError(err)
	patch, _ := newPatchStore(filePath)
	require.Equal(len(patch.Get(1)), 1)
	require.Equal(patch.Get(1)[0].Namespace, "test")
	require.Equal(patch.Get(1)[0].Type, _Delete)
	require.Equal(string(patch.Get(1)[0].Key), "test")
	require.Equal(len(patch.Get(1)[0].Value), 0)
	require.Equal(len(patch.Get(3)), 0)
	require.NoError(f.Close())
	require.NoError(os.RemoveAll(filePath))

	// empty filePath
	patch, err = newPatchStore("")
	require.NoError(err)
	require.Equal(len(patch.patchs), 0)

	// incorrect format of patch file
	patchTest2 := []struct {
		input  [][]string
		errMsg string
	}{
		{
			input: [][]string{
				{"1", "DEL", "test", "74657374"},
			},
			errMsg: "invalid patch type",
		},
		{
			input: [][]string{
				{"1", "DEL", "test"},
			},
			errMsg: "wrong format",
		},
		{
			input: [][]string{
				{"2", "PUT", "test", "74657374", "7465737432", "12332"},
			},
			errMsg: "wrong put format",
		},
		{
			input: [][]string{
				{"1", "PUT", "test", "0x74", "0x74"},
			},
			errMsg: "failed to parse value",
		},
	}

	for _, test := range patchTest2 {
		filePath, _ = testutil.PathOfTempFile("test")
		f, err = os.Create(filePath)
		require.NoError(err)

		w = csv.NewWriter(f)
		err = w.WriteAll(test.input)
		require.NoError(err)
		_, err := newPatchStore(filePath)
		require.Error(err)
		require.Contains(err.Error(), test.errMsg)
		require.NoError(f.Close())
		require.NoError(os.RemoveAll(filePath))
	}

}
