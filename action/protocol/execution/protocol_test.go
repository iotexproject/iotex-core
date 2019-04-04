// Copyright (c) 2018 IoTeX
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
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

// ExpectedBalance defines an account-balance pair
type ExpectedBalance struct {
	Account    string `json:"account"`
	RawBalance string `json:"rawBalance"`
}

func (eb *ExpectedBalance) Balance() *big.Int {
	balance, ok := new(big.Int).SetString(eb.RawBalance, 10)
	if !ok {
		log.L().Panic("invalid balance", zap.String("balance", eb.RawBalance))
	}

	return balance
}

type ExecutionConfig struct {
	Comment                string            `json:"comment"`
	RawPrivateKey          string            `json:"rawPrivateKey"`
	RawByteCode            string            `json:"rawByteCode"`
	RawAmount              string            `json:"rawAmount"`
	RawGasLimit            uint              `json:"rawGasLimit"`
	RawGasPrice            string            `json:"rawGasPrice"`
	Failed                 bool              `json:"failed"`
	HasReturnValue         bool              `json:"hasReturnValue"`
	RawReturnValue         string            `json:"rawReturnValue"`
	RawExpectedGasConsumed uint              `json:"rawExpectedGasConsumed"`
	ExpectedBalances       []ExpectedBalance `json:"expectedBalances"`
}

