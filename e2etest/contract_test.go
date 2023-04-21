package e2etest

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/stretchr/testify/require"
)

func TestContract(t *testing.T) {
	require := require.New(t)

	testReadContract := func(cfg config.Config, t *testing.T) {
		ctx := context.Background()

		// Create a new blockchain
		svr, err := itx.NewServer(cfg)
		require.NoError(err)
		require.NoError(svr.Start(ctx))
		defer func() {
			require.NoError(svr.Stop(ctx))
		}()

		chainID := cfg.Chain.ID
		bc := svr.ChainService(chainID).Blockchain()
		sf := svr.ChainService(chainID).StateFactory()
		ap := svr.ChainService(chainID).ActionPool()
		dao := svr.ChainService(chainID).BlockDAO()
		registry := svr.ChainService(chainID).Registry()
		require.NotNil(bc)
		require.NotNil(registry)
		admin := identityset.PrivateKey(26)
		state0 := hash.BytesToHash160(identityset.Address(26).Bytes())
		s := &state.Account{}
		_, err = sf.State(s, protocol.LegacyKeyOption(state0))
		require.NoError(err)
		require.Equal(unit.ConvertIotxToRau(100000000), s.Balance)

		// deploy staking contract
		data, _ := hex.DecodeString("608060405234801561001057600080fd5b50610187806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b610055600480360381019061005091906100fa565b610075565b005b61005f6100b6565b60405161006c9190610136565b60405180910390f35b806000819055507fdf7a95aebff315db1b7716215d602ab537373cdb769232aae6055c06e798425b816040516100ab9190610136565b60405180910390a150565b60008054905090565b600080fd5b6000819050919050565b6100d7816100c4565b81146100e257600080fd5b50565b6000813590506100f4816100ce565b92915050565b6000602082840312156101105761010f6100bf565b5b600061011e848285016100e5565b91505092915050565b610130816100c4565b82525050565b600060208201905061014b6000830184610127565b9291505056fea2646970667358221220f5056e078d0c1d02dd0cbf9d99c912c45a3d2078427fcf18f7f7eb1b15b263fb64736f6c63430008110033")
		fixedTime := time.Unix(cfg.Genesis.Timestamp, 0)
		ex, err := action.SignedExecution(action.EmptyAddress, admin, 1, big.NewInt(0), 10000000, big.NewInt(testutil.TestGasPriceInt64), data)
		require.NoError(err)

		deployHash, err := ex.Hash()
		require.NoError(err)
		require.NoError(ap.Add(context.Background(), ex))
		blk, err := bc.MintNewBlock(fixedTime)
		require.NoError(err)
		require.NoError(bc.CommitBlock(blk))
		r, err := dao.GetReceiptByActionHash(deployHash, 1)
		require.NoError(err)
		require.Equal(r.ContractAddress, "io123vqxxup8n3ld8jygvx729r6295pv9krjn2tjh")

		// set value
		_abi := `[{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint256","name":"x","type":"uint256"}],"name":"Set","type":"event"},{"inputs":[],"name":"get","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"x","type":"uint256"}],"name":"set","outputs":[],"stateMutability":"nonpayable","type":"function"}]`
		contractABI, err := abi.JSON(strings.NewReader(_abi))
		require.NoError(err)
		fixedAmount := unit.ConvertIotxToRau(200)
		sk := identityset.PrivateKey(26)
		nonce := uint64(0)
		data, err = contractABI.Pack("set", big.NewInt(10))
		require.NoError(err)
		require.True(len(data) > 0)
		ex, err = action.SignedExecution(r.ContractAddress, sk, nonce+2, fixedAmount, 1000000, big.NewInt(testutil.TestGasPriceInt64), data)
		require.NoError(err)
		require.NoError(ap.Add(context.Background(), ex))
		// data, err = contractABI.Pack("get")
		// require.NoError(err)
		// require.True(len(data) > 0)
		// ex, err = action.SignedExecution(r.ContractAddress, sk, nonce+2, fixedAmount, 1000000, big.NewInt(testutil.TestGasPriceInt64), data)
		// require.NoError(err)
		// require.NoError(ap.Add(context.Background(), ex))
		blk, err = bc.MintNewBlock(fixedTime)
		require.NoError(err)
		require.NoError(bc.CommitBlock(blk))
	}

	cfg := config.Default
	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)
	testBloomfilterIndexPath, err := testutil.PathOfTempFile("bloomfilterindex")
	require.NoError(err)
	testCandidateIndexPath, err := testutil.PathOfTempFile("candidateindex")
	require.NoError(err)
	testSystemLogPath, err := testutil.PathOfTempFile("systemlog")
	require.NoError(err)
	testConsensusPath, err := testutil.PathOfTempFile("consensus")
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
		testutil.CleanupPath(testBloomfilterIndexPath)
		testutil.CleanupPath(testCandidateIndexPath)
		testutil.CleanupPath(testSystemLogPath)
		testutil.CleanupPath(testConsensusPath)
		// clear the gateway
		delete(cfg.Plugins, config.GatewayPlugin)
	}()

	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.BloomfilterIndexDBPath = testBloomfilterIndexPath
	cfg.Chain.CandidateIndexDBPath = testCandidateIndexPath
	cfg.System.SystemLogDBPath = testSystemLogPath
	cfg.Consensus.RollDPoS.ConsensusDBPath = testConsensusPath
	cfg.Chain.ProducerPrivKey = "a000000000000000000000000000000000000000000000000000000000000000"
	cfg.Consensus.Scheme = config.RollDPoSScheme
	// cfg.Genesis.Blockchain = genesis.Blockchain{
	// 	Timestamp:               config.Default.Genesis.Timestamp,
	// 	BlockGasLimit:           config.Default.Genesis.BlockGasLimit,
	// 	ActionGasLimit:          config.Default.Genesis.ActionGasLimit,
	// 	BlockInterval:           config.Default.Genesis.BlockInterval,
	// 	NumSubEpochs:            uint64(config.Default.Genesis.NumSubEpochs),
	// 	DardanellesNumSubEpochs: uint64(config.Default.Genesis.DardanellesNumSubEpochs),
	// 	NumDelegates:            uint64(config.Default.Genesis.NumDelegates),
	// 	NumCandidateDelegates:   uint64(config.Default.Genesis.NumCandidateDelegates),
	// 	TimeBasedRotation:       config.Default.Genesis.TimeBasedRotation,
	// }
	cfg.Genesis.NumDelegates = 1
	cfg.Genesis.NumSubEpochs = 10
	cfg.Genesis.Delegates = []genesis.Delegate{
		{
			OperatorAddrStr: identityset.Address(0).String(),
			RewardAddrStr:   identityset.Address(0).String(),
			VotesStr:        "10",
		},
	}
	cfg.Genesis.PollMode = "lifeLong"
	cfg.Genesis.EnableGravityChainVoting = false
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.AleutianBlockHeight = 2
	cfg.Genesis.BeringBlockHeight = 0
	cfg.Genesis.HawaiiBlockHeight = 0

	t.Run("test read staking contract", func(t *testing.T) {
		testReadContract(cfg, t)
	})
}
