// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package execution

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

type (
	// ExpectedBalance defines an account-balance pair
	ExpectedBalance struct {
		Account    string `json:"account"`
		RawBalance string `json:"rawBalance"`
	}

	ExpectedBlockInfo struct {
		TxRootHash      string `json:"txRootHash"`
		StateRootHash   string `json:"stateRootHash"`
		ReceiptRootHash string `json:"receiptRootHash"`
	}

	// GenesisBlockHeight defines an genesis blockHeight
	GenesisBlockHeight struct {
		IsBering  bool `json:"isBering"`
		IsIceland bool `json:"isIceland"`
	}

	Log struct {
		Topics []string `json:"topics"`
		Data   string   `json:"data"`
	}

	ExecutionConfig struct {
		Comment                 string            `json:"comment"`
		ContractIndex           int               `json:"contractIndex"`
		AppendContractAddress   bool              `json:"appendContractAddress"`
		ContractIndexToAppend   int               `json:"contractIndexToAppend"`
		ContractAddressToAppend string            `json:"contractAddressToAppend"`
		ReadOnly                bool              `json:"readOnly"`
		RawPrivateKey           string            `json:"rawPrivateKey"`
		RawByteCode             string            `json:"rawByteCode"`
		RawAmount               string            `json:"rawAmount"`
		RawGasLimit             uint              `json:"rawGasLimit"`
		RawGasPrice             string            `json:"rawGasPrice"`
		Failed                  bool              `json:"failed"`
		RawReturnValue          string            `json:"rawReturnValue"`
		RawExpectedGasConsumed  uint              `json:"rawExpectedGasConsumed"`
		ExpectedStatus          uint64            `json:"expectedStatus"`
		ExpectedBalances        []ExpectedBalance `json:"expectedBalances"`
		ExpectedLogs            []Log             `json:"expectedLogs"`
		ExpectedErrorMsg        string            `json:"expectedErrorMsg"`
		ExpectedBlockInfos      ExpectedBlockInfo `json:"expectedBlockInfos"`
	}
)

func (eb *ExpectedBalance) Balance() *big.Int {
	balance, ok := new(big.Int).SetString(eb.RawBalance, 10)
	if !ok {
		log.L().Panic("invalid balance", zap.String("balance", eb.RawBalance))
	}
	return balance
}

func readCode(sr protocol.StateReader, addr []byte) ([]byte, error) {
	var c protocol.SerializableBytes
	account, err := accountutil.LoadAccountByHash160(sr, hash.BytesToHash160(addr))
	if err != nil {
		return nil, err
	}
	_, err = sr.State(&c, protocol.NamespaceOption(evm.CodeKVNameSpace), protocol.KeyOption(account.CodeHash[:]))

	return c[:], err
}

func (cfg *ExecutionConfig) PrivateKey() crypto.PrivateKey {
	priKey, err := crypto.HexStringToPrivateKey(cfg.RawPrivateKey)
	if err != nil {
		log.L().Panic(
			"invalid private key",
			zap.String("privateKey", cfg.RawPrivateKey),
			zap.Error(err),
		)
	}

	return priKey
}

func (cfg *ExecutionConfig) Executor() address.Address {
	priKey := cfg.PrivateKey()
	addr := priKey.PublicKey().Address()
	if addr == nil {
		log.L().Panic(
			"invalid private key",
			zap.String("privateKey", cfg.RawPrivateKey),
			zap.Error(errors.New("failed to get address")),
		)
	}

	return addr
}

func (cfg *ExecutionConfig) ByteCode() []byte {
	byteCode, err := hex.DecodeString(cfg.RawByteCode)
	if err != nil {
		log.L().Panic(
			"invalid byte code",
			zap.String("byteCode", cfg.RawByteCode),
			zap.Error(err),
		)
	}
	if cfg.AppendContractAddress {
		addr, err := address.FromString(cfg.ContractAddressToAppend)
		if err != nil {
			log.L().Panic(
				"invalid contract address to append",
				zap.String("contractAddressToAppend", cfg.ContractAddressToAppend),
				zap.Error(err),
			)
		}
		ba := addr.Bytes()
		ba = append(make([]byte, 12), ba...)
		byteCode = append(byteCode, ba...)
	}

	return byteCode
}

func (cfg *ExecutionConfig) Amount() *big.Int {
	amount, ok := new(big.Int).SetString(cfg.RawAmount, 10)
	if !ok {
		log.L().Panic("invalid amount", zap.String("amount", cfg.RawAmount))
	}

	return amount
}

func (cfg *ExecutionConfig) GasPrice() *big.Int {
	price, ok := new(big.Int).SetString(cfg.RawGasPrice, 10)
	if !ok {
		log.L().Panic("invalid gas price", zap.String("gasPrice", cfg.RawGasPrice))
	}

	return price
}

func (cfg *ExecutionConfig) GasLimit() uint64 {
	return uint64(cfg.RawGasLimit)
}

func (cfg *ExecutionConfig) ExpectedGasConsumed() uint64 {
	return uint64(cfg.RawExpectedGasConsumed)
}

func (cfg *ExecutionConfig) ExpectedReturnValue() []byte {
	retval, err := hex.DecodeString(cfg.RawReturnValue)
	if err != nil {
		log.L().Panic(
			"invalid return value",
			zap.String("returnValue", cfg.RawReturnValue),
			zap.Error(err),
		)
	}

	return retval
}

