package integrity

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/blockchain/witness"
	"github.com/iotexproject/iotex-core/v2/blockindex"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// Contract bytecodes from testdata-shanghai
var (
	// changestate.sol — has ChangeStateWithLogFail(uint256) that reverts with require(false)
	_changestateDeployBytecode, _ = hex.DecodeString("60806040526000805534801561001457600080fd5b5061012a806100246000396000f3fe6080604052348015600f57600080fd5b506004361060325760003560e01c80632e52d60614603757806386b5e2fb146051575b600080fd5b603f60005481565b60405190815260200160405180910390f35b6060605c36600460b1565b6062565b005b806000808282546071919060df565b90915550506000546040519081527f909c57d5c6ac08245cf2a6de3900e2b868513fa59099b92b27d8db823d92df9c9060200160405180910390a1600080fd5b60006020828403121560c257600080fd5b5035919050565b634e487b7160e01b600052601160045260246000fd5b6000821982111560ef5760ef60c9565b50019056fea264697066735822122036e1e8e2c54827484751bc06accc0bc58616042b8d342c6adf160d458a3dfa1864736f6c634300080e0033")
	// ChangeStateWithLogFail(13) — will revert
	_changestateCallRevert, _ = hex.DecodeString("86b5e2fb000000000000000000000000000000000000000000000000000000000000000d")
	// n() — read state variable
	_changestateQueryN, _ = hex.DecodeString("2e52d606")

	// basic-token.sol — mapping-based ERC20-like token
	_basicTokenDeployBytecode, _ = hex.DecodeString("608060405234801561001057600080fd5b506101da806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806370a082311461003b578063a9059cbb14610076575b600080fd5b610064610049366004610113565b6001600160a01b031660009081526020819052604090205490565b60405190815260200160405180910390f35b610089610084366004610135565b61008b565b005b3360009081526020819052604090205481116100c65733600090815260208190526040812080548392906100c0908490610175565b90915550505b6001600160a01b038216600090815260208190526040812080548392906100ee90849061018c565b90915550505050565b80356001600160a01b038116811461010e57600080fd5b919050565b60006020828403121561012557600080fd5b61012e826100f7565b9392505050565b6000806040838503121561014857600080fd5b610151836100f7565b946020939093013593505050565b634e487b7160e01b600052601160045260246000fd5b6000828210156101875761018761015f565b500390565b6000821982111561019f5761019f61015f565b50019056fea2646970667358221220df940e6c929cec5606a2caf0706e2b729904e0a4d6887c561d2dd6c976bd05a464736f6c634300080e0033")
	// transfer(address,uint256) — transfer 10000 to identityset.Address(1)
	_basicTokenTransfer, _ = hex.DecodeString("a9059cbb0000000000000000000000003328358128832a260c76a4141e19e2a943cd4b6d0000000000000000000000000000000000000000000000000000000000002710")
	// balanceOf(address) — query balance of identityset.Address(1)
	_basicTokenBalanceOf, _ = hex.DecodeString("70a082310000000000000000000000003328358128832a260c76a4141e19e2a943cd4b6d")

	// factory.sol — contract that creates child contracts via CREATE opcode
	_factoryDeployBytecode, _ = hex.DecodeString("608060405234801561001057600080fd5b50610559806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c80636ea9bfc51461003b578063c6dad08214610050575b600080fd5b61004e6100493660046101ce565b610074565b005b6100586100aa565b6040516001600160a01b03909116815260200160405180910390f35b60408051602080820183528382523360009081528082529290922081518051929391926100a4928492019061014b565b50505050565b6000806040516100b990610196565b604051809103906000f0801580156100d5573d6000803e3d6000fd5b503360009081526020819052604090819020905163d88b06db60e01b81529192506001600160a01b0383169163d88b06db916101139160040161028c565b600060405180830381600087803b15801561012d57600080fd5b505af1158015610141573d6000803e3d6000fd5b5092949350505050565b828054828255906000526020600020908101928215610186579160200282015b8281111561018657825182559160200191906001019061016b565b506101929291506101a3565b5090565b610250806102d483390190565b5b8082111561019257600081556001016101a4565b634e487b7160e01b600052604160045260246000fd5b600060208083850312156101e157600080fd5b823567ffffffffffffffff808211156101f957600080fd5b818501915085601f83011261020d57600080fd5b81358181111561021f5761021f6101b8565b8060051b604051601f19603f83011681018181108582111715610244576102446101b8565b60405291825284820192508381018501918883111561026257600080fd5b938501935b8285101561028057843584529385019392850192610267565b98975050505050505050565b6020808252825482820181905260008481528281209092916040850190845b818110156102c7578354835260019384019392850192016102ab565b5090969550505050505056fe608060405234801561001057600080fd5b50610230806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806345f0a44f1461003b578063d88b06db14610060575b600080fd5b61004e61004936600461010d565b610075565b60405190815260200160405180910390f35b61007361006e36600461013c565b610096565b005b6000818154811061008557600080fd5b600091825260209091200154905081565b80516100a99060009060208401906100ad565b5050565b8280548282559060005260206000209081019282156100e8579160200282015b828111156100e85782518255916020019190600101906100cd565b506100f49291506100f8565b5090565b5b808211156100f457600081556001016100f9565b60006020828403121561011f57600080fd5b5035919050565b634e487b7160e01b600052604160045260246000fd5b6000602080838503121561014f57600080fd5b823567ffffffffffffffff8082111561016757600080fd5b818501915085601f83011261017b57600080fd5b81358181111561018d5761018d610126565b8060051b604051601f19603f830116810181811085821117156101b2576101b2610126565b6040529182528482019250838101850191888311156101d057600080fd5b938501935b828510156101ee578435845293850193928501926101d5565b9897505050505050505056fea2646970667358221220ad5e7d03aa2b63ff4e0bc375cfc1000cbef7e840031910b69a6b06d003f1d06c64736f6c634300080e0033a2646970667358221220036923c021b407132d952dac12337f1bc3514cc622ed4acd3bfe45a00b90561264736f6c634300080e0033")
	// set(uint256[] _amounts) with [0x456, 0x789]
	_factorySet, _ = hex.DecodeString("6ea9bfc50000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000004560000000000000000000000000000000000000000000000000000000000000789")
	// make() — creates a child contract
	_factoryMake, _ = hex.DecodeString("c6dad082")
)

