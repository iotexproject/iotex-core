// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package execution_test

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
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/blockindex"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
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
		IsBering   bool `json:"isBering"`
		IsIceland  bool `json:"isIceland"`
		IsLondon   bool `json:"isLondon"`
		IsShanghai bool `json:"isShanghai"`
		IsCancun   bool `json:"isCancun"`
	}

	Log struct {
		Topics []string `json:"topics"`
		Data   string   `json:"data"`
	}

	AccessTuple struct {
		Address     string   `json:"address"`
		StorageKeys []string `json:"storageKeys"`
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
		RawAccessList           []AccessTuple     `json:"rawAccessList"`
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

var (
	fixedTime = time.Unix(genesis.TestDefault().Timestamp, 0)
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

func (cfg *ExecutionConfig) AccessList() types.AccessList {
	if len(cfg.RawAccessList) == 0 {
		return nil
	}
	accessList := make(types.AccessList, len(cfg.RawAccessList))
	for i, rawAccessList := range cfg.RawAccessList {
		accessList[i].Address = common.HexToAddress(rawAccessList.Address)
		if numKey := len(rawAccessList.StorageKeys); numKey > 0 {
			accessList[i].StorageKeys = make([]common.Hash, numKey)
			for j, rawStorageKey := range rawAccessList.StorageKeys {
				accessList[i].StorageKeys[j] = common.HexToHash(rawStorageKey)
			}
		}
	}
	return accessList
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
	state, err := accountutil.AccountState(genesis.WithGenesisContext(context.Background(), bc.Genesis()), sf, ecfg.Executor())
	if err != nil {
		return nil, nil, err
	}
	exec := action.NewExecution(contractAddr, ecfg.Amount(), ecfg.ByteCode())
	builder := (&action.EnvelopeBuilder{}).SetGasPrice(ecfg.GasPrice()).SetGasLimit(ecfg.GasLimit()).
		SetNonce(state.PendingNonce()).SetAction(exec)
	if len(ecfg.AccessList()) > 0 {
		builder.SetTxType(action.AccessListTxType).SetAccessList(ecfg.AccessList())
	}
	elp := builder.Build()
	addr := ecfg.PrivateKey().PublicKey().Address()
	if addr == nil {
		return nil, nil, errors.New("failed to get address")
	}
	ctx, err := bc.Context(context.Background())
	if err != nil {
		return nil, nil, err
	}
	ctx = evm.WithHelperCtx(ctx, evm.HelperContext{
		GetBlockHash:   dao.GetBlockHash,
		GetBlockTime:   getBlockTimeForTest,
		DepositGasFunc: rewarding.DepositGas,
	})
	ws, err := sf.WorkingSet(ctx)
	if err != nil {
		return nil, nil, err
	}
	return evm.SimulateExecution(ctx, ws, addr, elp)
}

func (sct *SmartContractTest) runExecutions(
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
			state, err := accountutil.AccountState(genesis.WithGenesisContext(context.Background(), bc.Genesis()), sf, executor)
			if err != nil {
				return nil, nil, err
			}
			nonce = state.PendingNonce()
		}
		nonces[executor.String()] = nonce
		exec := action.NewExecution(
			contractAddrs[i],
			ecfg.Amount(),
			ecfg.ByteCode(),
		)
		builder := (&action.EnvelopeBuilder{}).SetGasLimit(ecfg.GasLimit()).SetGasPrice(ecfg.GasPrice()).
			SetNonce(nonce).SetAction(exec)
		if sct.InitGenesis.IsShanghai {
			builder.SetChainID(bc.ChainID())
		}
		if len(ecfg.AccessList()) > 0 {
			builder.SetTxType(action.AccessListTxType).SetAccessList(ecfg.AccessList())
		}
		elp := builder.Build()
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
	t, err := getBlockTimeForTest(bc.TipHeight() + 1)
	if err != nil {
		return nil, nil, err
	}
	blk, err := bc.MintNewBlock(t)
	if err != nil {
		return nil, nil, err
	}

	if err := bc.CommitBlock(blk); err != nil {
		return nil, nil, err
	}
	receipts, err := dao.GetReceipts(blk.Height())
	if err != nil {
		return nil, nil, err
	}
	receiptMap := make(map[hash.Hash256]*action.Receipt, len(receipts))
	for _, receipt := range receipts {
		receiptMap[receipt.ActionHash] = receipt
	}
	receipts = receipts[:0]
	for _, h := range hashes {
		receipt, ok := receiptMap[h]
		if !ok {
			return nil, nil, errors.Errorf("failed to find receipt for action %x", h)
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
	if sct.InitGenesis.IsLondon {
		// London is enabled at okhotsk height
		cfg.Genesis.Blockchain.JutlandBlockHeight = 0
		cfg.Genesis.Blockchain.KamchatkaBlockHeight = 0
		cfg.Genesis.Blockchain.LordHoweBlockHeight = 0
		cfg.Genesis.Blockchain.MidwayBlockHeight = 0
		cfg.Genesis.Blockchain.NewfoundlandBlockHeight = 0
		cfg.Genesis.Blockchain.OkhotskBlockHeight = 0
	}
	if sct.InitGenesis.IsShanghai {
		// Shanghai is enabled at Sumatra height
		cfg.Genesis.Blockchain.PalauBlockHeight = 0
		cfg.Genesis.Blockchain.QuebecBlockHeight = 0
		cfg.Genesis.Blockchain.RedseaBlockHeight = 0
		cfg.Genesis.Blockchain.SumatraBlockHeight = 0
		cfg.Genesis.ActionGasLimit = 10000000
	}
	if sct.InitGenesis.IsCancun {
		cfg.Genesis.Blockchain.VanuatuBlockHeight = 1
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
	var daoKV db.KVStore

	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	if cfg.Chain.EnableStateDBCaching {
		daoKV, err = db.CreateKVStoreWithCache(cfg.DB, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
	} else {
		daoKV, err = db.CreateKVStore(cfg.DB, cfg.Chain.TrieDBPath)
	}
	r.NoError(err)
	sf, err = factory.NewStateDB(factoryCfg, daoKV, factory.RegistryStateDBOption(registry))
	r.NoError(err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	r.NoError(err)
	// create indexer
	indexer, err := blockindex.NewIndexer(db.NewMemKVStore(), cfg.Genesis.Hash())
	r.NoError(err)
	// create BlockDAO
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
	reward := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	r.NoError(reward.Register(registry))

	r.NotNil(bc)
	execution := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas, getBlockTimeForTest)
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
		receipts, _, err := sct.runExecutions(bc, sf, dao, ap, []*ExecutionConfig{&contract}, []string{action.EmptyAddress})
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
	defaultCfg := config.Default
	defaultCfg.Genesis = genesis.TestDefault()
	cfg := deepcopy.Copy(defaultCfg).(config.Config)
	cfg.Chain.ProducerPrivKey = identityset.PrivateKey(28).HexString()
	bc, sf, dao, ap := sct.prepareBlockchain(ctx, cfg, r)
	defer func() {
		r.NoError(bc.Stop(ctx))
	}()
	ctx = genesis.WithGenesisContext(context.Background(), bc.Genesis())
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
			receipts, blkInfo, err = sct.runExecutions(bc, sf, dao, ap, []*ExecutionConfig{&exec}, []string{contractAddr})
			r.NoError(err)
			r.Equal(1, len(receipts))
			receipt = receipts[0]
			r.NotNil(receipt)
		}

		if sct.InitGenesis.IsBering {
			// if it is post bering, it compares the status with expected status
			r.Equal(exec.ExpectedStatus, receipt.Status, receipt.ExecutionRevertMsg())
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
			state, err := accountutil.AccountState(ctx, sf, addr)
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
	p := execution.NewProtocol(func(uint64) (hash.Hash256, error) {
		return hash.ZeroHash256, nil
	}, rewarding.DepositGas, getBlockTimeForTest)
	g := genesis.TestDefault()

	cases := []struct {
		name      string
		height    uint64
		size      uint64
		expectErr error
	}{
		{"limit 32KB", 0, 32683, nil},
		{"exceed 32KB", 0, 32684, action.ErrOversizedData},
		{"limit 48KB", g.SumatraBlockHeight, uint64(48 * 1024), nil},
		{"exceed 48KB", g.SumatraBlockHeight, uint64(48*1024) + 1, action.ErrOversizedData},
	}

	builder := action.EnvelopeBuilder{}
	for i := range cases {
		t.Run(cases[i].name, func(t *testing.T) {
			ex := action.NewExecution("2", big.NewInt(0), make([]byte, cases[i].size))
			elp := builder.SetNonce(1).SetAction(ex).Build()
			ctx := genesis.WithGenesisContext(context.Background(), g)
			ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
				BlockHeight: cases[i].height,
			})
			ctx = protocol.WithFeatureCtx(ctx)
			require.Equal(cases[i].expectErr, errors.Cause(p.Validate(ctx, elp, nil)))
		})
	}
}

func TestProtocol_Handle(t *testing.T) {
	testEVM := func(t *testing.T) {
		log.S().Info("Test EVM")
		require := require.New(t)

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
		cfg.Genesis = genesis.TestDefault()
		cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(1000000000).String()
		ctx := genesis.WithGenesisContext(context.Background(), cfg.Genesis)

		registry := protocol.NewRegistry()
		acc := account.NewProtocol(rewarding.DepositGas)
		require.NoError(acc.Register(registry))
		rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
		require.NoError(rp.Register(registry))
		factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
		db2, err := db.CreateKVStoreWithCache(cfg.DB, cfg.Chain.TrieDBPath, cfg.Chain.StateDBCacheSize)
		require.NoError(err)
		// create state factory
		sf, err := factory.NewStateDB(factoryCfg, db2, factory.RegistryStateDBOption(registry))
		require.NoError(err)
		ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
		require.NoError(err)
		// create indexer
		cfg.DB.DbPath = cfg.Chain.IndexDBPath
		indexer, err := blockindex.NewIndexer(db.NewBoltDB(cfg.DB), hash.ZeroHash256)
		require.NoError(err)
		// create BlockDAO
		cfg.DB.DbPath = cfg.Chain.ChainDBPath
		store, err := filedao.NewFileDAOInMemForTest()
		require.NoError(err)
		dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf, indexer}, cfg.DB.MaxCacheSize)
		require.NotNil(dao)
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
		exeProtocol := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas, getBlockTimeForTest)
		require.NoError(exeProtocol.Register(registry))
		require.NoError(bc.Start(ctx))
		require.NotNil(bc)
		defer func() {
			require.NoError(bc.Stop(ctx))
		}()

		data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
		execution := action.NewExecution(action.EmptyAddress, big.NewInt(0), data)
		elp := (&action.EnvelopeBuilder{}).SetAction(execution).SetNonce(1).
			SetGasLimit(100000).Build()
		selp, err := action.Sign(elp, identityset.PrivateKey(27))
		require.NoError(err)

		require.NoError(ap.Add(context.Background(), selp))
		blk, err := bc.MintNewBlock(fixedTime)
		require.NoError(err)
		require.NoError(bc.CommitBlock(blk))
		require.Equal(1, len(blk.Receipts))

		eHash, err := selp.Hash()
		require.NoError(err)
		r := blk.Receipts[0]
		require.Equal(eHash, r.ActionHash)
		contract, err := address.FromString(r.ContractAddress)
		require.NoError(err)

		// test IsContract
		state, err := accountutil.AccountState(ctx, sf, contract)
		require.NoError(err)
		require.True(state.IsContract())

		c, err := readCode(sf, contract.Bytes())
		require.NoError(err)
		require.Equal(data[31:], c)

		exe, _, err := blk.ActionByHash(eHash)
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
		execution = action.NewExecution(r.ContractAddress, big.NewInt(0), data)
		elp = (&action.EnvelopeBuilder{}).SetAction(execution).SetNonce(2).
			SetGasLimit(120000).Build()
		selp, err = action.Sign(elp, identityset.PrivateKey(27))
		require.NoError(err)

		log.S().Infof("execution %+v", execution)

		require.NoError(ap.Add(context.Background(), selp))
		blk, err = bc.MintNewBlock(fixedTime)
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
		require.Equal(eHash, blk.Receipts[0].ActionHash)

		// read from key 0
		data, err = hex.DecodeString("6d4ce63c")
		require.NoError(err)
		execution = action.NewExecution(r.ContractAddress, big.NewInt(0), data)
		elp = (&action.EnvelopeBuilder{}).SetAction(execution).SetNonce(3).
			SetGasLimit(120000).Build()
		selp, err = action.Sign(elp, identityset.PrivateKey(27))
		require.NoError(err)

		log.S().Infof("execution %+v", execution)
		require.NoError(ap.Add(context.Background(), selp))
		blk, err = bc.MintNewBlock(fixedTime)
		require.NoError(err)
		require.NoError(bc.CommitBlock(blk))
		require.Equal(1, len(blk.Receipts))

		eHash, err = selp.Hash()
		require.NoError(err)
		require.Equal(eHash, blk.Receipts[0].ActionHash)

		data, _ = hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
		execution1 := action.NewExecution(action.EmptyAddress, big.NewInt(0), data)
		elp = (&action.EnvelopeBuilder{}).SetAction(execution1).SetNonce(4).
			SetGasLimit(100000).SetGasPrice(big.NewInt(10)).Build()
		selp, err = action.Sign(elp, identityset.PrivateKey(27))
		require.NoError(err)

		require.NoError(ap.Add(context.Background(), selp))
		blk, err = bc.MintNewBlock(fixedTime)
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
		NewSmartContractTest(t, "testdata-istanbul/CVE-2021-39137-attack-replay.json")
	})
	t.Run("err-write-protection", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-istanbul/write-protection.json")
	})
	t.Run("err-write-protection-twice-delta-0", func(t *testing.T) {
		// hit errWriteProtection 2 times, delta is 0
		NewSmartContractTest(t, "testdata-istanbul/write-protection-001.json")
	})
	t.Run("err-write-protection-once-delta-0", func(t *testing.T) {
		// hit errWriteProtection 1 times, delta is 0
		NewSmartContractTest(t, "testdata-istanbul/write-protection-002.json")
	})
	t.Run("err-write-protection-twice-delta-0-0", func(t *testing.T) {
		// hit errWriteProtection twice, delta is not 0
		NewSmartContractTest(t, "testdata-istanbul/write-protection-003.json")
	})
	t.Run("err-write-protection-twice-delta-0-1", func(t *testing.T) {
		// hit errWriteProtection twice, first delta is not 0, second delta is 0
		NewSmartContractTest(t, "testdata-istanbul/write-protection-004.json")
	})
	t.Run("err-write-protection-once-delta-1", func(t *testing.T) {
		// hit errWriteProtection once, delta is not 0,but no revert
		NewSmartContractTest(t, "testdata-istanbul/write-protection-005.json")
	})
	t.Run("err-write-protection-twice-delta-1-1", func(t *testing.T) {
		// hit errWriteProtection twice,, first delta is not 0, second delta is not 0, no revert
		NewSmartContractTest(t, "testdata-istanbul/write-protection-006.json")
	})
	t.Run("err-write-protection-twice-delta-0-1", func(t *testing.T) {
		// hit errWriteProtection twice,, first delta is 0, second delta is not 0, no revert
		NewSmartContractTest(t, "testdata-istanbul/write-protection-007.json")
	})
	t.Run("err-write-protection-call-staticcall-revrt", func(t *testing.T) {
		// call -> staticcall -> revrt
		NewSmartContractTest(t, "testdata-istanbul/write-protection-008.json")
	})
	t.Run("err-write-protection-staticcall-staticcall-revrt", func(t *testing.T) {
		// staticcall -> staticcall -> revrt
		NewSmartContractTest(t, "testdata-istanbul/write-protection-009.json")
	})
	t.Run("err-write-protection-staticcall-staticcall-revrt-1", func(t *testing.T) {
		// staticcall -> staticcall -> revrt twice
		NewSmartContractTest(t, "testdata-istanbul/write-protection-010.json")
	})
}

