package factory

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/stretchr/testify/require"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-core/action"
	"time"
	"github.com/iotexproject/iotex-core/pkg/unit"

	//"github.com/iotexproject/iotex-proto/golang/iotextypes"
	//"github.com/golang/protobuf/ptypes/timestamp"
)


func TestWriteReadView(t *testing.T){
	require := require.New(t)
	sf, err := NewFactory(config.Default, InMemTrieOption())
	require.NoError(err)
	ctx := protocol.WithBlockchainCtx(
		protocol.WithRegistry(context.Background(), protocol.NewRegistry()),
		protocol.BlockchainCtx{Genesis: config.Default.Genesis},
	)
	require.NoError(sf.Start(ctx))
	ws, err := sf.(workingSetCreator).newWorkingSet(ctx, 1)

	t.Run("ReadView call error", func(t *testing.T) {
		retrdf,retErr := ws.ReadView("")
		expectedErrors := errors.New("name : name does not exist")
		require.EqualError(retErr, expectedErrors.Error())
		require.Equal(retrdf, nil)
	})

	t.Run("WriteView ReadView normal call ", func(t *testing.T) {
		require.NoError(ws.WriteView("test","test"))
		retrdf,retErr := ws.ReadView("test")
		require.NoError(retErr)
		require.Equal(retrdf, "test")
	})

	t.Run("Unload call error", func(t *testing.T) {
		retrdf,retErr := ws.Unload("")
		expectedErrors := errors.New("name : name does not exist")
		require.EqualError(retErr, expectedErrors.Error())
		require.Equal(retrdf, nil)
	})

	t.Run("Load Unload normal call ", func(t *testing.T) {
		require.NoError(ws.Load("test","test"))
		retrdf,retErr := ws.Unload("test")
		require.NoError(retErr)
		require.Equal(retrdf, "test")
	})
}

func TestProtocolDirty(t *testing.T){
	require := require.New(t)
	sf, err := NewFactory(config.Default, InMemTrieOption())
	require.NoError(err)
	ctx := protocol.WithBlockchainCtx(
		protocol.WithRegistry(context.Background(), protocol.NewRegistry()),
		protocol.BlockchainCtx{Genesis: config.Default.Genesis},
	)
	require.NoError(sf.Start(ctx))
	ws, err := sf.(workingSetCreator).newWorkingSet(ctx, 1)
	require.False(ws.ProtocolDirty("test"))

	require.NoError(ws.Load("test","test"))
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

	t.Run("normal call", func(t *testing.T) {
		blk := makeBlock(t, 0,hash.ZeroHash256,hash.Hash256b([]byte{65,99,99,111,117,110,116,99,117,114,114,101,110,116,72,101,105,103,104,116,1,0,0,0,0,0,0,0}))
		require.NoError(sf.Validate(zctx, blk))
	})

	t.Run("nonce error", func(t *testing.T) {
		blk := makeBlock(t, 1,hash.ZeroHash256,hash.Hash256b([]byte{65,99,99,111,117,110,116,99,117,114,114,101,110,116,72,101,105,103,104,116,1,0,0,0,0,0,0,0}))
		require.NoError(sf.Validate(zctx, blk))
	})

	t.Run("root hash error", func(t *testing.T) {
		blk := makeBlock(t, 0,hash.Hash256b([]byte("test")),hash.Hash256b([]byte("test")))
		expectedErrors := errors.New("failed to validate block with workingset in factory:" +
			" failed to verify delta state digest: delta state digest doesn't match, " +
			"expected = 9c22ff5f21f0b81b113e63f7db6da94fedef11b2119b4088b89664fb9a3cb658, " +
			"actual = b672c9597b38b451ea5fc53536e3fadd197b12f035f40a26122a66b8d7fa6e3f")
		require.EqualError(sf.Validate(zctx, blk), expectedErrors.Error())
	})

	t.Run("delta state digest error", func(t *testing.T) {
		blk := makeBlock(t, 0,hash.ZeroHash256,hash.Hash256b([]byte("test")))
		expectedErrors := errors.New("failed to validate block with workingset in factory: " +
			"failed to verify delta state digest: delta state digest doesn't match," +
			" expected = 9c22ff5f21f0b81b113e63f7db6da94fedef11b2119b4088b89664fb9a3cb658, " +
			"actual = b672c9597b38b451ea5fc53536e3fadd197b12f035f40a26122a66b8d7fa6e3f")
		require.EqualError(sf.Validate(zctx, blk), expectedErrors.Error())
	})

}


func makeBlock(tb testing.TB, nonce uint64, rootHash hash.Hash256, digest hash.Hash256) *block.Block {
	rand.Seed(time.Now().Unix())
	sevlps := make([]action.SealedEnvelope, 0)
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
