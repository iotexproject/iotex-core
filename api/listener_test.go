package api

import (
	"errors"
	"github.com/golang/mock/gomock"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_responder"
	"github.com/stretchr/testify/require"
)

func TestServer_Start(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	responder := mock_responder.NewMockResponder(ctrl)
	listener := NewChainListener()
	require.NoError(listener.Start())
	for _, i := range []int{1} {
		blockT, err := makeBlock(i)
		if i == 1 {
			responder.EXPECT().Respond(blockT).Return(nil).Times(1)
		} else if i == 10 {
			responder.EXPECT().Respond(blockT).Return(nil)
		} else if i == 100 {
			responder.EXPECT().Respond(blockT).Return(errors.New("testing error"))
		} else if i == 1000 {
			responder.EXPECT().Respond(blockT).Return(errors.New("testing error"))
		}
		listener.AddResponder(responder)
		listener.HandleBlock(blockT)
		require.NoError(err)
	}
}

func makeBlock(n int) (*block.Block, error) {
	sevlps := make([]action.SealedEnvelope, 0)
	for i := 1; i <= n; i++ {
		tsf, _ := action.NewTransfer(
			uint64(i),
			unit.ConvertIotxToRau(1000+int64(i)),
			identityset.Address(i%identityset.Size()).String(),
			nil,
			20000+uint64(i),
			unit.ConvertIotxToRau(1+int64(i)),
		)
		eb := action.EnvelopeBuilder{}
		evlp := eb.
			SetAction(tsf).
			SetGasLimit(tsf.GasLimit()).
			SetGasPrice(tsf.GasPrice()).
			SetNonce(tsf.Nonce()).
			SetVersion(1).
			Build()
		sevlp, _ := action.Sign(evlp, identityset.PrivateKey((i+1)%identityset.Size()))
		sevlps = append(sevlps, sevlp)
	}
	rap := block.RunnableActionsBuilder{}
	ra := rap.
		SetHeight(1).
		SetTimeStamp(time.Now()).
		AddActions(sevlps...).
		Build(identityset.PrivateKey(0).PublicKey())
	blk, err := block.NewBuilder(ra).
		SetVersion(1).
		SetReceiptRoot(hash.Hash256b([]byte("hello, world!"))).
		SetDeltaStateDigest(hash.Hash256b([]byte("world, hello!"))).
		SetPrevBlockHash(hash.Hash256b([]byte("hello, block!"))).
		SignAndBuild(identityset.PrivateKey(0))
	return &blk, err
}