func TestLondonEVM(t *testing.T) {
	t.Run("factory", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/factory.json")
	})
	t.Run("ArrayReturn", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/array-return.json")
	})
	t.Run("BaseFee", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/basefee.json")
	})
	t.Run("BasicToken", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/basic-token.json")
	})
	t.Run("CallDynamic", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/call-dynamic.json")
	})
	t.Run("chainid-selfbalance", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/chainid-selfbalance.json")
	})
	t.Run("ChangeState", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/changestate.json")
	})
	t.Run("F.value", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/f.value.json")
	})
	t.Run("Gas-test", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/gas-test.json")
	})
	t.Run("InfiniteLoop", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/infiniteloop.json")
	})
	t.Run("MappingDelete", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/mapping-delete.json")
	})
	t.Run("max-time", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/maxtime.json")
	})
	t.Run("Modifier", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/modifiers.json")
	})
	t.Run("Multisend", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/multisend.json")
	})
	t.Run("NoVariableLengthReturns", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/no-variable-length-returns.json")
	})
	t.Run("PublicMapping", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/public-mapping.json")
	})
	t.Run("reentry-attack", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/reentry-attack.json")
	})
	t.Run("RemoveFromArray", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/remove-from-array.json")
	})
	t.Run("SendEth", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/send-eth.json")
	})
	t.Run("Sha3", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/sha3.json")
	})
	t.Run("storage-test", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/storage-test.json")
	})
	t.Run("TailRecursion", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/tail-recursion.json")
	})
	t.Run("Tuple", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/tuple.json")
	})
	t.Run("wireconnection", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/wireconnection.json")
	})
	t.Run("self-destruct", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/self-destruct.json")
	})
	t.Run("datacopy", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/datacopy.json")
	})
	t.Run("datacopy-with-accesslist", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/datacopy-accesslist.json")
	})
	t.Run("CVE-2021-39137-attack-replay", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/CVE-2021-39137-attack-replay.json")
	})
	t.Run("difficulty", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/difficulty.json")
	})
	t.Run("push0-invalid", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-london/push0.json")
	})
}

