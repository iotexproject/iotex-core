package explorer

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestWeb3Api(t *testing.T) {
	require := require.New(t)
	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Explorer.Enabled = true

	testutil.CleanupPath(t, testTriePath)
	defer testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)
	defer testutil.CleanupPath(t, testDBPath)

	sf, err := state.NewFactory(&cfg, state.InMemTrieOption())
	require.Nil(err)
	require.Nil(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))
	// Disable block reward to make bookkeeping easier
	blockchain.Gen.BlockReward = big.NewInt(0)

	// create chain
	ctx := context.Background()
	bc := blockchain.NewBlockchain(&cfg, blockchain.PrecreatedStateFactoryOption(sf), blockchain.InMemDaoOption())
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	require.Nil(err)
	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)
	require.Nil(addTestingBlocks(bc))
	err = bc.Stop(ctx)
	require.NoError(err)

	svc := PublicWeb3API{
		bc: bc,
	}

	blockNumber, err := svc.IotxBlockNumber()
	require.Nil(err)
	require.Equal(int64(4), blockNumber)

	balance, err := svc.IotxGetBalance(ta.Addrinfo["charlie"].RawAddress, LatestBlockNumber)
	require.Nil(err)
	require.Equal("0x6", balance)

	count, err := svc.IotxGetTransferCount(ta.Addrinfo["charlie"].RawAddress, LatestBlockNumber)
	require.Nil(err)
	require.Equal(int64(5), count)

	count, err = svc.IotxGetVoteCount(ta.Addrinfo["charlie"].RawAddress, LatestBlockNumber)
	require.Nil(err)
	require.Equal(int64(3), count)

	count, err = svc.IotxGetExecutionCount(ta.Addrinfo["charlie"].RawAddress, LatestBlockNumber)
	require.Nil(err)
	require.Equal(int64(2), count)

	blk, err := svc.IotxGetBlockByNumber(LatestBlockNumber)
	require.Nil(err)
	require.Equal(1, len(blk.Transfers))

	blkHash, err := svc.IotxGetBlockHashByNumber(LatestBlockNumber)
	require.Nil(err)
	require.Equal(blk.Hash, blkHash)

	blk2, err := svc.IotxGetBlockByHash(blk.Hash)
	require.Nil(err)
	require.Equal(blk, blk2)

	count, err = svc.IotxGetBlockTransferCountByHash(blk.Hash)
	require.Nil(err)
	require.Equal(int64(1), count)

	count, err = svc.IotxGetBlockTransferCountByNumber(LatestBlockNumber)
	require.Nil(err)
	require.Equal(int64(1), count)

	count, err = svc.IotxGetBlockVoteCountByHash(blk.Hash)
	require.Nil(err)
	require.Equal(int64(2), count)

	count, err = svc.IotxGetBlockVoteCountByNumber(LatestBlockNumber)
	require.Nil(err)
	require.Equal(int64(2), count)

	count, err = svc.IotxGetBlockExecutionCountByHash(blk.Hash)
	require.Nil(err)
	require.Equal(int64(2), count)

	count, err = svc.IotxGetBlockExecutionCountByNumber(LatestBlockNumber)
	require.Nil(err)
	require.Equal(int64(2), count)

	transfer, err := svc.IotxGetTransferByHash(blk.Transfers[0].Hash)
	require.Nil(err)
	require.Equal(blk.Transfers[0], transfer)

	transfer, err = svc.IotxGetTransferByBlockHashAndIndex(blk.Hash, 0)
	require.Nil(err)
	require.Equal(blk.Transfers[0], transfer)

	transfer, err = svc.IotxGetTransferByBlockNumberAndIndex(LatestBlockNumber, 0)
	require.Nil(err)
	require.Equal(blk.Transfers[0], transfer)
}
