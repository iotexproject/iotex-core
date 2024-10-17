package block

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/stretchr/testify/require"
)

func TestRunnableActionsBuilder(t *testing.T) {
	require := require.New(t)
	data, err := hex.DecodeString("")
	require.NoError(err)
	v := action.NewExecution("", big.NewInt(10), data)
	ra := NewRunnableActionsBuilder()
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetGasPrice(big.NewInt(10)).SetGasLimit(uint64(100000)).
		SetAction(v).Build()

	selp, err := action.Sign(elp, identityset.PrivateKey(28))
	require.NoError(err)
	ra.AddActions(selp)
	racs := ra.Build()
	require.Equal(hash.Hash256{0x76, 0x6d, 0x8b, 0x5b, 0x98, 0xa5, 0xb2, 0xdb, 0x8d, 0x99, 0x0, 0xd2, 0x9c, 0xd1, 0x31, 0xf1, 0x59, 0xb6, 0x2f, 0x7e, 0x74, 0x6b, 0x92, 0x1b, 0x42, 0x68, 0x97, 0x4a, 0x47, 0x3e, 0x8d, 0xc5}, racs.TxHash())
	require.Equal(1, len(racs.Actions()))
	act := racs.Actions()[0]
	require.Equal(big.NewInt(10), act.GasPrice())
	require.Equal(uint64(100000), act.Gas())
}
