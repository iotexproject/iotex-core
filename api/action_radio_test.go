package api

import (
	"context"
	"math/big"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestActionRadio(t *testing.T) {
	r := require.New(t)
	broadcastCount := uint64(0)
	radio := NewActionRadio(
		func(_ context.Context, _ uint32, _ proto.Message) error {
			atomic.AddUint64(&broadcastCount, 1)
			return nil
		},
		0)
	r.NoError(radio.Start())
	defer func() {
		r.NoError(radio.Stop())
	}()

	gas := uint64(100000)
	gasPrice := big.NewInt(10)
	selp, err := action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(1), 1, big.NewInt(1), nil, gas, gasPrice)
	r.NoError(err)

	radio.OnAdded(selp)
	r.Equal(uint64(1), atomic.LoadUint64(&broadcastCount))
}