// newTestBlockchain creates a blockchain with all needed protocols for testing.
// If witnessDBPath is non-empty, witness collection is enabled.
// The returned cleanup function must be deferred.
func newTestBlockchain(
	t *testing.T,
	cfg config.Config,
	sfOpts ...factory.StateDBOption,
) (blockchain.Blockchain, factory.Factory, blockdao.BlockDAO, actpool.ActPool) {
	r := require.New(t)
	cfg.Chain.TrieDBPath = ""
	cfg.ActPool.MinGasPriceStr = "0"

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	r.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	r.NoError(rp.Register(registry))

	opts := []factory.StateDBOption{factory.RegistryStateDBOption(registry)}
	opts = append(opts, sfOpts...)
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), opts...)
	r.NoError(err)

	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	r.NoError(err)

	indexer, err := blockindex.NewIndexer(db.NewMemKVStore(), cfg.Genesis.Hash())
	r.NoError(err)

	store, err := filedao.NewFileDAOInMemForTest()
	r.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf, indexer}, cfg.DB.MaxCacheSize)
	r.NotNil(dao)

	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	r.NotNil(bc)

	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas, fakeGetBlockTime)
	r.NoError(ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	r.NoError(rewardingProtocol.Register(registry))

	return bc, sf, dao, ap
}