type SmartContractTest struct {
	// the order matters
	InitGenesis  GenesisBlockHeight `json:"initGenesis"`
	InitBalances []ExpectedBalance  `json:"initBalances"`
	Deployments  []ExecutionConfig  `json:"deployments"`
	Executions   []ExecutionConfig  `json:"executions"`
}

func NewSmartContractTest(t *testing.T, file string) {
	require := require.New(t)
	jsonFile, err := os.Open(file)
	require.NoError(err)
	sctBytes, err := io.ReadAll(jsonFile)
	require.NoError(err)
	sct := &SmartContractTest{}
	require.NoError(json.Unmarshal(sctBytes, sct))
	sct.run(require)
}

func readExecution(
	bc blockchain.Blockchain,
	sf factory.Factory,
	dao blockdao.BlockDAO,
	ap actpool.ActPool,
	ecfg *ExecutionConfig,
	contractAddr string,
) ([]byte, *action.Receipt, error) {
	log.S().Info(ecfg.Comment)
	state, err := accountutil.AccountState(sf, ecfg.Executor())
	if err != nil {
		return nil, nil, err
	}
	exec, err := action.NewExecution(
		contractAddr,
		state.PendingNonce(),
		ecfg.Amount(),
		ecfg.GasLimit(),
		ecfg.GasPrice(),
		ecfg.ByteCode(),
	)
	if err != nil {
		return nil, nil, err
	}
	addr := ecfg.PrivateKey().PublicKey().Address()
	if addr == nil {
		return nil, nil, errors.New("failed to get address")
	}
	ctx, err := bc.Context(context.Background())
	if err != nil {
		return nil, nil, err
	}

	return sf.SimulateExecution(ctx, addr, exec, dao.GetBlockHash)
}

func runExecutions(
	bc blockchain.Blockchain,
	sf factory.Factory,
	dao blockdao.BlockDAO,
	ap actpool.ActPool,
	ecfgs []*ExecutionConfig,
	contractAddrs []string,
) ([]*action.Receipt, *ExpectedBlockInfo, error) {
	nonces := map[string]uint64{}
	hashes := []hash.Hash256{}
	for i, ecfg := range ecfgs {
		log.S().Info(ecfg.Comment)
		nonce := uint64(1)
		var ok bool
		executor := ecfg.Executor()
		if nonce, ok = nonces[executor.String()]; !ok {
			state, err := accountutil.AccountState(sf, executor)
			if err != nil {
				return nil, nil, err
			}
			nonce = state.PendingNonce()
		}
		nonces[executor.String()] = nonce
		exec, err := action.NewExecution(
			contractAddrs[i],
			nonce,
			ecfg.Amount(),
			ecfg.GasLimit(),
			ecfg.GasPrice(),
			ecfg.ByteCode(),
		)
		if err != nil {
			return nil, nil, err
		}
		builder := &action.EnvelopeBuilder{}
		elp := builder.SetAction(exec).
			SetNonce(exec.Nonce()).
			SetGasLimit(ecfg.GasLimit()).
			SetGasPrice(ecfg.GasPrice()).
			Build()
		selp, err := action.Sign(elp, ecfg.PrivateKey())
		if err != nil {
			return nil, nil, err
		}
		if err := ap.Add(context.Background(), selp); err != nil {
			return nil, nil, err
		}
		selpHash, err := selp.Hash()
		if err != nil {
			return nil, nil, err
		}
		hashes = append(hashes, selpHash)
	}
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return nil, nil, err
	}

	if err := bc.CommitBlock(blk); err != nil {
		return nil, nil, err
	}
	receipts := []*action.Receipt{}
	for _, hash := range hashes {
		receipt, err := dao.GetReceiptByActionHash(hash, blk.Height())
		if err != nil {
			return nil, nil, err
		}
		receipts = append(receipts, receipt)
	}
	stateRootHash, txRootHash, receiptRootHash := blk.DeltaStateDigest(), blk.TxRoot(), blk.ReceiptRoot()
	blkInfo := &ExpectedBlockInfo{
		hex.EncodeToString(txRootHash[:]),
		hex.EncodeToString(stateRootHash[:]),
		hex.EncodeToString(receiptRootHash[:]),
	}

	return receipts, blkInfo, nil
}

