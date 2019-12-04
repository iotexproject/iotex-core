package blockindex

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestActionIndex(t *testing.T) {
	require := require.New(t)

	ad := []*actionIndex{
		{1048000},
		{1048001},
	}

	for i := range ad {
		s := ad[i].Serialize()
		bd2 := &actionIndex{}
		require.NoError(bd2.Deserialize(s))
		require.Equal(ad[i], bd2)
	}
}

func TestBlockIndex(t *testing.T) {
	require := require.New(t)

	h, _ := hex.DecodeString("d1ff0e7fe2a54600a171d3bcc9e222c656d584b3a0e7b33373e634de3f8cd010")
	bd := []*blockIndex{
		{
			h, 1048000, big.NewInt(1048000),
		},
		{
			nil, 1048000, big.NewInt(1048000),
		},
		{
			h, 1048000, big.NewInt(0),
		},
	}

	for i := range bd {
		s := bd[i].Serialize()
		bd2 := &blockIndex{}
		require.NoError(bd2.Deserialize(s))
		require.Equal(bd[i], bd2)
	}
}