func TestShanghaiEVM(t *testing.T) {
	t.Run("array-return", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/array-return.json")
	})
	t.Run("basefee", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/basefee.json")
	})
	t.Run("basic-token", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/basic-token.json")
	})
	t.Run("call-dynamic", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/call-dynamic.json")
	})
	t.Run("chainid-selfbalance", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/chainid-selfbalance.json")
	})
	t.Run("CVE-2021-39137-attack-replay", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/CVE-2021-39137-attack-replay.json")
	})
	t.Run("datacopy", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/datacopy.json")
	})
	t.Run("datacopy-with-accesslist", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/datacopy-accesslist.json")
	})
	t.Run("f.value", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/f.value.json")
	})
	t.Run("factory", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/factory.json")
	})
	t.Run("gas-test", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/gas-test.json")
	})
	t.Run("infiniteloop", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/infiniteloop.json")
	})
	t.Run("mapping-delete", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/mapping-delete.json")
	})
	t.Run("maxtime", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/maxtime.json")
	})
	t.Run("modifiers", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/modifiers.json")
	})
	t.Run("multisend", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/multisend.json")
	})
	t.Run("no-variable-length-returns", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/no-variable-length-returns.json")
	})
	t.Run("public-mapping", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/public-mapping.json")
	})
	t.Run("reentry-attack", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/reentry-attack.json")
	})
	t.Run("remove-from-array", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/remove-from-array.json")
	})
	t.Run("self-destruct", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/self-destruct.json")
	})
	t.Run("send-eth", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/send-eth.json")
	})
	t.Run("sha3", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/sha3.json")
	})
	t.Run("storage-test", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/storage-test.json")
	})
	t.Run("tail-recursion", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/tail-recursion.json")
	})
	t.Run("tuple", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/tuple.json")
	})
	t.Run("wireconnection", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/wireconnection.json")
	})
	t.Run("prevrandao", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/prevrandao.json")
	})
	t.Run("push0-valid", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-shanghai/push0.json")
	})
}