func (sct *SmartContractTest) prepareBlockchain(
	ctx context.Context,
	cfg config.Config,
	r *require.Assertions,
) (blockchain.Blockchain, factory.Factory, blockdao.BlockDAO, actpool.ActPool) {
	defer func() {
		delete(cfg.Plugins, config.GatewayPlugin)
	}()
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.EnableGravityChainVoting = false
	testTriePath, err := testutil.PathOfTempFile("trie")
	r.NoError(err)
	defer testutil.CleanupPath(testTriePath)

	cfg.Chain.TrieDBPath = testTriePath
	cfg.ActPool.MinGasPriceStr = "0"
	if sct.InitGenesis.IsBering {
		cfg.Genesis.Blockchain.AleutianBlockHeight = 0
		cfg.Genesis.Blockchain.BeringBlockHeight = 0
	}
	cfg.Genesis.HawaiiBlockHeight = 0
	if sct.InitGenesis.IsIceland {
		cfg.Genesis.CookBlockHeight = 0
		cfg.Genesis.DardanellesBlockHeight = 0
		cfg.Genesis.DaytonaBlockHeight = 0
		cfg.Genesis.EasterBlockHeight = 0
		cfg.Genesis.FbkMigrationBlockHeight = 0
		cfg.Genesis.FairbankBlockHeight = 0
		cfg.Genesis.GreenlandBlockHeight = 0
		cfg.Genesis.IcelandBlockHeight = 0
	}
	for _, expectedBalance := range sct.InitBalances {
		cfg.Genesis.InitBalanceMap[expectedBalance.Account] = expectedBalance.Balance().String()
	}
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	r.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	r.NoError(rp.Register(registry))
	// create state factory
	var sf factory.Factory
	if cfg.Chain.EnableTrielessStateDB {
		if cfg.Chain.EnableStateDBCaching {
			sf, err = factory.NewStateDB(cfg, factory.CachedStateDBOption(), factory.RegistryStateDBOption(registry))
		} else {
			sf, err = factory.NewStateDB(cfg, factory.DefaultStateDBOption(), factory.RegistryStateDBOption(registry))
		}
	} else {
		sf, err = factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	}
	r.NoError(err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	r.NoError(err)
	// create indexer
	indexer, err := blockindex.NewIndexer(db.NewMemKVStore(), cfg.Genesis.Hash())
	r.NoError(err)
	// create BlockDAO
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf, indexer})
	r.NotNil(dao)
	bc := blockchain.NewBlockchain(
		cfg,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	reward := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	r.NoError(reward.Register(registry))

	r.NotNil(bc)
	execution := NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
	r.NoError(execution.Register(registry))
	r.NoError(bc.Start(ctx))

	return bc, sf, dao, ap
}

func (sct *SmartContractTest) deployContracts(
	bc blockchain.Blockchain,
	sf factory.Factory,
	dao blockdao.BlockDAO,
	ap actpool.ActPool,
	r *require.Assertions,
) (contractAddresses []string) {
	for i, contract := range sct.Deployments {
		if contract.AppendContractAddress {
			contract.ContractAddressToAppend = contractAddresses[contract.ContractIndexToAppend]
		}
		receipts, _, err := runExecutions(bc, sf, dao, ap, []*ExecutionConfig{&contract}, []string{action.EmptyAddress})
		r.NoError(err)
		r.Equal(1, len(receipts))
		receipt := receipts[0]
		r.NotNil(receipt)
		if sct.InitGenesis.IsBering {
			// if it is post bering, it compares the status with expected status
			r.Equal(sct.Deployments[i].ExpectedStatus, receipt.Status)
			if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
				return []string{}
			}
		} else {
			if !sct.Deployments[i].Failed {
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status, i)
			} else {
				r.Equal(uint64(iotextypes.ReceiptStatus_Failure), receipt.Status, i)
				return []string{}
			}
		}
		if sct.Deployments[i].ExpectedGasConsumed() != 0 {
			r.Equal(sct.Deployments[i].ExpectedGasConsumed(), receipt.GasConsumed)
		}

		addr, _ := address.FromString(receipt.ContractAddress)
		c, err := readCode(sf, addr.Bytes())
		r.NoError(err)
		if contract.AppendContractAddress {
			lenOfByteCode := len(contract.ByteCode())
			r.True(bytes.Contains(contract.ByteCode()[:lenOfByteCode-32], c))
		} else {
			r.True(bytes.Contains(sct.Deployments[i].ByteCode(), c))
		}
		contractAddresses = append(contractAddresses, receipt.ContractAddress)
	}
	return
}