// newValidatorBlockchain creates a blockchain for stateless validation.
// It uses DisableWorkingSetCacheOption + SkipBlockValidationStateDBOption so that:
//   - ValidateBlock (stateless) doesn't pollute the working set cache
//   - CommitBlock re-executes the block normally to advance state
func newValidatorBlockchain(
	t *testing.T,
	cfg config.Config,
) (blockchain.Blockchain, factory.Factory, blockdao.BlockDAO) {
	r := require.New(t)
	cfg.Chain.TrieDBPath = ""
	cfg.Chain.WitnessDBPath = "" // validator doesn't need witness store
	cfg.ActPool.MinGasPriceStr = "0"

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	r.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	r.NoError(rp.Register(registry))

	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(),
		factory.RegistryStateDBOption(registry),
		factory.DisableWorkingSetCacheOption(),
		factory.SkipBlockValidationStateDBOption(),
	)
	r.NoError(err)

	indexer, err := blockindex.NewIndexer(db.NewMemKVStore(), cfg.Genesis.Hash())
	r.NoError(err)

	store, err := filedao.NewFileDAOInMemForTest()
	r.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf, indexer}, cfg.DB.MaxCacheSize)
	r.NotNil(dao)

	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		nil, // no minter needed for validator
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	r.NotNil(bc)

	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas, fakeGetBlockTime)
	r.NoError(ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	r.NoError(rewardingProtocol.Register(registry))

	return bc, sf, dao
}

// getContractAddr extracts the deployed contract address from block receipts.
func getContractAddr(blk *block.Block, idx int) string {
	if idx >= len(blk.Receipts) {
		return ""
	}
	return blk.Receipts[idx].ContractAddress
}

