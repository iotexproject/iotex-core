package blockindex

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBloomRange(t *testing.T) {
	require := require.New(t)

	t.Run("unenough data", func(t *testing.T) {
		var br bloomRange
		err := br.FromBytes(make([]byte, 16))
		require.Error(err)
	})

	tests := []struct {
		start, end uint64
		success    bool
	}{
		{21, 20, false},
		{0, 8, true},
		{5, 8, true},
	}

	for _, v := range tests {
		newBr, err := newBloomRange(64, 3)
		require.NoError(err)
		newBr.BloomFilter.Add([]byte("test"))
		newBr.SetStart(v.start)
		newBr.SetEnd(v.end)

		data, err := newBr.Bytes()
		if !v.success {
			require.Error(err)
			continue
		} else {
			require.NoError(err)
		}
		err = newBr.FromBytes(data)
		require.NoError(err)
		require.Equal(v.start, newBr.Start())
		require.Equal(v.end, newBr.End())
	}
}
