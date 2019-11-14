package api

import (
	"github.com/golang/mock/gomock"
	"math/rand"
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
	defer ctrl.Finish()
	responder := mock_responder.NewMockResponder(ctrl)
	responder.EXPECT().Respond(gomock.Any()).Return(nil).AnyTimes()
	lister := NewChainListener()
	lister.AddResponder(responder)
	err := lister.Start()
	for _, i := range []int{1, 10, 100, 1000} {
		block1 := makeBlock(t, i)
		lister.HandleBlock(block1)
	}
	require.NoError(err)
}

func makeBlock(tb testing.TB, n int) *block.Block {
	rand.Seed(time.Now().Unix())
	sevlps := make([]action.SealedEnvelope, 0)
	for j := 1; j <= n; j++ {
		i := rand.Int()
		tsf, err := action.NewTransfer(
			uint64(i),
			unit.ConvertIotxToRau(1000+int64(i)),
			identityset.Address(i%identityset.Size()).String(),
			nil,
			20000+uint64(i),
			unit.ConvertIotxToRau(1+int64(i)),
		)
		require.NoError(tb, err)
		eb := action.EnvelopeBuilder{}
		evlp := eb.
			SetAction(tsf).
			SetGasLimit(tsf.GasLimit()).
			SetGasPrice(tsf.GasPrice()).
			SetNonce(tsf.Nonce()).
			SetVersion(1).
			Build()
		sevlp, err := action.Sign(evlp, identityset.PrivateKey((i+1)%identityset.Size()))
		require.NoError(tb, err)
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
	require.NoError(tb, err)
	return &blk
}