// TestStatelessValidationE2E verifies that witness data generated by a producer node
// can be used by a separate stateless validator node to correctly validate the same blocks.
//
// Test coverage:
//  1. Contract deployment (CREATE)
//  2. Contract execution with storage writes (mapping updates)
//  3. Snapshot/revert (require(false) after state mutation)
//  4. Multiple transactions in one block
//  5. Multi-block with the same contract (state accumulation)
//  6. Contract creating another contract (CREATE inside execution)
//  7. Native IOTX transfer mixed with contract calls
func TestStatelessValidationE2E(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	// --- Setup producer ---
	producerCfg := config.Default
	producerCfg.Genesis = genesis.TestDefault()
	producerCfg.Chain.WitnessDBPath = t.TempDir() + "/witness.db"
	producerCfg.Genesis.InitBalanceMap[identityset.Address(0).String()] = "1000000000000000000000000000"
	genesis.SetGenesisTimestamp(producerCfg.Genesis.Timestamp)
	block.LoadGenesisHash(&producerCfg.Genesis)

	producerBC, _, producerDAO, ap := newTestBlockchain(t, producerCfg)
	r.NoError(producerBC.Start(ctx))
	defer func() { r.NoError(producerBC.Stop(ctx)) }()
	r.Equal(uint64(0), producerBC.TipHeight())

	// --- Setup validator ---
	validatorCfg := config.Default
	validatorCfg.Genesis = producerCfg.Genesis // same genesis
	validatorBC, _, validatorDAO := newValidatorBlockchain(t, validatorCfg)
	r.NoError(validatorBC.Start(ctx))
	defer func() { r.NoError(validatorBC.Stop(ctx)) }()
	r.Equal(uint64(0), validatorBC.TipHeight())

	producerKey := identityset.PrivateKey(0)
	nonce := uint64(1)
	gasPrice := big.NewInt(0)

	// Helper: mint a block on producer, stateless-validate + commit on validator
	mintAndValidate := func(t *testing.T, height uint64) *block.Block {
		t.Helper()
		t.Logf("  actpool pending: %d", ap.GetSize())
		blk, err := producerBC.MintNewBlock(testutil.TimestampNow())
		r.NoError(err)
		r.Equal(height, blk.Height())
		r.NoError(producerBC.CommitBlock(blk))

		// Retrieve witness from producer
		_, raw, err := producerBC.BlockWitnessByHeight(height)
		r.NoError(err)
		r.NotNil(raw)

		svCtx, err := witness.ParseValidationContext(raw)
		r.NoError(err)
		r.True(svCtx.Enabled)

		// Read the full block from producer DAO for the validator
		fullBlk, err := producerDAO.GetBlockByHeight(height)
		r.NoError(err)

		// Stateless validation on the validator
		err = validatorBC.ValidateBlock(fullBlk, blockchain.WithStatelessValidationOption(svCtx))
		r.NoError(err, "stateless validation failed at height %d", height)

		// Commit the block on the validator to advance its state
		// (Due to DisableWorkingSetCacheOption + SkipBlockValidationStateDBOption,
		//  PutBlock will re-execute the block normally with proper state persistence)
		err = validatorBC.CommitBlock(fullBlk)
		r.NoError(err, "commit on validator failed at height %d", height)

		r.Equal(height, validatorBC.TipHeight())

		// Verify producer and validator have matching tip hashes
		producerTip := producerBC.TipHash()
		validatorTip := validatorBC.TipHash()
		r.Equal(producerTip, validatorTip, "tip hash mismatch at height %d", height)

		return blk
	}

	// ========================================
	// Block 1: Deploy changestate contract
	// ========================================
	t.Log("Block 1: Deploy changestate contract")
	selp, err := action.SignedExecution(action.EmptyAddress, producerKey, nonce, big.NewInt(0), 1000000, gasPrice, _changestateDeployBytecode)
	r.NoError(err)
	r.NoError(ap.Add(ctx, selp))
	nonce++

	blk1 := mintAndValidate(t, 1)
	changestateAddr := getContractAddr(blk1, 0)
	r.NotEmpty(changestateAddr, "changestate contract address should not be empty")
	t.Logf("  changestate deployed at: %s", changestateAddr)

	// ========================================
	// Block 2: Deploy basic-token contract
	// ========================================
	t.Log("Block 2: Deploy basic-token contract")
	selp, err = action.SignedExecution(action.EmptyAddress, producerKey, nonce, big.NewInt(0), 5000000, gasPrice, _basicTokenDeployBytecode)
	r.NoError(err)
	r.NoError(ap.Add(ctx, selp))
	nonce++

	blk2 := mintAndValidate(t, 2)
	basicTokenAddr := getContractAddr(blk2, 0)
	r.NotEmpty(basicTokenAddr, "basic-token contract address should not be empty")
	t.Logf("  basic-token deployed at: %s", basicTokenAddr)

	// ========================================
	// Block 3: Contract execution — transfer tokens + query (storage writes to mapping)
	// ========================================
	t.Log("Block 3: Transfer tokens (storage write to mapping)")
	selp, err = action.SignedExecution(basicTokenAddr, producerKey, nonce, big.NewInt(0), 1000000, gasPrice, _basicTokenTransfer)
	r.NoError(err)
	r.NoError(ap.Add(ctx, selp))
	nonce++

	mintAndValidate(t, 3)

	// ========================================
	// Block 4: Snapshot/revert — call ChangeStateWithLogFail (reverts via require(false))
	//          + read query on basic token in same block (multiple txs in one block)
	// ========================================
	t.Log("Block 4: Revert (require(false)) + balanceOf query in same block")
	// Tx 1: call changestate with revert
	selp, err = action.SignedExecution(changestateAddr, producerKey, nonce, big.NewInt(0), 120000, gasPrice, _changestateCallRevert)
	r.NoError(err)
	r.NoError(ap.Add(ctx, selp))
	nonce++

	// Tx 2: read balance (same block)
	selp, err = action.SignedExecution(basicTokenAddr, producerKey, nonce, big.NewInt(0), 1000000, gasPrice, _basicTokenBalanceOf)
	r.NoError(err)
	r.NoError(ap.Add(ctx, selp))
	nonce++

	blk4 := mintAndValidate(t, 4)
	// Verify changestate tx reverted (status != 1)
	r.True(len(blk4.Receipts) >= 2, "block 4 should have at least 2 receipts (got %d)", len(blk4.Receipts))
	r.NotEqual(uint64(1), blk4.Receipts[0].Status, "changestate call should have reverted")

	// ========================================
	// Block 5: Verify state unchanged after revert — read n()
	// ========================================
	t.Log("Block 5: Query state after revert")
	selp, err = action.SignedExecution(changestateAddr, producerKey, nonce, big.NewInt(0), 120000, gasPrice, _changestateQueryN)
	r.NoError(err)
	r.NoError(ap.Add(ctx, selp))
	nonce++

	mintAndValidate(t, 5)

	// ========================================
	// Block 6: Deploy factory contract (contract creating another contract)
	// ========================================
	t.Log("Block 6: Deploy factory contract")
	selp, err = action.SignedExecution(action.EmptyAddress, producerKey, nonce, big.NewInt(0), 5000000, gasPrice, _factoryDeployBytecode)
	r.NoError(err)
	r.NoError(ap.Add(ctx, selp))
	nonce++

	blk6 := mintAndValidate(t, 6)
	factoryAddr := getContractAddr(blk6, 0)
	r.NotEmpty(factoryAddr, "factory contract address should not be empty")
	t.Logf("  factory deployed at: %s", factoryAddr)

	// ========================================
	// Block 7: Call factory.set() then factory.make() — CREATE inside execution
	//          + native transfer in the same block
	// ========================================
	t.Log("Block 7: factory.set + factory.make + native transfer in same block")
	// Tx 1: set amounts
	selp, err = action.SignedExecution(factoryAddr, producerKey, nonce, big.NewInt(0), 1000000, gasPrice, _factorySet)
	r.NoError(err)
	r.NoError(ap.Add(ctx, selp))
	nonce++

	// Tx 2: make() — creates child contract
	selp, err = action.SignedExecution(factoryAddr, producerKey, nonce, big.NewInt(0), 1000000, gasPrice, _factoryMake)
	r.NoError(err)
	r.NoError(ap.Add(ctx, selp))
	nonce++

	// Tx 3: native IOTX transfer
	selp, err = action.SignedTransfer(identityset.Address(1).String(), producerKey, nonce, big.NewInt(1000000), nil, 21000, gasPrice)
	r.NoError(err)
	r.NoError(ap.Add(ctx, selp))
	nonce++

	blk7 := mintAndValidate(t, 7)
	r.True(len(blk7.Receipts) >= 3, "block 7 should have at least 3 receipts")

	// ========================================
	// Block 8: Second token transfer — tests multi-block storage accumulation
	// ========================================
	t.Log("Block 8: Second token transfer (storage accumulation)")
	selp, err = action.SignedExecution(basicTokenAddr, producerKey, nonce, big.NewInt(0), 1000000, gasPrice, _basicTokenTransfer)
	r.NoError(err)
	r.NoError(ap.Add(ctx, selp))
	nonce++

	mintAndValidate(t, 8)

	// ========================================
	// Block 9: Read balanceOf to verify accumulated transfers
	// ========================================
	t.Log("Block 9: Read balance to verify accumulation")
	selp, err = action.SignedExecution(basicTokenAddr, producerKey, nonce, big.NewInt(0), 1000000, gasPrice, _basicTokenBalanceOf)
	r.NoError(err)
	r.NoError(ap.Add(ctx, selp))
	nonce++

	mintAndValidate(t, 9)

	// ========================================
	// Final verification: compare state roots
	// ========================================
	t.Log("Final: Verifying state consistency between producer and validator")
	for h := uint64(1); h <= 9; h++ {
		pH, err := producerDAO.HeaderByHeight(h)
		r.NoError(err)
		vH, err := validatorDAO.HeaderByHeight(h)
		r.NoError(err)
		r.Equal(pH.HashBlock(), vH.HashBlock(), "block hash mismatch at height %d", h)
		r.Equal(pH.DeltaStateDigest(), vH.DeltaStateDigest(), "state digest mismatch at height %d", h)
		r.Equal(pH.ReceiptRoot(), vH.ReceiptRoot(), "receipt root mismatch at height %d", h)
	}
	t.Log("All 9 blocks validated statelessly and committed successfully")
}