func TestCancunEVM(t *testing.T) {
	t.Run("eip1153-transientstorage", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-cancun/transientstorage.json")
	})
	t.Run("eip5656-mcopy", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-cancun/mcopy.json")
	})
	t.Run("eip4844-point_evaluation_precompile", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-cancun/point_evaluation.json")
	})
	t.Run("eip4844-blobhash", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-cancun/blobhash.json")
	})
	t.Run("eip7516-blobbasefee", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-cancun/blobbasefee.json")
	})
	t.Run("eip1559-basefee", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-cancun/basefee.json")
	})
	t.Run("eip6780-selfdestruct", func(t *testing.T) {
		NewSmartContractTest(t, "testdata-cancun/selfdestruct.json")
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
	cfg.Genesis = genesis.TestDefault()
	cfg.Genesis.NumSubEpochs = uint64(b.N)
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
		receipts, _, err := sct.runExecutions(
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
		receipts, _, err = sct.runExecutions(bc, sf, dao, ap, ecfgs, contractAddrs)
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
	cfg.Genesis = genesis.TestDefault()
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
		receipts, _, err := sct.runExecutions(
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
		receipts, _, err = sct.runExecutions(bc, sf, dao, ap, ecfgs, contractAddrs)
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

func getBlockTimeForTest(h uint64) (time.Time, error) {
	return fixedTime.Add(time.Duration(h) * 5 * time.Second), nil
}