func (sct *SmartContractTest) run(r *require.Assertions) {
	// prepare blockchain
	ctx := context.Background()
	cfg := config.Default
	cfg.Chain.ProducerPrivKey = identityset.PrivateKey(28).HexString()
	cfg.Chain.EnableTrielessStateDB = false
	bc, sf, dao, ap := sct.prepareBlockchain(ctx, cfg, r)
	defer func() {
		r.NoError(bc.Stop(ctx))
	}()

	// deploy smart contract
	contractAddresses := sct.deployContracts(bc, sf, dao, ap, r)
	if len(contractAddresses) == 0 {
		return
	}

	// run executions
	for i, exec := range sct.Executions {
		contractAddr := contractAddresses[exec.ContractIndex]
		if exec.AppendContractAddress {
			exec.ContractAddressToAppend = contractAddresses[exec.ContractIndexToAppend]
		}
		var retval []byte
		var receipt *action.Receipt
		var blkInfo *ExpectedBlockInfo
		var err error
		if exec.ReadOnly {
			retval, receipt, err = readExecution(bc, sf, dao, ap, &exec, contractAddr)
			r.NoError(err)
			expected := exec.ExpectedReturnValue()
			if len(expected) == 0 {
				r.Equal(0, len(retval))
			} else {
				r.Equal(expected, retval)
			}
		} else {
			var receipts []*action.Receipt
			receipts, blkInfo, err = runExecutions(bc, sf, dao, ap, []*ExecutionConfig{&exec}, []string{contractAddr})
			r.NoError(err)
			r.Equal(1, len(receipts))
			receipt = receipts[0]
			r.NotNil(receipt)
		}

		if sct.InitGenesis.IsBering {
			// if it is post bering, it compares the status with expected status
			r.Equal(exec.ExpectedStatus, receipt.Status)
		} else {
			if exec.Failed {
				r.Equal(uint64(iotextypes.ReceiptStatus_Failure), receipt.Status)
			} else {
				r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
			}
		}
		if exec.ExpectedGasConsumed() != 0 {
			r.Equal(exec.ExpectedGasConsumed(), receipt.GasConsumed, i)
		}
		if exec.ExpectedBlockInfos != (ExpectedBlockInfo{}) {
			r.Equal(exec.ExpectedBlockInfos.ReceiptRootHash, blkInfo.ReceiptRootHash)
			r.Equal(exec.ExpectedBlockInfos.TxRootHash, blkInfo.TxRootHash)
			r.Equal(exec.ExpectedBlockInfos.StateRootHash, blkInfo.StateRootHash)
		}
		for _, expectedBalance := range exec.ExpectedBalances {
			account := expectedBalance.Account
			if account == "" {
				account = contractAddr
			}
			addr, err := address.FromString(account)
			r.NoError(err)
			state, err := accountutil.AccountState(sf, addr)
			r.NoError(err)
			r.Equal(
				0,
				state.Balance.Cmp(expectedBalance.Balance()),
				"balance of account %s is different from expectation, %d vs %d",
				account,
				state.Balance,
				expectedBalance.Balance(),
			)
		}
		if receipt.Status == uint64(iotextypes.ReceiptStatus_Success) {
			r.Equal(len(exec.ExpectedLogs), len(receipt.Logs()), i)
			// TODO: check value of logs
		}
		if receipt.Status == uint64(iotextypes.ReceiptStatus_ErrExecutionReverted) {
			r.Equal(exec.ExpectedErrorMsg, receipt.ExecutionRevertMsg())
		}
	}
}

func TestProtocol_Validate(t *testing.T) {
	require := require.New(t)
	p := NewProtocol(func(uint64) (hash.Hash256, error) {
		return hash.ZeroHash256, nil
	}, rewarding.DepositGas)

	ex, err := action.NewExecution("2", uint64(1), big.NewInt(0), uint64(0), big.NewInt(0), make([]byte, 32684))
	require.NoError(err)
	ex1, err := action.NewExecution(identityset.Address(29).String()+"bbb", uint64(1), big.NewInt(0), uint64(0), big.NewInt(0), nil)
	require.NoError(err)

	g := genesis.Default
	ctx := protocol.WithFeatureCtx(genesis.WithGenesisContext(protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{
		BlockHeight: g.NewfoundlandBlockHeight,
	}), g))
	for _, v := range []struct {
		ex  *action.Execution
		err error
	}{
		{ex, action.ErrOversizedData},
		{ex1, address.ErrInvalidAddr},
	} {
		require.Equal(v.err, errors.Cause(p.Validate(ctx, v.ex, nil)))
	}
}