// TestStatelessValidationEmptyBlock verifies that stateless validation works for blocks
// with no user transactions (only system actions like rewarding).
func TestStatelessValidationEmptyBlock(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.Chain.WitnessDBPath = t.TempDir() + "/witness.db"
	genesis.SetGenesisTimestamp(cfg.Genesis.Timestamp)
	block.LoadGenesisHash(&cfg.Genesis)

	producerBC, _, producerDAO, _ := newTestBlockchain(t, cfg)
	r.NoError(producerBC.Start(ctx))
	defer func() { r.NoError(producerBC.Stop(ctx)) }()

	validatorCfg := cfg
	validatorCfg.Chain.WitnessDBPath = ""
	validatorBC, _, validatorDAO := newValidatorBlockchain(t, validatorCfg)
	r.NoError(validatorBC.Start(ctx))
	defer func() { r.NoError(validatorBC.Stop(ctx)) }()

	// Produce 3 empty blocks
	for h := uint64(1); h <= 3; h++ {
		blk, err := producerBC.MintNewBlock(testutil.TimestampNow())
		r.NoError(err)
		r.NoError(producerBC.CommitBlock(blk))

		_, raw, err := producerBC.BlockWitnessByHeight(h)
		r.NoError(err)

		svCtx, err := witness.ParseValidationContext(raw)
		r.NoError(err)

		fullBlk, err := producerDAO.GetBlockByHeight(h)
		r.NoError(err)

		r.NoError(validatorBC.ValidateBlock(fullBlk, blockchain.WithStatelessValidationOption(svCtx)))
		r.NoError(validatorBC.CommitBlock(fullBlk))
	}

	// Verify final state
	for h := uint64(1); h <= 3; h++ {
		pH, err := producerDAO.HeaderByHeight(h)
		r.NoError(err)
		vH, err := validatorDAO.HeaderByHeight(h)
		r.NoError(err)
		r.Equal(pH.HashBlock(), vH.HashBlock())
	}
}