func (cfg *ExecutionConfig) PrivateKey() keypair.PrivateKey {
	priKey, err := keypair.HexStringToPrivateKey(cfg.RawPrivateKey)
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
	addr, err := address.FromBytes(priKey.PublicKey().Hash())
	if err != nil {
		log.L().Panic(
			"invalid private key",
			zap.String("privateKey", cfg.RawPrivateKey),
			zap.Error(err),
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
	InitBalances []ExpectedBalance `json:"initBalances"`
	Deploy       ExecutionConfig   `json:"deploy"`
	// the order matters
	Executions []ExecutionConfig `json:"executions"`
}

func NewSmartContractTest(t *testing.T, file string) {
	require := require.New(t)
	jsonFile, err := os.Open(file)
	require.NoError(err)
	sctBytes, err := ioutil.ReadAll(jsonFile)
	require.NoError(err)
	sct := &SmartContractTest{}
	require.NoError(json.Unmarshal(sctBytes, sct))
	sct.run(require)
}

func runExecution(
	bc blockchain.Blockchain,
	ecfg *ExecutionConfig,
	contractAddr string,
) (*action.Receipt, error) {
	log.S().Info(ecfg.Comment)
	nonce, err := bc.Nonce(ecfg.Executor().String())
	if err != nil {
		return nil, err
	}
	exec, err := action.NewExecution(
		contractAddr,
		nonce+1,
		ecfg.Amount(),
		ecfg.GasLimit(),
		ecfg.GasPrice(),
		ecfg.ByteCode(),
	)
	if err != nil {
		return nil, err
	}
	builder := &action.EnvelopeBuilder{}
	elp := builder.SetAction(exec).
		SetNonce(exec.Nonce()).
		SetGasLimit(ecfg.GasLimit()).
		SetGasPrice(ecfg.GasPrice()).
		Build()
	selp, err := action.Sign(elp, ecfg.PrivateKey())
	if err != nil {
		return nil, err
	}
	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[ecfg.Executor().String()] = []action.SealedEnvelope{selp}
	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	if err != nil {
		return nil, err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return nil, err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return nil, err
	}

	return bc.GetReceiptByActionHash(exec.Hash())
}

func (sct *SmartContractTest) prepareBlockchain(
	ctx context.Context,
	r *require.Assertions,
) blockchain.Blockchain {
	cfg := config.Default
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	registry := protocol.Registry{}
	acc := account.NewProtocol()
	registry.Register(account.ProtocolID, acc)
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	registry.Register(rolldpos.ProtocolID, rp)
	bc := blockchain.NewBlockchain(
		cfg,
		blockchain.InMemDaoOption(),
		blockchain.InMemStateFactoryOption(),
		blockchain.RegistryOption(&registry),
	)
	r.NotNil(bc)
	registry.Register(vote.ProtocolID, vote.NewProtocol(bc))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	bc.Validator().AddActionValidators(account.NewProtocol(), NewProtocol(bc))
	sf := bc.GetFactory()
	r.NotNil(sf)
	sf.AddActionHandlers(NewProtocol(bc))
	r.NoError(bc.Start(ctx))
	ws, err := sf.NewWorkingSet()
	r.NoError(err)
	for _, expectedBalance := range sct.InitBalances {
		_, err = accountutil.LoadOrCreateAccount(ws, expectedBalance.Account, expectedBalance.Balance())
		r.NoError(err)
	}
	ctx = protocol.WithRunActionsCtx(ctx,
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: uint64(10000000),
		})
	_, err = ws.RunActions(ctx, 0, nil)
	r.NoError(err)
	r.NoError(sf.Commit(ws))

	return bc
}

func (sct *SmartContractTest) deployContract(
	bc blockchain.Blockchain,
	r *require.Assertions,
) string {
	receipt, err := runExecution(bc, &sct.Deploy, action.EmptyAddress)
	r.NoError(err)
	r.NotNil(receipt)
	if sct.Deploy.Failed {
		r.Equal(action.FailureReceiptStatus, receipt.Status)
		return ""
	}
	contractAddress := receipt.ContractAddress
	if sct.Deploy.ExpectedGasConsumed() != 0 {
		r.Equal(sct.Deploy.ExpectedGasConsumed(), receipt.GasConsumed)
	}

	ws, err := bc.GetFactory().NewWorkingSet()
	r.NoError(err)
	stateDB := evm.NewStateDBAdapter(bc, ws, uint64(0), hash.ZeroHash256)
	var evmContractAddrHash common.Address
	addr, _ := address.FromString(contractAddress)
	copy(evmContractAddrHash[:], addr.Bytes())
	r.True(bytes.Contains(sct.Deploy.ByteCode(), stateDB.GetCode(evmContractAddrHash)))

	return contractAddress
}

func (sct *SmartContractTest) run(r *require.Assertions) {
	// prepare blockchain
	ctx := context.Background()
	bc := sct.prepareBlockchain(ctx, r)
	defer r.NoError(bc.Stop(ctx))

	// deploy smart contract
	contractAddress := sct.deployContract(bc, r)
	if contractAddress == "" {
		return
	}

	// run executions
	for _, exec := range sct.Executions {
		receipt, err := runExecution(bc, &exec, contractAddress)
		r.NoError(err)
		r.NotNil(receipt)
		if exec.Failed {
			r.Equal(action.FailureReceiptStatus, receipt.Status)
		} else {
			r.Equal(action.SuccessReceiptStatus, receipt.Status)
		}
		if exec.HasReturnValue {
			r.Equal(exec.ExpectedReturnValue(), receipt.ReturnValue)
		}
		if exec.ExpectedGasConsumed() != 0 {
			r.Equal(exec.ExpectedGasConsumed(), receipt.GasConsumed)
		}
		for _, expectedBalance := range exec.ExpectedBalances {
			account := expectedBalance.Account
			if account == "" {
				account = contractAddress
			}
			balance, err := bc.Balance(account)
			r.NoError(err)
			r.Equal(0, balance.Cmp(expectedBalance.Balance()))
		}
	}
}

func TestProtocol_Handle(t *testing.T) {
	testEVM := func(t *testing.T) {
		log.S().Info("Test EVM")
		require := require.New(t)

		ctx := context.Background()
		cfg := config.Default

		testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
		testTriePath := testTrieFile.Name()
		testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
		testDBPath := testDBFile.Name()

		cfg.Plugins[config.GatewayPlugin] = true
		cfg.Chain.TrieDBPath = testTriePath
		cfg.Chain.ChainDBPath = testDBPath
		cfg.Chain.EnableAsyncIndexWrite = false
		registry := protocol.Registry{}
		acc := account.NewProtocol()
		registry.Register(account.ProtocolID, acc)
		rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
		registry.Register(rolldpos.ProtocolID, rp)
		bc := blockchain.NewBlockchain(
			cfg,
			blockchain.DefaultStateFactoryOption(),
			blockchain.BoltDBDaoOption(),
			blockchain.RegistryOption(&registry),
		)
		registry.Register(vote.ProtocolID, vote.NewProtocol(bc))
		bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
		bc.Validator().AddActionValidators(account.NewProtocol(), NewProtocol(bc))
		sf := bc.GetFactory()
		require.NotNil(sf)
		sf.AddActionHandlers(NewProtocol(bc))

		require.NoError(bc.Start(ctx))
		require.NotNil(bc)
		defer func() {
			err := bc.Stop(ctx)
			require.NoError(err)
		}()
		ws, err := sf.NewWorkingSet()
		require.NoError(err)
		_, err = accountutil.LoadOrCreateAccount(ws, testaddress.Addrinfo["producer"].String(), unit.ConvertIotxToRau(1000000000))
		require.NoError(err)
		gasLimit := testutil.TestGasLimit
		ctx = protocol.WithRunActionsCtx(ctx,
			protocol.RunActionsCtx{
				Producer: testaddress.Addrinfo["producer"],
				GasLimit: gasLimit,
			})
		_, err = ws.RunActions(ctx, 0, nil)
		require.NoError(err)
		require.NoError(sf.Commit(ws))

		data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
		execution, err := action.NewExecution(action.EmptyAddress, 1, big.NewInt(0), uint64(100000), big.NewInt(0), data)
		require.NoError(err)

		bd := &action.EnvelopeBuilder{}
		elp := bd.SetAction(execution).
			SetNonce(1).
			SetGasLimit(100000).Build()
		selp, err := action.Sign(elp, testaddress.Keyinfo["producer"].PriKey)
		require.NoError(err)

		actionMap := make(map[string][]action.SealedEnvelope)
		actionMap[testaddress.Addrinfo["producer"].String()] = []action.SealedEnvelope{selp}
		blk, err := bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
		)
		require.NoError(err)
		require.NoError(bc.ValidateBlock(blk))
		require.Nil(bc.CommitBlock(blk))
		require.Equal(1, len(blk.Receipts))

		eHash := execution.Hash()
		r, _ := bc.GetReceiptByActionHash(eHash)
		require.Equal(eHash, r.ActionHash)
		contract, err := address.FromString(r.ContractAddress)
		require.NoError(err)
		ws, err = sf.NewWorkingSet()
		require.NoError(err)

		stateDB := evm.NewStateDBAdapter(bc, ws, uint64(0), hash.ZeroHash256)
		var evmContractAddrHash common.Address
		copy(evmContractAddrHash[:], contract.Bytes())
		code := stateDB.GetCode(evmContractAddrHash)
		require.Nil(err)
		require.Equal(data[31:], code)

		exe, err := bc.GetActionByActionHash(eHash)
		require.Nil(err)
		require.Equal(eHash, exe.Hash())

		exes, err := bc.GetActionsFromAddress(testaddress.Addrinfo["producer"].String())
		require.Nil(err)
		require.Equal(1, len(exes))
		require.Equal(eHash, exes[0])

		blkHash, err := bc.GetBlockHashByActionHash(eHash)
		require.Nil(err)
		require.Equal(blk.HashBlock(), blkHash)

		// store to key 0
		data, _ = hex.DecodeString("60fe47b1000000000000000000000000000000000000000000000000000000000000000f")
		execution, err = action.NewExecution(r.ContractAddress, 2, big.NewInt(0), uint64(120000), big.NewInt(0), data)
		require.NoError(err)

		bd = &action.EnvelopeBuilder{}
		elp = bd.SetAction(execution).
			SetNonce(2).
			SetGasLimit(120000).Build()
		selp, err = action.Sign(elp, testaddress.Keyinfo["producer"].PriKey)
		require.NoError(err)

		log.S().Infof("execution %+v", execution)

		actionMap = make(map[string][]action.SealedEnvelope)
		actionMap[testaddress.Addrinfo["producer"].String()] = []action.SealedEnvelope{selp}
		blk, err = bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
		)
		require.NoError(err)
		require.NoError(bc.ValidateBlock(blk))
		require.Nil(bc.CommitBlock(blk))
		require.Equal(1, len(blk.Receipts))

		ws, err = sf.NewWorkingSet()
		require.NoError(err)
		stateDB = evm.NewStateDBAdapter(bc, ws, uint64(0), hash.ZeroHash256)
		var emptyEVMHash common.Hash
		v := stateDB.GetState(evmContractAddrHash, emptyEVMHash)
		require.Equal(byte(15), v[31])

		eHash = execution.Hash()
		r, _ = bc.GetReceiptByActionHash(eHash)
		require.Equal(eHash, r.ActionHash)

		// read from key 0
		data, _ = hex.DecodeString("6d4ce63c")
		execution, err = action.NewExecution(r.ContractAddress, 3, big.NewInt(0), uint64(120000), big.NewInt(0), data)
		require.NoError(err)

		bd = &action.EnvelopeBuilder{}
		elp = bd.SetAction(execution).
			SetNonce(3).
			SetGasLimit(120000).Build()
		selp, err = action.Sign(elp, testaddress.Keyinfo["producer"].PriKey)
		require.NoError(err)

		log.S().Infof("execution %+v", execution)
		actionMap = make(map[string][]action.SealedEnvelope)
		actionMap[testaddress.Addrinfo["producer"].String()] = []action.SealedEnvelope{selp}
		blk, err = bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
		)
		require.NoError(err)
		require.NoError(bc.ValidateBlock(blk))
		require.Nil(bc.CommitBlock(blk))
		require.Equal(1, len(blk.Receipts))

		eHash = execution.Hash()
		r, _ = bc.GetReceiptByActionHash(eHash)
		require.Equal(eHash, r.ActionHash)

		data, _ = hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
		execution1, err := action.NewExecution(action.EmptyAddress, 4, big.NewInt(0), uint64(100000), big.NewInt(10), data)
		require.NoError(err)
		bd = &action.EnvelopeBuilder{}

		elp = bd.SetAction(execution1).
			SetNonce(4).
			SetGasLimit(100000).SetGasPrice(big.NewInt(10)).Build()
		selp, err = action.Sign(elp, testaddress.Keyinfo["producer"].PriKey)
		require.NoError(err)

		actionMap = make(map[string][]action.SealedEnvelope)
		actionMap[testaddress.Addrinfo["producer"].String()] = []action.SealedEnvelope{selp}
		blk, err = bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
		)
		require.NoError(err)
		require.NoError(bc.ValidateBlock(blk))
		require.Nil(bc.CommitBlock(blk))
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
}

func TestProtocol_Validate(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mbc := mock_blockchain.NewMockBlockchain(ctrl)
	protocol := NewProtocol(mbc)
	// Case I: Oversized data
	tmpPayload := [32769]byte{}
	data := tmpPayload[:]
	ex, err := action.NewExecution("2", uint64(1), big.NewInt(0), uint64(0), big.NewInt(0), data)
	require.NoError(err)
	err = protocol.Validate(context.Background(), ex)
	require.Equal(action.ErrActPool, errors.Cause(err))
	// Case II: Negative amount
	ex, err = action.NewExecution("2", uint64(1), big.NewInt(-100), uint64(0), big.NewInt(0), []byte{})
	require.NoError(err)
	err = protocol.Validate(context.Background(), ex)
	require.Equal(action.ErrBalance, errors.Cause(err))
	// Case IV: Invalid contract address
	ex, err = action.NewExecution(
		testaddress.Addrinfo["bravo"].String()+"bbb",
		uint64(1),
		big.NewInt(0),
		uint64(0),
		big.NewInt(0),
		[]byte{},
	)
	require.NoError(err)
	err = protocol.Validate(context.Background(), ex)
	require.Error(err)
	require.True(strings.Contains(err.Error(), "error when validating contract's address"))
}