func TestProtocol_Handle(t *testing.T) {
	testEVM := func(t *testing.T) {
		log.S().Info("Test EVM")
		require := require.New(t)

		ctx := context.Background()
		cfg := config.Default
		defer func() {
			delete(cfg.Plugins, config.GatewayPlugin)
		}()

		testTriePath, err := testutil.PathOfTempFile("trie")
		require.NoError(err)
		testDBPath, err := testutil.PathOfTempFile("db")
		require.NoError(err)
		testIndexPath, err := testutil.PathOfTempFile("index")
		require.NoError(err)
		defer func() {
			testutil.CleanupPath(testTriePath)
			testutil.CleanupPath(testDBPath)
			testutil.CleanupPath(testIndexPath)
		}()

		cfg.Plugins[config.GatewayPlugin] = true
		cfg.Chain.TrieDBPath = testTriePath
		cfg.Chain.ChainDBPath = testDBPath
		cfg.Chain.IndexDBPath = testIndexPath
		cfg.Chain.EnableAsyncIndexWrite = false
		cfg.Genesis.EnableGravityChainVoting = false
		cfg.ActPool.MinGasPriceStr = "0"
		cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(1000000000).String()
		registry := protocol.NewRegistry()
		acc := account.NewProtocol(rewarding.DepositGas)
		require.NoError(acc.Register(registry))
		rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
		require.NoError(rp.Register(registry))
		// create state factory
		sf, err := factory.NewStateDB(cfg, factory.CachedStateDBOption(), factory.RegistryStateDBOption(registry))
		require.NoError(err)
		ap, err := actpool.NewActPool(sf, cfg.ActPool)
		require.NoError(err)
		// create indexer
		cfg.DB.DbPath = cfg.Chain.IndexDBPath
		indexer, err := blockindex.NewIndexer(db.NewBoltDB(cfg.DB), hash.ZeroHash256)
		require.NoError(err)
		// create BlockDAO
		cfg.DB.DbPath = cfg.Chain.ChainDBPath
		dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf, indexer})
		require.NotNil(dao)
		bc := blockchain.NewBlockchain(
			cfg,
			dao,
			factory.NewMinter(sf, ap),
			blockchain.BlockValidatorOption(block.NewValidator(
				sf,
				protocol.NewGenericValidator(sf, accountutil.AccountState),
			)),
		)
		exeProtocol := NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
		require.NoError(exeProtocol.Register(registry))
		require.NoError(bc.Start(ctx))
		require.NotNil(bc)
		defer func() {
			require.NoError(bc.Stop(ctx))
		}()

		data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
		execution, err := action.NewExecution(action.EmptyAddress, 1, big.NewInt(0), uint64(100000), big.NewInt(0), data)
		require.NoError(err)

		bd := &action.EnvelopeBuilder{}
		elp := bd.SetAction(execution).
			SetNonce(1).
			SetGasLimit(100000).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(27))
		require.NoError(err)

		require.NoError(ap.Add(context.Background(), selp))
		blk, err := bc.MintNewBlock(testutil.TimestampNow())
		require.NoError(err)
		require.NoError(bc.CommitBlock(blk))
		require.Equal(1, len(blk.Receipts))

		eHash, err := selp.Hash()
		require.NoError(err)
		r, _ := dao.GetReceiptByActionHash(eHash, blk.Height())
		require.NotNil(r)
		require.Equal(eHash, r.ActionHash)
		contract, err := address.FromString(r.ContractAddress)
		require.NoError(err)

		// test IsContract
		state, err := accountutil.AccountState(sf, contract)
		require.NoError(err)
		require.True(state.IsContract())

		c, err := readCode(sf, contract.Bytes())
		require.NoError(err)
		require.Equal(data[31:], c)

		exe, _, err := dao.GetActionByActionHash(eHash, blk.Height())
		require.NoError(err)
		exeHash, err := exe.Hash()
		require.NoError(err)
		require.Equal(eHash, exeHash)

		addr27 := hash.BytesToHash160(identityset.Address(27).Bytes())
		total, err := indexer.GetActionCountByAddress(addr27)
		require.NoError(err)
		exes, err := indexer.GetActionsByAddress(addr27, 0, total)
		require.NoError(err)
		require.Equal(1, len(exes))
		require.Equal(eHash[:], exes[0])

		actIndex, err := indexer.GetActionIndex(eHash[:])
		require.NoError(err)
		blkHash, err := dao.GetBlockHash(actIndex.BlockHeight())
		require.NoError(err)
		require.Equal(blk.HashBlock(), blkHash)

		// store to key 0
		data, _ = hex.DecodeString("60fe47b1000000000000000000000000000000000000000000000000000000000000000f")
		execution, err = action.NewExecution(r.ContractAddress, 2, big.NewInt(0), uint64(120000), big.NewInt(0), data)
		require.NoError(err)

		bd = &action.EnvelopeBuilder{}
		elp = bd.SetAction(execution).
			SetNonce(2).
			SetGasLimit(120000).Build()
		selp, err = action.Sign(elp, identityset.PrivateKey(27))
		require.NoError(err)

		log.S().Infof("execution %+v", execution)

		require.NoError(ap.Add(context.Background(), selp))
		blk, err = bc.MintNewBlock(testutil.TimestampNow())
		require.NoError(err)
		require.NoError(bc.CommitBlock(blk))
		require.Equal(1, len(blk.Receipts))

		// TODO (zhi): reenable the unit test
		/*
			ws, err = sf.NewWorkingSet()
			require.NoError(err)
			stateDB = evm.NewStateDBAdapter(ws, uint64(0), true, hash.ZeroHash256)
			var emptyEVMHash common.Hash
			v := stateDB.GetState(evmContractAddrHash, emptyEVMHash)
			require.Equal(byte(15), v[31])
		*/
		eHash, err = selp.Hash()
		require.NoError(err)
		r, err = dao.GetReceiptByActionHash(eHash, blk.Height())
		require.NoError(err)
		require.Equal(eHash, r.ActionHash)

		// read from key 0
		data, err = hex.DecodeString("6d4ce63c")
		require.NoError(err)
		execution, err = action.NewExecution(r.ContractAddress, 3, big.NewInt(0), uint64(120000), big.NewInt(0), data)
		require.NoError(err)

		bd = &action.EnvelopeBuilder{}
		elp = bd.SetAction(execution).
			SetNonce(3).
			SetGasLimit(120000).Build()
		selp, err = action.Sign(elp, identityset.PrivateKey(27))
		require.NoError(err)

		log.S().Infof("execution %+v", execution)
		require.NoError(ap.Add(context.Background(), selp))
		blk, err = bc.MintNewBlock(testutil.TimestampNow())
		require.NoError(err)
		require.NoError(bc.CommitBlock(blk))
		require.Equal(1, len(blk.Receipts))

		eHash, err = selp.Hash()
		require.NoError(err)
		r, err = dao.GetReceiptByActionHash(eHash, blk.Height())
		require.NoError(err)
		require.Equal(eHash, r.ActionHash)

		data, _ = hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
		execution1, err := action.NewExecution(action.EmptyAddress, 4, big.NewInt(0), uint64(100000), big.NewInt(10), data)
		require.NoError(err)
		bd = &action.EnvelopeBuilder{}

		elp = bd.SetAction(execution1).
			SetNonce(4).
			SetGasLimit(100000).SetGasPrice(big.NewInt(10)).Build()
		selp, err = action.Sign(elp, identityset.PrivateKey(27))
		require.NoError(err)

		require.NoError(ap.Add(context.Background(), selp))
		blk, err = bc.MintNewBlock(testutil.TimestampNow())
		require.NoError(err)
		require.NoError(bc.CommitBlock(blk))
		require.Equal(1, len(blk.Receipts))
	}

	t.Run("EVM", func(t *testing.T) {
		testEVM(t)
	})
	/**
	 * source of smart contract: https://etherscan.io/address/0x6fb3e0a217407efff7ca062d46c26e5d60a14d69#code
	 */
	t.Run("ERC20", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/erc20.json")
	})
	/**
	 * Source of smart contract: https://etherscan.io/address/0x8dd5fbce2f6a956c3022ba3663759011dd51e73e#code
	 */
	t.Run("DelegateERC20", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/delegate_erc20.json")
	})
	/*
	 * Source code: https://kovan.etherscan.io/address/0x81f85886749cbbf3c2ec742db7255c6b07c63c69
	 */
	t.Run("InfiniteLoop", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/infiniteloop.json")
	})
	// RollDice
	t.Run("RollDice", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/rolldice.json")
	})
	// ChangeState
	t.Run("ChangeState", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/changestate.json")
	})
	// array-return
	t.Run("ArrayReturn", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/array-return.json")
	})
	// basic-token
	t.Run("BasicToken", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/basic-token.json")
	})
	// call-dynamic
	t.Run("CallDynamic", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/call-dynamic.json")
	})
	// factory
	t.Run("Factory", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/factory.json")
	})
	// mapping-delete
	t.Run("MappingDelete", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/mapping-delete.json")
	})
	// f.value
	t.Run("F.value", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/f.value.json")
	})
	// proposal
	t.Run("Proposal", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/proposal.json")
	})
	// public-length
	t.Run("PublicLength", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/public-length.json")
	})
	// public-mapping
	t.Run("PublicMapping", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/public-mapping.json")
	})
	// no-variable-length-returns
	t.Run("NoVariableLengthReturns", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/no-variable-length-returns.json")
	})
	// tuple
	t.Run("Tuple", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/tuple.json")
	})
	// tail-recursion
	t.Run("TailRecursion", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/tail-recursion.json")
	})
	// sha3
	t.Run("Sha3", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/sha3.json")
	})
	// remove-from-array
	t.Run("RemoveFromArray", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/remove-from-array.json")
	})
	// send-eth
	t.Run("SendEth", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/send-eth.json")
	})
	// modifier
	t.Run("Modifier", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/modifiers.json")
	})
	// multisend
	t.Run("Multisend", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/multisend.json")
	})
	t.Run("Multisend2", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/multisend2.json")
	})
	// reentry
	t.Run("reentry-attack", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/reentry-attack.json")
	})
	// cashier
	t.Run("cashier", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/cashier.json")
	})
	// wireconnection
	// [Issue #1422] This unit test proves that there is no problem when we want to deploy and execute the contract
	// which inherits abstract contract and implements abstract functions and call each other (Utterance() calls utterance())
	t.Run("wireconnection", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/wireconnection.json")
	})
	// gas-test
	t.Run("gas-test", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/gas-test.json")
	})
	// storage-test
	t.Run("storage-test", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/storage-test.json")
	})
	// cashier-bering
	t.Run("cashier-bering", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/cashier-bering.json")
	})
	// infiniteloop-bering
	t.Run("infiniteloop-bering", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/infiniteloop-bering.json")
	})
	// self-destruct
	t.Run("self-destruct", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/self-destruct.json")
	})
	// datacopy
	t.Run("datacopy", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/datacopy.json")
	})
	// this test replay CVE-2021-39137 attack, see attack details
	// at https://github.com/ethereum/go-ethereum/blob/master/docs/postmortems/2021-08-22-split-postmortem.md
	t.Run("CVE-2021-39137-attack-replay", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/CVE-2021-39137-attack-replay.json")
	})
}

