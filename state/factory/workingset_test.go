package factory

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestWriteReadView(t *testing.T) {
	require := require.New(t)
	sf, err := NewFactory(config.Default, InMemTrieOption())
	require.NoError(err)
	ctx := protocol.WithBlockchainCtx(
		protocol.WithRegistry(context.Background(), protocol.NewRegistry()),
		protocol.BlockchainCtx{Genesis: config.Default.Genesis},
	)
	require.NoError(sf.Start(ctx))
	ws, err := sf.(workingSetCreator).newWorkingSet(ctx, 1)

	var retrdf interface{}
	var retErr error
	retrdf, retErr = ws.ReadView("")
	require.Equal(protocol.ErrNoName, errors.Cause(retErr))
	require.Equal(retrdf, nil)

	retrdf, retErr = ws.Unload("")
	require.Equal(protocol.ErrNoName, errors.Cause(retErr))
	require.Equal(retrdf, nil)

	require.False(ws.ProtocolDirty("test"))

	require.NoError(ws.WriteView("test", "test"))
	retrdf, retErr = ws.ReadView("test")
	require.NoError(retErr)
	require.Equal(retrdf, "test")

	require.NoError(ws.Load("test", "test"))
	retrdf, retErr = ws.Unload("test")
	require.NoError(retErr)
	require.Equal(retrdf, "test")

	require.True(ws.ProtocolDirty("test"))
}

func TestValidateBlock(t *testing.T) {
	require := require.New(t)
	sf, err := NewFactory(config.Default, InMemTrieOption())
	require.NoError(err)
	gasLimit := testutil.TestGasLimit * 100000
	zctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: uint64(1),
			Producer:    identityset.Address(27),
			GasLimit:    gasLimit,
		})
	zctx = protocol.WithBlockchainCtx(
		zctx,
		protocol.BlockchainCtx{Genesis: config.Default.Genesis},
	)
	require.NoError(sf.Start(zctx))

	digestHash := hash.Hash256b([]byte{65, 99, 99, 111, 117, 110, 116, 99, 117, 114, 114,
		101, 110, 116, 72, 101, 105, 103, 104, 116, 1, 0, 0, 0, 0, 0, 0, 0})

	t.Run("normal call", func(t *testing.T) {
		blk := makeBlock(t, 1, hash.ZeroHash256, digestHash)
		require.NoError(sf.Validate(zctx, blk))
	})

	t.Run("nonce error", func(t *testing.T) {
		blk := makeBlock(t, 3, hash.ZeroHash256, digestHash)
		require.Equal(action.ErrNonce, errors.Cause(sf.Validate(zctx, blk)))
	})

	t.Run("root hash error", func(t *testing.T) {
		blk := makeBlock(t, 1, hash.Hash256b([]byte("test")), digestHash)
		expectedErrors := errors.New("failed to validate block with workingset in factory: " +
			"Failed to verify receipt root: receipt root hash does not match")
		require.EqualError(sf.Validate(zctx, blk), expectedErrors.Error())
	})

	t.Run("delta state digest error", func(t *testing.T) {
		blk := makeBlock(t, 1, hash.ZeroHash256, hash.Hash256b([]byte("test")))
		require.Equal(block.ErrDeltaStateMismatch, errors.Cause(sf.Validate(zctx, blk)))
	})

}

func makeBlock(tb *testing.T, nonce uint64, rootHash hash.Hash256, digest hash.Hash256) *block.Block {
	rand.Seed(time.Now().Unix())
	var sevlps []action.SealedEnvelope
	r := rand.Int()
	tsf, err := action.NewTransfer(
		uint64(r),
		unit.ConvertIotxToRau(1000+int64(r)),
		identityset.Address(r%identityset.Size()).String(),
		nil,
		20000+uint64(r),
		unit.ConvertIotxToRau(1+int64(r)),
	)
	require.NoError(tb, err)
	eb := action.EnvelopeBuilder{}
	evlp := eb.
		SetAction(tsf).
		SetGasLimit(tsf.GasLimit()).
		SetGasPrice(tsf.GasPrice()).
		SetNonce(nonce).
		SetVersion(1).
		Build()
	sevlp, err := action.Sign(evlp, identityset.PrivateKey((r+1)%identityset.Size()))
	require.NoError(tb, err)
	sevlps = append(sevlps, sevlp)
	rap := block.RunnableActionsBuilder{}
	ra := rap.AddActions(sevlps...).Build()
	blk, err := block.NewBuilder(ra).
		SetHeight(1).
		SetTimestamp(time.Now()).
		SetVersion(1).
		SetReceiptRoot(rootHash).
		SetDeltaStateDigest(digest).
		SetPrevBlockHash(hash.Hash256b([]byte("test"))).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(tb, err)
	return &blk
}
