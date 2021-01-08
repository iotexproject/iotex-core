package blockindex

import (
	"testing"

	"github.com/iotexproject/go-pkgs/bloom"
	"github.com/stretchr/testify/require"
)

func TestNewBloomRange(t *testing.T) {
	require := require.New(t)

	br, err := bloomRangeFromBytes(make([]byte, 16))
	require.Error(err)
	require.Nil(br)

	tests := []struct {
		start, end uint64
		success    bool
	}{
		{21, 20, false},
		{0, 8, true},
		{5, 8, true},
	}

	for _, v := range tests {
		bf, err := bloom.NewBloomFilter(64, 3)
		bf.Add([]byte("test"))
		require.NoError(err)

		data, err := newBloomRange(v.start, bf).SetEnd(v.end).Bytes()
		if !v.success {
			require.Error(err)
			continue
		}

		br, err = bloomRangeFromBytes(data)
		require.NoError(err)
		require.Equal(v.start, br.Start())
		require.Equal(v.end, br.End())
		require.Equal(bf, br.BloomFilter)
	}
}