func TestMaxTime(t *testing.T) {
	t.Run("max-time", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/maxtime.json")
	})

	t.Run("max-time-2", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/maxtime2.json")
	})
}

func TestIstanbulEVM(t *testing.T) {
	cfg := config.Default
	config.SetEVMNetworkID(cfg.Chain.EVMNetworkID)
	t.Run("ArrayReturn", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/array-return.json")
	})
	t.Run("BasicToken", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/basic-token.json")
	})
	t.Run("CallDynamic", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/call-dynamic.json")
	})
	t.Run("chainid-selfbalance", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/chainid-selfbalance.json")
	})
	t.Run("ChangeState", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/changestate.json")
	})
	t.Run("F.value", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/f.value.json")
	})
	t.Run("Gas-test", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/gas-test.json")
	})
	t.Run("InfiniteLoop", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/infiniteloop.json")
	})
	t.Run("MappingDelete", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/mapping-delete.json")
	})
	t.Run("max-time", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/maxtime.json")
	})
	t.Run("Modifier", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/modifiers.json")
	})
	t.Run("Multisend", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/multisend.json")
	})
	t.Run("NoVariableLengthReturns", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/no-variable-length-returns.json")
	})
	t.Run("PublicMapping", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/public-mapping.json")
	})
	t.Run("reentry-attack", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/reentry-attack.json")
	})
	t.Run("RemoveFromArray", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/remove-from-array.json")
	})
	t.Run("SendEth", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/send-eth.json")
	})
	t.Run("Sha3", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/sha3.json")
	})
	t.Run("storage-test", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/storage-test.json")
	})
	t.Run("TailRecursion", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/tail-recursion.json")
	})
	t.Run("Tuple", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/tuple.json")
	})
	t.Run("wireconnection", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/wireconnection.json")
	})
	t.Run("self-destruct", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/self-destruct.json")
	})
	t.Run("datacopy", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/datacopy.json")
	})
	t.Run("CVE-2021-39137-attack-replay", func(t *testing.T) {
		NewSmartContractTest(t, "testdata/CVE-2021-39137-attack-replay.json")
	})
}

