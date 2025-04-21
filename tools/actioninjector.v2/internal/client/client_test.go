package client

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/api"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/blockindex"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_factory"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestClient(t *testing.T) {
	require := require.New(t)
	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	b := identityset.Address(29).String()

	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = testutil.RandomPort()
	ctx := context.Background()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tx := action.NewTransfer(big.NewInt(10), b, nil)
	elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetAction(tx).Build()
	selp, err := action.Sign(elp, priKeyA)
	require.NoError(err)

	bc := mock_blockchain.NewMockBlockchain(mockCtrl)
	sf := mock_factory.NewMockFactory(mockCtrl)
	ap := mock_actpool.NewMockActPool(mockCtrl)

	sf.EXPECT().State(gomock.Any(), gomock.Any()).Do(func(accountState *state.Account, _ protocol.StateOption) {
	})
	sf.EXPECT().Height().Return(uint64(10), nil).AnyTimes()
	bc.EXPECT().Genesis().Return(cfg.Genesis).AnyTimes()
	bc.EXPECT().ChainID().Return(uint32(1)).AnyTimes()
	bc.EXPECT().EvmNetworkID().Return(uint32(0)).AnyTimes()
	bc.EXPECT().TipHeight().Return(uint64(4)).AnyTimes()
	bc.EXPECT().AddSubscriber(gomock.Any()).Return(nil).AnyTimes()
	bh := &iotextypes.BlockHeader{Core: &iotextypes.BlockHeaderCore{
		Version:          1,
		Height:           10,
		Timestamp:        timestamppb.Now(),
		PrevBlockHash:    []byte(""),
		TxRoot:           []byte(""),
		DeltaStateDigest: []byte(""),
		ReceiptRoot:      []byte(""),
	}, ProducerPubkey: identityset.PrivateKey(27).PublicKey().Bytes()}
	blh := block.Header{}
	require.NoError(blh.LoadFromBlockHeaderProto(bh))
	bc.EXPECT().BlockHeaderByHeight(gomock.Any()).Return(&blh, nil).AnyTimes()
	ap.EXPECT().GetPendingNonce(gomock.Any()).Return(uint64(1), nil).AnyTimes()
	ap.EXPECT().Add(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ap.EXPECT().AddSubscriber(gomock.Any()).AnyTimes()
	newOption := api.WithBroadcastOutbound(func(_ context.Context, _ uint32, _ proto.Message) error {
		return nil
	})
	indexer, err := blockindex.NewIndexer(db.NewMemKVStore(), hash.ZeroHash256)
	require.NoError(err)
	bfIndexer, err := blockindex.NewBloomfilterIndexer(db.NewMemKVStore(), cfg.Indexer)
	require.NoError(err)
	apiServer, err := api.NewServerV2(cfg.API, bc, nil, sf, nil, indexer, bfIndexer, ap, nil, nil, newOption)
	require.NoError(err)
	require.NoError(apiServer.Start(ctx))
	// test New()
	serverAddr := fmt.Sprintf("127.0.0.1:%d", cfg.API.GRPCPort)
	cli, err := New(serverAddr, true)
	require.NoError(err)

	// test GetAccount()
	response, err := cli.GetAccount(ctx, a)
	require.NotNil(response)
	require.NoError(err)

	// test SendAction
	require.NoError(cli.SendAction(ctx, selp))
}