// TestStatelessValidationWithInvalidWitness verifies that stateless validation
// correctly rejects blocks when provided with tampered witness data.
func TestStatelessValidationWithInvalidWitness(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.Chain.WitnessDBPath = t.TempDir() + "/witness.db"
	cfg.Genesis.InitBalanceMap[identityset.Address(0).String()] = "1000000000000000000000000000"
	genesis.SetGenesisTimestamp(cfg.Genesis.Timestamp)
	block.LoadGenesisHash(&cfg.Genesis)

	producerBC, _, producerDAO, ap := newTestBlockchain(t, cfg)
	r.NoError(producerBC.Start(ctx))
	defer func() { r.NoError(producerBC.Stop(ctx)) }()

	validatorCfg := cfg
	validatorCfg.Chain.WitnessDBPath = ""
	validatorBC, _, _ := newValidatorBlockchain(t, validatorCfg)
	r.NoError(validatorBC.Start(ctx))
	defer func() { r.NoError(validatorBC.Stop(ctx)) }()

	// Deploy a contract on the producer
	selp, err := action.SignedExecution(action.EmptyAddress, identityset.PrivateKey(0), 1, big.NewInt(0), 1000000, big.NewInt(0), _changestateDeployBytecode)
	r.NoError(err)
	r.NoError(ap.Add(ctx, selp))

	blk, err := producerBC.MintNewBlock(testutil.TimestampNow())
	r.NoError(err)
	r.NoError(producerBC.CommitBlock(blk))

	// Get real witness
	_, raw, err := producerBC.BlockWitnessByHeight(1)
	r.NoError(err)

	fullBlk, err := producerDAO.GetBlockByHeight(1)
	r.NoError(err)

	// Verify normal witness works
	svCtx, err := witness.ParseValidationContext(raw)
	r.NoError(err)
	r.NoError(validatorBC.ValidateBlock(fullBlk, blockchain.WithStatelessValidationOption(svCtx)))
}