func benchmarkHotContractWithFactory(b *testing.B, async bool) {
	sct := SmartContractTest{
		InitBalances: []ExpectedBalance{
			{
				Account:    "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
				RawBalance: "1000000000000000000000000000",
			},
		},
		Deployments: []ExecutionConfig{
			{
				ContractIndex: 0,
				RawPrivateKey: "cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1",
				RawByteCode:   "608060405234801561001057600080fd5b506040516040806108018339810180604052810190808051906020019092919080519060200190929190505050816004819055508060058190555050506107a58061005c6000396000f300608060405260043610610078576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680631249c58b1461007d57806327e235e31461009457806353277879146100eb5780636941b84414610142578063810ad50514610199578063a9059cbb14610223575b600080fd5b34801561008957600080fd5b50610092610270565b005b3480156100a057600080fd5b506100d5600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610475565b6040518082815260200191505060405180910390f35b3480156100f757600080fd5b5061012c600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061048d565b6040518082815260200191505060405180910390f35b34801561014e57600080fd5b50610183600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506104a5565b6040518082815260200191505060405180910390f35b3480156101a557600080fd5b506101da600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506104bd565b604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390f35b34801561022f57600080fd5b5061026e600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610501565b005b436004546000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054011115151561032a576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260108152602001807f746f6f20736f6f6e20746f206d696e740000000000000000000000000000000081525060200191505060405180910390fd5b436000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550600554600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008282540192505081905550600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600081548092919060010191905055503373ffffffffffffffffffffffffffffffffffffffff16600073ffffffffffffffffffffffffffffffffffffffff167fec61728879a33aa50b55e1f4789dcfc1c680f30a24d7b8694a9f874e242a97b46005546040518082815260200191505060405180910390a3565b60016020528060005260406000206000915090505481565b60026020528060005260406000206000915090505481565b60006020528060005260406000206000915090505481565b60036020528060005260406000206000915090508060000160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16908060010154905082565b80600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054101515156105b8576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260148152602001807f696e73756666696369656e742062616c616e636500000000000000000000000081525060200191505060405180910390fd5b80600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254039250508190555080600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254019250508190555060408051908101604052803373ffffffffffffffffffffffffffffffffffffffff16815260200182815250600360008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008201518160000160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550602082015181600101559050508173ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167fec61728879a33aa50b55e1f4789dcfc1c680f30a24d7b8694a9f874e242a97b4836040518082815260200191505060405180910390a350505600a165627a7a7230582047e5e1380e66d6b109548617ae59ff7baf70ee2d4a6734559b8fc5cabca0870b0029000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000186a0",
				RawAmount:     "0",
				RawGasLimit:   5000000,
				RawGasPrice:   "0",
			},
		},
	}
	r := require.New(b)
	ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.NumSubEpochs = uint64(b.N)
	cfg.Chain.EnableTrielessStateDB = false
	if async {
		cfg.Genesis.GreenlandBlockHeight = 0
	} else {
		cfg.Genesis.GreenlandBlockHeight = 10000000000
	}
	bc, sf, dao, ap := sct.prepareBlockchain(ctx, cfg, r)
	defer func() {
		r.NoError(bc.Stop(ctx))
	}()
	contractAddresses := sct.deployContracts(bc, sf, dao, ap, r)
	r.Equal(1, len(contractAddresses))
	contractAddr := contractAddresses[0]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		receipts, _, err := runExecutions(
			bc, sf, dao, ap, []*ExecutionConfig{
				{
					RawPrivateKey: "cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1",
					RawByteCode:   "1249c58b",
					RawAmount:     "0",
					RawGasLimit:   5000000,
					RawGasPrice:   "0",
					Failed:        false,
					Comment:       "mint token",
				},
			},
			[]string{contractAddr},
		)
		r.NoError(err)
		r.Equal(1, len(receipts))
		r.Equal(uint64(1), receipts[0].Status)
		ecfgs := []*ExecutionConfig{}
		contractAddrs := []string{}
		for j := 0; j < 100; j++ {
			ecfgs = append(ecfgs, &ExecutionConfig{
				RawPrivateKey: "cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1",
				RawByteCode:   fmt.Sprintf("a9059cbb000000000000000000000000123456789012345678900987%016x0000000000000000000000000000000000000000000000000000000000000039", 100*i+j),
				RawAmount:     "0",
				RawGasLimit:   5000000,
				RawGasPrice:   "0",
				Failed:        false,
				Comment:       "send token",
			})
			contractAddrs = append(contractAddrs, contractAddr)
		}
		receipts, _, err = runExecutions(bc, sf, dao, ap, ecfgs, contractAddrs)
		r.NoError(err)
		for _, receipt := range receipts {
			r.Equal(uint64(1), receipt.Status)
		}
	}
	b.StopTimer()
}

func benchmarkHotContractWithStateDB(b *testing.B, cachedStateDBOption bool) {
	sct := SmartContractTest{
		InitBalances: []ExpectedBalance{
			{
				Account:    "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
				RawBalance: "1000000000000000000000000000",
			},
		},
		Deployments: []ExecutionConfig{
			{
				ContractIndex: 0,
				RawPrivateKey: "cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1",
				RawByteCode:   "608060405234801561001057600080fd5b506040516040806108018339810180604052810190808051906020019092919080519060200190929190505050816004819055508060058190555050506107a58061005c6000396000f300608060405260043610610078576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680631249c58b1461007d57806327e235e31461009457806353277879146100eb5780636941b84414610142578063810ad50514610199578063a9059cbb14610223575b600080fd5b34801561008957600080fd5b50610092610270565b005b3480156100a057600080fd5b506100d5600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610475565b6040518082815260200191505060405180910390f35b3480156100f757600080fd5b5061012c600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061048d565b6040518082815260200191505060405180910390f35b34801561014e57600080fd5b50610183600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506104a5565b6040518082815260200191505060405180910390f35b3480156101a557600080fd5b506101da600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506104bd565b604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390f35b34801561022f57600080fd5b5061026e600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610501565b005b436004546000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054011115151561032a576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260108152602001807f746f6f20736f6f6e20746f206d696e740000000000000000000000000000000081525060200191505060405180910390fd5b436000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550600554600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008282540192505081905550600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600081548092919060010191905055503373ffffffffffffffffffffffffffffffffffffffff16600073ffffffffffffffffffffffffffffffffffffffff167fec61728879a33aa50b55e1f4789dcfc1c680f30a24d7b8694a9f874e242a97b46005546040518082815260200191505060405180910390a3565b60016020528060005260406000206000915090505481565b60026020528060005260406000206000915090505481565b60006020528060005260406000206000915090505481565b60036020528060005260406000206000915090508060000160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16908060010154905082565b80600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054101515156105b8576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260148152602001807f696e73756666696369656e742062616c616e636500000000000000000000000081525060200191505060405180910390fd5b80600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254039250508190555080600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254019250508190555060408051908101604052803373ffffffffffffffffffffffffffffffffffffffff16815260200182815250600360008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008201518160000160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550602082015181600101559050508173ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167fec61728879a33aa50b55e1f4789dcfc1c680f30a24d7b8694a9f874e242a97b4836040518082815260200191505060405180910390a350505600a165627a7a7230582047e5e1380e66d6b109548617ae59ff7baf70ee2d4a6734559b8fc5cabca0870b0029000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000186a0",
				RawAmount:     "0",
				RawGasLimit:   5000000,
				RawGasPrice:   "0",
			},
		},
	}
	r := require.New(b)
	ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.NumSubEpochs = uint64(b.N)
	if cachedStateDBOption {
		cfg.Chain.EnableStateDBCaching = true
	} else {
		cfg.Chain.EnableStateDBCaching = false
	}
	bc, sf, dao, ap := sct.prepareBlockchain(ctx, cfg, r)
	defer func() {
		r.NoError(bc.Stop(ctx))
	}()
	contractAddresses := sct.deployContracts(bc, sf, dao, ap, r)
	r.Equal(1, len(contractAddresses))
	contractAddr := contractAddresses[0]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		receipts, _, err := runExecutions(
			bc, sf, dao, ap, []*ExecutionConfig{
				{
					RawPrivateKey: "cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1",
					RawByteCode:   "1249c58b",
					RawAmount:     "0",
					RawGasLimit:   5000000,
					RawGasPrice:   "0",
					Failed:        false,
					Comment:       "mint token",
				},
			},
			[]string{contractAddr},
		)
		r.NoError(err)
		r.Equal(1, len(receipts))
		r.Equal(uint64(1), receipts[0].Status)
		ecfgs := []*ExecutionConfig{}
		contractAddrs := []string{}
		for j := 0; j < 100; j++ {
			ecfgs = append(ecfgs, &ExecutionConfig{
				RawPrivateKey: "cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1",
				RawByteCode:   fmt.Sprintf("a9059cbb000000000000000000000000123456789012345678900987%016x0000000000000000000000000000000000000000000000000000000000000039", 100*i+j),
				RawAmount:     "0",
				RawGasLimit:   5000000,
				RawGasPrice:   "0",
				Failed:        false,
				Comment:       "send token",
			})
			contractAddrs = append(contractAddrs, contractAddr)
		}
		receipts, _, err = runExecutions(bc, sf, dao, ap, ecfgs, contractAddrs)
		r.NoError(err)
		for _, receipt := range receipts {
			r.Equal(uint64(1), receipt.Status)
		}
	}
	b.StopTimer()
}

func BenchmarkHotContract(b *testing.B) {
	b.Run("async mode", func(b *testing.B) {
		benchmarkHotContractWithFactory(b, true)
	})
	b.Run("sync mode", func(b *testing.B) {
		benchmarkHotContractWithFactory(b, false)
	})
	b.Run("cachedStateDB", func(b *testing.B) {
		benchmarkHotContractWithStateDB(b, true)
	})
	b.Run("defaultStateDB", func(b *testing.B) {
		benchmarkHotContractWithStateDB(b, false)
	})
}
