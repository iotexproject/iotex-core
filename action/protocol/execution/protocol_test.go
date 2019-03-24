// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package execution

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
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

type execCfg struct {
	executor            string
	privateKey          keypair.PrivateKey
	codeHex             string
	amount              uint64
	gasLimit            uint64
	gasPrice            int64
	hasReturnValue      bool
	expectedReturnValue interface{}
	expectedBalances    map[string]*big.Int
}

type smartContractTest struct {
	prepare map[string]*big.Int
	deploy  execCfg
	// the order matters
	executions []execCfg
}

func runExecution(
	bc blockchain.Blockchain,
	ecfg *execCfg,
	contractAddr string,
) (*action.Receipt, error) {
	code, err := hex.DecodeString(ecfg.codeHex)
	if err != nil {
		return nil, err
	}
	nonce, err := bc.Nonce(ecfg.executor)
	if err != nil {
		return nil, err
	}
	exec, err := action.NewExecution(
		contractAddr,
		nonce+1,
		big.NewInt(int64(ecfg.amount)),
		ecfg.gasLimit,
		big.NewInt(ecfg.gasPrice),
		code,
	)
	if err != nil {
		return nil, err
	}
	builder := &action.EnvelopeBuilder{}
	elp := builder.SetAction(exec).
		SetNonce(exec.Nonce()).
		SetGasLimit(exec.GasLimit()).
		Build()
	selp, err := action.Sign(elp, ecfg.privateKey)
	if err != nil {
		return nil, err
	}
	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[ecfg.executor] = []action.SealedEnvelope{selp}
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

func (sct *smartContractTest) prepareBlockchain(
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
	for acct, supply := range sct.prepare {
		_, err = accountutil.LoadOrCreateAccount(ws, acct, supply)
		r.NoError(err)
	}

	gasLimit := uint64(10000000)
	ctx = protocol.WithRunActionsCtx(ctx,
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, nil)
	r.NoError(err)
	r.NoError(sf.Commit(ws))

	return bc
}

func (sct *smartContractTest) deployContract(
	bc blockchain.Blockchain,
	r *require.Assertions,
) string {
	receipt, err := runExecution(bc, &sct.deploy, action.EmptyAddress)
	r.NoError(err)
	r.NotNil(receipt)
	contractAddress := receipt.ContractAddress

	ws, err := bc.GetFactory().NewWorkingSet()
	r.NoError(err)
	stateDB := evm.NewStateDBAdapter(bc, ws, uint64(0), hash.ZeroHash256)
	var evmContractAddrHash common.Address
	addr, _ := address.FromString(contractAddress)
	copy(evmContractAddrHash[:], addr.Bytes())
	code := stateDB.GetCode(evmContractAddrHash)
	codeHex := hex.EncodeToString(code)
	r.True(strings.Contains(sct.deploy.codeHex, codeHex))

	return contractAddress
}

func (sct *smartContractTest) run(r *require.Assertions) {
	// prepare blockchain
	ctx := context.Background()
	bc := sct.prepareBlockchain(ctx, r)
	defer r.NoError(bc.Stop(ctx))

	// deploy smart contract
	contractAddress := sct.deployContract(bc, r)

	// run executions
	for _, exec := range sct.executions {
		receipt, err := runExecution(bc, &exec, contractAddress)
		r.NoError(err)
		r.NotNil(receipt)
		if exec.hasReturnValue {
			r.Equal(exec.expectedReturnValue, receipt.ReturnValue)
		}
		for addr, expectedBalance := range exec.expectedBalances {
			balance, err := bc.Balance(addr)
			r.NoError(err)
			r.Equal(0, balance.Cmp(expectedBalance))
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
		require.Equal(eHash, r.ActHash)
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
		require.Equal(eHash, r.ActHash)

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
		require.Equal(eHash, r.ActHash)

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

	testRollDice := func(t *testing.T) {
		log.S().Warn("======= Test RollDice")
		require := require.New(t)

		testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
		testTriePath := testTrieFile.Name()
		testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
		testDBPath := testDBFile.Name()

		ctx := context.Background()
		cfg := config.Default
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
		_, err = accountutil.LoadOrCreateAccount(ws, testaddress.Addrinfo["alfa"].String(), big.NewInt(0))
		require.NoError(err)
		_, err = accountutil.LoadOrCreateAccount(ws, testaddress.Addrinfo["bravo"].String(), big.NewInt(12000000))
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

		data, _ := hex.DecodeString("608060405234801561001057600080fd5b506102f5806100206000396000f3006080604052600436106100615763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416632885ad2c8114610066578063797d9fbd14610070578063cd5e3c5d14610091578063d0e30db0146100b8575b600080fd5b61006e6100c0565b005b61006e73ffffffffffffffffffffffffffffffffffffffff600435166100cb565b34801561009d57600080fd5b506100a6610159565b60408051918252519081900360200190f35b61006e610229565b6100c9336100cb565b565b60006100d5610159565b6040805182815290519192507fbae72e55df73720e0f671f4d20a331df0c0dc31092fda6c573f35ff7f37f283e919081900360200190a160405173ffffffffffffffffffffffffffffffffffffffff8316906305f5e100830280156108fc02916000818181858888f19350505050158015610154573d6000803e3d6000fd5b505050565b604080514460208083019190915260001943014082840152825180830384018152606090920192839052815160009360059361021a9360029391929182918401908083835b602083106101bd5780518252601f19909201916020918201910161019e565b51815160209384036101000a600019018019909216911617905260405191909301945091925050808303816000865af11580156101fe573d6000803e3d6000fd5b5050506040513d602081101561021357600080fd5b5051610261565b81151561022357fe5b06905090565b60408051348152905133917fe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c919081900360200190a2565b600080805b60208110156102c25780600101602060ff160360080260020a848260208110151561028d57fe5b7f010000000000000000000000000000000000000000000000000000000000000091901a810204029190910190600101610266565b50929150505600a165627a7a72305820a426929891673b0a04d7163b60113d28e7d0f48ea667680ba48126c182b872c10029")
		execution, err := action.NewExecution(action.EmptyAddress, 1, big.NewInt(0), uint64(1000000), big.NewInt(0), data)
		require.NoError(err)

		bd := &action.EnvelopeBuilder{}
		elp := bd.SetAction(execution).
			SetNonce(1).
			SetGasLimit(1000000).Build()
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

		log.S().Info("======= Test RollDice")
		eHash := execution.Hash()
		r, _ := bc.GetReceiptByActionHash(eHash)
		require.Equal(eHash, r.ActHash)
		contractAddr := r.ContractAddress
		data, _ = hex.DecodeString("d0e30db0")
		execution, err = action.NewExecution(contractAddr, 2, big.NewInt(500000000), uint64(120000), big.NewInt(0), data)
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

		balance, err := bc.Balance(contractAddr)
		require.NoError(err)
		require.Equal(0, balance.Cmp(big.NewInt(500000000)))

		log.S().Info("Roll Dice")
		h := testaddress.Keyinfo["alfa"].PubKey.Hash()
		data, _ = hex.DecodeString(fmt.Sprintf("797d9fbd000000000000000000000000%x", h))
		execution, err = action.NewExecution(contractAddr, 3, big.NewInt(0), uint64(120000), big.NewInt(0), data)
		require.NoError(err)

		bd = &action.EnvelopeBuilder{}
		elp = bd.SetAction(execution).
			SetNonce(3).
			SetGasLimit(120000).Build()
		selp, err = action.Sign(elp, testaddress.Keyinfo["producer"].PriKey)
		require.NoError(err)
		log.S().Infof("execution %+v\n", execution)

		actionMap = make(map[string][]action.SealedEnvelope)
		actionMap[testaddress.Addrinfo["producer"].String()] = []action.SealedEnvelope{selp}
		blk, err = bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
		)
		require.NoError(err)
		require.NoError(bc.ValidateBlock(blk))
		require.Nil(bc.CommitBlock(blk))

		// verify balance
		balance, err = bc.Balance(testaddress.Addrinfo["alfa"].String())
		require.NoError(err)
		require.Equal(0, balance.Cmp(big.NewInt(100000000)))

		balance, err = bc.Balance(contractAddr)
		require.NoError(err)
		require.Equal(0, balance.Cmp(big.NewInt(400000000)))

		log.S().Info("Roll Dice To Self")
		balance, err = bc.Balance(testaddress.Addrinfo["bravo"].String())
		require.NoError(err)
		data, _ = hex.DecodeString("2885ad2c")
		execution, err = action.NewExecution(contractAddr, 1, big.NewInt(0), uint64(120000), big.NewInt(10), data)
		require.NoError(err)

		bd = &action.EnvelopeBuilder{}
		elp = bd.SetAction(execution).
			SetNonce(1).
			SetGasLimit(120000).SetGasPrice(big.NewInt(10)).Build()
		selp, err = action.Sign(elp, testaddress.Keyinfo["bravo"].PriKey)
		require.NoError(err)
		log.S().Infof("execution %+v\n", execution)

		actionMap = make(map[string][]action.SealedEnvelope)
		actionMap[testaddress.Addrinfo["bravo"].String()] = []action.SealedEnvelope{selp}
		blk, err = bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
		)
		require.NoError(err)
		require.NoError(bc.ValidateBlock(blk))
		require.Nil(bc.CommitBlock(blk))
		balance, err = bc.Balance(testaddress.Addrinfo["bravo"].String())
		require.NoError(err)
		log.S().Infof("balance: %d", balance)
		require.Equal(0, balance.Cmp(big.NewInt(12000000+100000000-274950)))
	}

	testERC20 := func(t *testing.T) {
		require := require.New(t)

		testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
		testTriePath := testTrieFile.Name()
		testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
		testDBPath := testDBFile.Name()

		ctx := context.Background()
		cfg := config.Default
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
		require.NoError(bc.Start(ctx))
		require.NotNil(bc)
		defer func() {
			err := bc.Stop(ctx)
			require.NoError(err)
		}()
		sf := bc.GetFactory()
		require.NotNil(sf)
		sf.AddActionHandlers(NewProtocol(bc))
		ws, err := sf.NewWorkingSet()
		require.NoError(err)
		_, err = accountutil.LoadOrCreateAccount(ws, testaddress.Addrinfo["producer"].String(), unit.ConvertIotxToRau(1000000000))
		require.NoError(err)
		_, err = accountutil.LoadOrCreateAccount(ws, testaddress.Addrinfo["alfa"].String(), big.NewInt(0))
		require.NoError(err)
		_, err = accountutil.LoadOrCreateAccount(ws, testaddress.Addrinfo["bravo"].String(), big.NewInt(0))
		require.NoError(err)
		gasLimit := uint64(10000000)
		ctx = protocol.WithRunActionsCtx(ctx,
			protocol.RunActionsCtx{
				Producer: testaddress.Addrinfo["producer"],
				GasLimit: gasLimit,
			})
		_, err = ws.RunActions(ctx, 0, nil)
		require.NoError(err)
		require.NoError(sf.Commit(ws))

		data, _ := hex.DecodeString("60806040526000600360146101000a81548160ff02191690831515021790555034801561002b57600080fd5b506040516020806119938339810180604052810190808051906020019092919050505033600360006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555080600181905550806000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055503373ffffffffffffffffffffffffffffffffffffffff16600073ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef836040518082815260200191505060405180910390a3506118448061014f6000396000f3006080604052600436106100e6576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806306fdde03146100eb578063095ea7b31461017b57806318160ddd146101e057806323b872dd1461020b578063313ce567146102905780633f4ba83a146102c15780635c975abb146102d8578063661884631461030757806370a082311461036c5780638456cb59146103c35780638da5cb5b146103da57806395d89b4114610431578063a9059cbb146104c1578063d73dd62314610526578063dd62ed3e1461058b578063f2fde38b14610602575b600080fd5b3480156100f757600080fd5b50610100610645565b6040518080602001828103825283818151815260200191508051906020019080838360005b83811015610140578082015181840152602081019050610125565b50505050905090810190601f16801561016d5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561018757600080fd5b506101c6600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291908035906020019092919050505061067e565b604051808215151515815260200191505060405180910390f35b3480156101ec57600080fd5b506101f56106ae565b6040518082815260200191505060405180910390f35b34801561021757600080fd5b50610276600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803590602001909291905050506106b8565b604051808215151515815260200191505060405180910390f35b34801561029c57600080fd5b506102a5610763565b604051808260ff1660ff16815260200191505060405180910390f35b3480156102cd57600080fd5b506102d6610768565b005b3480156102e457600080fd5b506102ed610828565b604051808215151515815260200191505060405180910390f35b34801561031357600080fd5b50610352600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291908035906020019092919050505061083b565b604051808215151515815260200191505060405180910390f35b34801561037857600080fd5b506103ad600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061086b565b6040518082815260200191505060405180910390f35b3480156103cf57600080fd5b506103d86108b3565b005b3480156103e657600080fd5b506103ef610974565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561043d57600080fd5b5061044661099a565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561048657808201518184015260208101905061046b565b50505050905090810190601f1680156104b35780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b3480156104cd57600080fd5b5061050c600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803590602001909291905050506109d3565b604051808215151515815260200191505060405180910390f35b34801561053257600080fd5b50610571600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610a7c565b604051808215151515815260200191505060405180910390f35b34801561059757600080fd5b506105ec600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610aac565b6040518082815260200191505060405180910390f35b34801561060e57600080fd5b50610643600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610b33565b005b6040805190810160405280600d81526020017f496f546558204e6574776f726b0000000000000000000000000000000000000081525081565b6000600360149054906101000a900460ff1615151561069c57600080fd5b6106a68383610c8b565b905092915050565b6000600154905090565b6000600360149054906101000a900460ff161515156106d657600080fd5b82600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff161415151561071357600080fd5b3073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff161415151561074e57600080fd5b610759858585610d7d565b9150509392505050565b601281565b600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161415156107c457600080fd5b600360149054906101000a900460ff1615156107df57600080fd5b6000600360146101000a81548160ff0219169083151502179055507f7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b3360405160405180910390a1565b600360149054906101000a900460ff1681565b6000600360149054906101000a900460ff1615151561085957600080fd5b6108638383611137565b905092915050565b60008060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561090f57600080fd5b600360149054906101000a900460ff1615151561092b57600080fd5b6001600360146101000a81548160ff0219169083151502179055507f6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff62560405160405180910390a1565b600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6040805190810160405280600481526020017f494f54580000000000000000000000000000000000000000000000000000000081525081565b6000600360149054906101000a900460ff161515156109f157600080fd5b82600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff1614151515610a2e57600080fd5b3073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff1614151515610a6957600080fd5b610a7384846113c8565b91505092915050565b6000600360149054906101000a900460ff16151515610a9a57600080fd5b610aa483836115e7565b905092915050565b6000600260008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054905092915050565b600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515610b8f57600080fd5b600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff1614151515610bcb57600080fd5b8073ffffffffffffffffffffffffffffffffffffffff16600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff167f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e060405160405180910390a380600360006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b600081600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925846040518082815260200191505060405180910390a36001905092915050565b60008073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff1614151515610dba57600080fd5b6000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020548211151515610e0757600080fd5b600260008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020548211151515610e9257600080fd5b610ee3826000808773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117e390919063ffffffff16565b6000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550610f76826000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117fc90919063ffffffff16565b6000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000208190555061104782600260008773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117e390919063ffffffff16565b600260008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff168473ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef846040518082815260200191505060405180910390a3600190509392505050565b600080600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054905080831115611248576000600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055506112dc565b61125b83826117e390919063ffffffff16565b600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055505b8373ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008873ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546040518082815260200191505060405180910390a3600191505092915050565b60008073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff161415151561140557600080fd5b6000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054821115151561145257600080fd5b6114a3826000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117e390919063ffffffff16565b6000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550611536826000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117fc90919063ffffffff16565b6000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef846040518082815260200191505060405180910390a36001905092915050565b600061167882600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117fc90919063ffffffff16565b600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546040518082815260200191505060405180910390a36001905092915050565b60008282111515156117f157fe5b818303905092915050565b6000818301905082811015151561180f57fe5b809050929150505600a165627a7a72305820ffa710f4c82e1f12645713d71da89f0c795cce49fbe12e060ea17f520d6413f800290000000000000000000000000000000000000000204fce5e3e25026110000000")
		execution, err := action.NewExecution(action.EmptyAddress, 1, big.NewInt(0), uint64(5000000), big.NewInt(0), data)
		require.NoError(err)

		bd := &action.EnvelopeBuilder{}
		elp := bd.SetAction(execution).
			SetNonce(1).
			SetGasLimit(5000000).Build()
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
		require.Equal(eHash, r.ActHash)
		contract := r.ContractAddress
		contractAddr, err := address.FromString(contract)
		require.NoError(err)
		h := contractAddr.Bytes()
		ws, err = sf.NewWorkingSet()
		require.NoError(err)

		stateDB := evm.NewStateDBAdapter(bc, ws, uint64(0), hash.ZeroHash256)
		var evmContractAddrHash common.Address
		copy(evmContractAddrHash[:], h)
		code := stateDB.GetCode(evmContractAddrHash)
		require.Equal(data[335:len(data)-32], code)

		log.S().Info("======= Transfer to alfa")
		data, _ = hex.DecodeString("a9059cbb")
		addr, _ := address.FromString(testaddress.Addrinfo["alfa"].String())
		to := addr.Bytes()
		alfa := hash.BytesToHash256(to)
		value := hash.ZeroHash256
		// send 10000 token to Alfa
		h = value[24:]
		binary.BigEndian.PutUint64(h, 10000)
		data = append(data, alfa[:]...)
		data = append(data, value[:]...)
		log.L().Warn("TestER", zap.Binary("v", data[:]))
		execution, err = action.NewExecution(contract, 2, big.NewInt(0), uint64(1000000), big.NewInt(0), data)
		require.NoError(err)

		bd = &action.EnvelopeBuilder{}
		elp = bd.SetAction(execution).
			SetNonce(2).
			SetGasLimit(1000000).Build()
		selp, err = action.Sign(elp, testaddress.Keyinfo["producer"].PriKey)
		require.NoError(err)

		// send 20000 token to bravo
		data, _ = hex.DecodeString("a9059cbb")
		addr, _ = address.FromString(testaddress.Addrinfo["bravo"].String())
		to = addr.Bytes()
		bravo := hash.BytesToHash256(to)
		value = hash.ZeroHash256
		h = value[24:]
		binary.BigEndian.PutUint64(h, 20000)
		data = append(data, bravo[:]...)
		data = append(data, value[:]...)
		log.L().Warn("TestER", zap.Binary("v", data[:]))
		ex2, err := action.NewExecution(contract, 3, big.NewInt(0), uint64(1000000), big.NewInt(0), data)
		require.NoError(err)

		bd = &action.EnvelopeBuilder{}
		elp2 := bd.SetAction(ex2).
			SetNonce(3).
			SetGasLimit(1000000).Build()
		selp2, err := action.Sign(elp2, testaddress.Keyinfo["producer"].PriKey)
		require.NoError(err)

		actionMap = make(map[string][]action.SealedEnvelope)
		actionMap[testaddress.Addrinfo["producer"].String()] = []action.SealedEnvelope{selp, selp2}
		blk, err = bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
		)
		require.NoError(err)
		require.NoError(bc.ValidateBlock(blk))
		require.Nil(bc.CommitBlock(blk))

		log.S().Info("======= Transfer to bravo")
		// alfa send 2000 to bravo
		data, _ = hex.DecodeString("a9059cbb")
		value = hash.ZeroHash256
		h = value[24:]
		binary.BigEndian.PutUint64(h, 2000)
		data = append(data, bravo[:]...)
		data = append(data, value[:]...)
		log.L().Warn("TestER", zap.Binary("v", data[:]))
		ex3, err := action.NewExecution(contract, 1, big.NewInt(0), uint64(10000000), big.NewInt(0), data)
		require.NoError(err)

		bd = &action.EnvelopeBuilder{}
		elp = bd.SetAction(ex3).
			SetNonce(1).
			SetGasLimit(1000000).Build()
		selp3, err := action.Sign(elp, testaddress.Keyinfo["alfa"].PriKey)
		require.NoError(err)

		actionMap = make(map[string][]action.SealedEnvelope)
		actionMap[testaddress.Addrinfo["alfa"].String()] = []action.SealedEnvelope{selp3}
		blk, err = bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
		)
		require.NoError(err)
		require.NoError(bc.ValidateBlock(blk))
		require.Nil(bc.CommitBlock(blk))

		// get balance
		data, _ = hex.DecodeString("70a08231")
		data = append(data, alfa[:]...)
		log.L().Warn("TestER", zap.Binary("v", data[:]))
		execution, err = action.NewExecution(contract, 4, big.NewInt(0), uint64(10000000), big.NewInt(0), data)
		require.NoError(err)
		bd = &action.EnvelopeBuilder{}
		elp = bd.SetAction(execution).
			SetNonce(4).
			SetGasLimit(1000000).Build()
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

		// verify balance
		eHash = execution.Hash()
		r, _ = bc.GetReceiptByActionHash(eHash)
		require.Equal(eHash, r.ActHash)
		h = r.ReturnValue[len(r.ReturnValue)-8:]
		amount := binary.BigEndian.Uint64(h)
		require.Equal(uint64(8000), amount)
	}

	t.Run("EVM", func(t *testing.T) {
		testEVM(t)
	})
	t.Run("RollDice", func(t *testing.T) {
		testRollDice(t)
	})
	t.Run("ERC20", func(t *testing.T) {
		testERC20(t)
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

/**
 * source of smart contract: https://etherscan.io/address/0x6fb3e0a217407efff7ca062d46c26e5d60a14d69#code
 */
func TestERC20(t *testing.T) {
	sct := &smartContractTest{
		prepare: map[string]*big.Int{
			testaddress.Addrinfo["producer"].String(): unit.ConvertIotxToRau(1000000000),
			testaddress.Addrinfo["alfa"].String():     big.NewInt(0),
			testaddress.Addrinfo["bravo"].String():    big.NewInt(0),
		},
		deploy: execCfg{
			codeHex:          "60806040526000600360146101000a81548160ff02191690831515021790555034801561002b57600080fd5b506040516020806119938339810180604052810190808051906020019092919050505033600360006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555080600181905550806000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055503373ffffffffffffffffffffffffffffffffffffffff16600073ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef836040518082815260200191505060405180910390a3506118448061014f6000396000f3006080604052600436106100e6576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806306fdde03146100eb578063095ea7b31461017b57806318160ddd146101e057806323b872dd1461020b578063313ce567146102905780633f4ba83a146102c15780635c975abb146102d8578063661884631461030757806370a082311461036c5780638456cb59146103c35780638da5cb5b146103da57806395d89b4114610431578063a9059cbb146104c1578063d73dd62314610526578063dd62ed3e1461058b578063f2fde38b14610602575b600080fd5b3480156100f757600080fd5b50610100610645565b6040518080602001828103825283818151815260200191508051906020019080838360005b83811015610140578082015181840152602081019050610125565b50505050905090810190601f16801561016d5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561018757600080fd5b506101c6600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291908035906020019092919050505061067e565b604051808215151515815260200191505060405180910390f35b3480156101ec57600080fd5b506101f56106ae565b6040518082815260200191505060405180910390f35b34801561021757600080fd5b50610276600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803590602001909291905050506106b8565b604051808215151515815260200191505060405180910390f35b34801561029c57600080fd5b506102a5610763565b604051808260ff1660ff16815260200191505060405180910390f35b3480156102cd57600080fd5b506102d6610768565b005b3480156102e457600080fd5b506102ed610828565b604051808215151515815260200191505060405180910390f35b34801561031357600080fd5b50610352600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291908035906020019092919050505061083b565b604051808215151515815260200191505060405180910390f35b34801561037857600080fd5b506103ad600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061086b565b6040518082815260200191505060405180910390f35b3480156103cf57600080fd5b506103d86108b3565b005b3480156103e657600080fd5b506103ef610974565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561043d57600080fd5b5061044661099a565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561048657808201518184015260208101905061046b565b50505050905090810190601f1680156104b35780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b3480156104cd57600080fd5b5061050c600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803590602001909291905050506109d3565b604051808215151515815260200191505060405180910390f35b34801561053257600080fd5b50610571600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610a7c565b604051808215151515815260200191505060405180910390f35b34801561059757600080fd5b506105ec600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610aac565b6040518082815260200191505060405180910390f35b34801561060e57600080fd5b50610643600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610b33565b005b6040805190810160405280600d81526020017f496f546558204e6574776f726b0000000000000000000000000000000000000081525081565b6000600360149054906101000a900460ff1615151561069c57600080fd5b6106a68383610c8b565b905092915050565b6000600154905090565b6000600360149054906101000a900460ff161515156106d657600080fd5b82600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff161415151561071357600080fd5b3073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff161415151561074e57600080fd5b610759858585610d7d565b9150509392505050565b601281565b600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161415156107c457600080fd5b600360149054906101000a900460ff1615156107df57600080fd5b6000600360146101000a81548160ff0219169083151502179055507f7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b3360405160405180910390a1565b600360149054906101000a900460ff1681565b6000600360149054906101000a900460ff1615151561085957600080fd5b6108638383611137565b905092915050565b60008060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561090f57600080fd5b600360149054906101000a900460ff1615151561092b57600080fd5b6001600360146101000a81548160ff0219169083151502179055507f6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff62560405160405180910390a1565b600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6040805190810160405280600481526020017f494f54580000000000000000000000000000000000000000000000000000000081525081565b6000600360149054906101000a900460ff161515156109f157600080fd5b82600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff1614151515610a2e57600080fd5b3073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff1614151515610a6957600080fd5b610a7384846113c8565b91505092915050565b6000600360149054906101000a900460ff16151515610a9a57600080fd5b610aa483836115e7565b905092915050565b6000600260008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054905092915050565b600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515610b8f57600080fd5b600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff1614151515610bcb57600080fd5b8073ffffffffffffffffffffffffffffffffffffffff16600360009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff167f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e060405160405180910390a380600360006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b600081600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925846040518082815260200191505060405180910390a36001905092915050565b60008073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff1614151515610dba57600080fd5b6000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020548211151515610e0757600080fd5b600260008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020548211151515610e9257600080fd5b610ee3826000808773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117e390919063ffffffff16565b6000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550610f76826000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117fc90919063ffffffff16565b6000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000208190555061104782600260008773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117e390919063ffffffff16565b600260008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff168473ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef846040518082815260200191505060405180910390a3600190509392505050565b600080600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054905080831115611248576000600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055506112dc565b61125b83826117e390919063ffffffff16565b600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055505b8373ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008873ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546040518082815260200191505060405180910390a3600191505092915050565b60008073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff161415151561140557600080fd5b6000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054821115151561145257600080fd5b6114a3826000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117e390919063ffffffff16565b6000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550611536826000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117fc90919063ffffffff16565b6000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef846040518082815260200191505060405180910390a36001905092915050565b600061167882600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546117fc90919063ffffffff16565b600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546040518082815260200191505060405180910390a36001905092915050565b60008282111515156117f157fe5b818303905092915050565b6000818301905082811015151561180f57fe5b809050929150505600a165627a7a72305820ffa710f4c82e1f12645713d71da89f0c795cce49fbe12e060ea17f520d6413f800290000000000000000000000000000000000000000204fce5e3e25026110000000",
			executor:         testaddress.Addrinfo["producer"].String(),
			privateKey:       testaddress.Keyinfo["producer"].PriKey,
			amount:           0,
			gasLimit:         uint64(5000000),
			gasPrice:         0,
			hasReturnValue:   false,
			expectedBalances: map[string]*big.Int{},
		},
		executions: []execCfg{
			func() execCfg {
				data, _ := hex.DecodeString("a9059cbb")
				to := testaddress.Addrinfo["alfa"].Bytes()
				alfa := hash.BytesToHash256(to)
				value := hash.ZeroHash256
				// send 10000 token to Alfa
				h := value[24:]
				binary.BigEndian.PutUint64(h, 10000)
				data = append(data, alfa[:]...)
				data = append(data, value[:]...)
				return execCfg{
					executor:         testaddress.Addrinfo["producer"].String(),
					privateKey:       testaddress.Keyinfo["producer"].PriKey,
					codeHex:          hex.EncodeToString(data),
					amount:           0,
					gasLimit:         uint64(1000000),
					gasPrice:         0,
					hasReturnValue:   false,
					expectedBalances: map[string]*big.Int{},
				}
			}(),
			func() execCfg {
				data, _ := hex.DecodeString("a9059cbb")
				to := testaddress.Addrinfo["bravo"].Bytes()
				bravo := hash.BytesToHash256(to)
				value := hash.ZeroHash256
				h := value[24:]
				binary.BigEndian.PutUint64(h, 20000)
				data = append(data, bravo[:]...)
				data = append(data, value[:]...)
				return execCfg{
					executor:         testaddress.Addrinfo["producer"].String(),
					privateKey:       testaddress.Keyinfo["producer"].PriKey,
					codeHex:          hex.EncodeToString(data),
					amount:           0,
					gasLimit:         uint64(1000000),
					gasPrice:         0,
					hasReturnValue:   false,
					expectedBalances: map[string]*big.Int{},
				}
			}(),
			func() execCfg {
				data, _ := hex.DecodeString("a9059cbb")
				to := testaddress.Addrinfo["bravo"].Bytes()
				bravo := hash.BytesToHash256(to)
				value := hash.ZeroHash256
				h := value[24:]
				binary.BigEndian.PutUint64(h, 2000)
				data = append(data, bravo[:]...)
				data = append(data, value[:]...)
				return execCfg{
					executor:         testaddress.Addrinfo["alfa"].String(),
					privateKey:       testaddress.Keyinfo["alfa"].PriKey,
					codeHex:          hex.EncodeToString(data),
					amount:           0,
					gasLimit:         uint64(1000000),
					gasPrice:         0,
					hasReturnValue:   false,
					expectedBalances: map[string]*big.Int{},
				}
			}(),
			func() execCfg {
				data, _ := hex.DecodeString("70a08231")
				to := testaddress.Addrinfo["alfa"].Bytes()
				alfa := hash.BytesToHash256(to)
				data = append(data, alfa[:]...)
				retval, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000001f40")
				return execCfg{
					executor:            testaddress.Addrinfo["producer"].String(),
					privateKey:          testaddress.Keyinfo["producer"].PriKey,
					codeHex:             hex.EncodeToString(data),
					amount:              0,
					gasLimit:            uint64(1000000),
					gasPrice:            0,
					hasReturnValue:      true,
					expectedReturnValue: retval,
					expectedBalances:    map[string]*big.Int{},
				}
			}(),
		},
	}

	sct.run(require.New(t))
}

/**
 * Source of smart contract: https://etherscan.io/address/0x8dd5fbce2f6a956c3022ba3663759011dd51e73e#code
 */
func TestDelegateERC20(t *testing.T) {
	t.Skip("Skipping when the execution require too much gas.")
	sct := &smartContractTest{
		prepare: map[string]*big.Int{testaddress.Addrinfo["alfa"].String(): big.NewInt(9876543210)},
		deploy: execCfg{
			executor:            testaddress.Addrinfo["alfa"].String(),
			privateKey:          testaddress.Keyinfo["alfa"].PriKey,
			codeHex:             "60606040526000600560146101000a81548160ff0219169083151502179055506040805190810160405280600781526020017f5472756555534400000000000000000000000000000000000000000000000000815250600790805190602001906200006c929190620002a7565b506040805190810160405280600481526020017f545553440000000000000000000000000000000000000000000000000000000081525060089080519060200190620000ba929190620002a7565b50601260ff16600a0a61271002600d55601260ff16600a0a6301312d0002600e556007600f60006101000a81548169ffffffffffffffffffff021916908369ffffffffffffffffffff160217905550612710600f600a6101000a81548169ffffffffffffffffffff021916908369ffffffffffffffffffff1602179055506000600f60146101000a81548169ffffffffffffffffffff021916908369ffffffffffffffffffff160217905550612710601060006101000a81548169ffffffffffffffffffff021916908369ffffffffffffffffffff16021790555060006011556000601260006101000a81548169ffffffffffffffffffff021916908369ffffffffffffffffffff1602179055506127106012600a6101000a81548169ffffffffffffffffffff021916908369ffffffffffffffffffff160217905550600060135534156200020857600080fd5b336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506000341415156200025857600080fd5b600060038190555033601460006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555062000356565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10620002ea57805160ff19168380011785556200031b565b828001600101855582156200031b579182015b828111156200031a578251825591602001919060010190620002fd565b5b5090506200032a91906200032e565b5090565b6200035391905b808211156200034f57600081600090555060010162000335565b5090565b90565b615ac980620003666000396000f3006060604052600436106102d5576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806302d3fdc9146102e257806306fdde031461030b578063095ea7b31461039957806309ab8bba146103f35780630b8e845a1461045f5780630ce511791461048857806317ffc3201461051e57806318160ddd146105575780631d2d8400146105805780631db8cb3f146105b95780631f7af1df1461066357806323b872dd146106b857806326fe995114610731578063296f4000146107865780632aed7f3f146107ff578063313ce567146108385780633ed10b92146108675780633f4ba83a146108bc57806340c10f19146108d157806342966c681461091357806343a468c8146109365780634df6b45d146109835780634e71e0c814610a1b57806354f78dad14610a30578063554249b314610a6957806356e1c40d14610ae25780635a44413914610b235780635c131d7014610b785780635c975abb14610ba15780635db07aee14610bce5780635ebaf1db14610c0f57806361927adb14610c645780636618846314610c9d5780636d4717fe14610cf757806370a0823114610d4c57806370df42e114610d9957806376e71dd814610dc55780637bb98a6814610dee5780638456cb5914610e4357806386575e4014610e585780638d93eac214610ef85780638da5cb5b14610f395780638f98ce8f14610f8e57806393d3173a14610fcf57806395d89b41146110485780639cd1a121146110d65780639f727c271461114f578063a9059cbb14611164578063ab55979d146111be578063bd7243f6146111f7578063c0ee0b8a14611230578063c18f483114611286578063c89e4361146112c7578063cdab73b51461131c578063d42cfc4114611371578063d63a1389146113b2578063d73dd623146113db578063dd62ed3e14611435578063e30c3978146114a1578063edc1e4f9146114f6578063f2fde38b1461152f575b34156102e057600080fd5b005b34156102ed57600080fd5b6102f5611568565b6040518082815260200191505060405180910390f35b341561031657600080fd5b61031e61156e565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561035e578082015181840152602081019050610343565b50505050905090810190601f16801561038b5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34156103a457600080fd5b6103d9600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803590602001909190505061160c565b604051808215151515815260200191505060405180910390f35b34156103fe57600080fd5b610449600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803573ffffffffffffffffffffffffffffffffffffffff1690602001909190505061179a565b6040518082815260200191505060405180910390f35b341561046a57600080fd5b6104726117ae565b6040518082815260200191505060405180910390f35b341561049357600080fd5b61051c600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506117b4565b005b341561052957600080fd5b610555600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050611919565b005b341561056257600080fd5b61056a611a86565b6040518082815260200191505060405180910390f35b341561058b57600080fd5b6105b7600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050611b9c565b005b34156105c457600080fd5b610661600480803569ffffffffffffffffffff1690602001909190803569ffffffffffffffffffff1690602001909190803569ffffffffffffffffffff1690602001909190803569ffffffffffffffffffff1690602001909190803590602001909190803569ffffffffffffffffffff1690602001909190803569ffffffffffffffffffff16906020019091908035906020019091905050611ca0565b005b341561066e57600080fd5b610676611e75565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34156106c357600080fd5b610717600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803573ffffffffffffffffffffffffffffffffffffffff16906020019091908035906020019091905050611e9b565b604051808215151515815260200191505060405180910390f35b341561073c57600080fd5b61074461205f565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b341561079157600080fd5b6107e5600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803590602001909190803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050612085565b604051808215151515815260200191505060405180910390f35b341561080a57600080fd5b610836600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506120fb565b005b341561084357600080fd5b61084b61222e565b604051808260ff1660ff16815260200191505060405180910390f35b341561087257600080fd5b61087a612233565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34156108c757600080fd5b6108cf612259565b005b34156108dc57600080fd5b610911600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091908035906020019091905050612318565b005b341561091e57600080fd5b610934600480803590602001909190505061264b565b005b341561094157600080fd5b61096d600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506127c1565b6040518082815260200191505060405180910390f35b341561098e57600080fd5b610a01600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803590602001909190803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506127d3565b604051808215151515815260200191505060405180910390f35b3415610a2657600080fd5b610a2e61284b565b005b3415610a3b57600080fd5b610a67600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506129ea565b005b3415610a7457600080fd5b610ac8600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803590602001909190803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050612b22565b604051808215151515815260200191505060405180910390f35b3415610aed57600080fd5b610af5612b98565b604051808269ffffffffffffffffffff1669ffffffffffffffffffff16815260200191505060405180910390f35b3415610b2e57600080fd5b610b36612bb4565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3415610b8357600080fd5b610b8b612bda565b6040518082815260200191505060405180910390f35b3415610bac57600080fd5b610bb4612be0565b604051808215151515815260200191505060405180910390f35b3415610bd957600080fd5b610be1612bf3565b604051808269ffffffffffffffffffff1669ffffffffffffffffffff16815260200191505060405180910390f35b3415610c1a57600080fd5b610c22612c0f565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3415610c6f57600080fd5b610c9b600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050612c35565b005b3415610ca857600080fd5b610cdd600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091908035906020019091905050612cd4565b604051808215151515815260200191505060405180910390f35b3415610d0257600080fd5b610d0a612e62565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3415610d5757600080fd5b610d83600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050612e88565b6040518082815260200191505060405180910390f35b3415610da457600080fd5b610dc36004808035906020019091908035906020019091905050612fd8565b005b3415610dd057600080fd5b610dd8613093565b6040518082815260200191505060405180910390f35b3415610df957600080fd5b610e016130a2565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3415610e4e57600080fd5b610e566130c8565b005b3415610e6357600080fd5b610ef6600480803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509190803590602001908201803590602001908080601f01602080910402602001604051908101604052809392919081815260200183838082843782019150505050505091905050613188565b005b3415610f0357600080fd5b610f0b613215565b604051808269ffffffffffffffffffff1669ffffffffffffffffffff16815260200191505060405180910390f35b3415610f4457600080fd5b610f4c613231565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b3415610f9957600080fd5b610fa1613256565b604051808269ffffffffffffffffffff1669ffffffffffffffffffff16815260200191505060405180910390f35b3415610fda57600080fd5b61102e600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803590602001909190803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050613272565b604051808215151515815260200191505060405180910390f35b341561105357600080fd5b61105b6132e8565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561109b578082015181840152602081019050611080565b50505050905090810190601f1680156110c85780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34156110e157600080fd5b611135600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803590602001909190803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050613386565b604051808215151515815260200191505060405180910390f35b341561115a57600080fd5b6111626133fc565b005b341561116f57600080fd5b6111a4600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919080359060200190919050506134ce565b604051808215151515815260200191505060405180910390f35b34156111c957600080fd5b6111f5600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190505061365c565b005b341561120257600080fd5b61122e600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050613737565b005b341561123b57600080fd5b611284600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803590602001909190803590602001908201803590602001919091929050506139d0565b005b341561129157600080fd5b6112996139d5565b604051808269ffffffffffffffffffff1669ffffffffffffffffffff16815260200191505060405180910390f35b34156112d257600080fd5b6112da6139f1565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b341561132757600080fd5b61132f613a17565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b341561137c57600080fd5b611384613a3d565b604051808269ffffffffffffffffffff1669ffffffffffffffffffff16815260200191505060405180910390f35b34156113bd57600080fd5b6113c5613a59565b6040518082815260200191505060405180910390f35b34156113e657600080fd5b61141b600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091908035906020019091905050613a5f565b604051808215151515815260200191505060405180910390f35b341561144057600080fd5b61148b600480803573ffffffffffffffffffffffffffffffffffffffff1690602001909190803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050613bed565b6040518082815260200191505060405180910390f35b34156114ac57600080fd5b6114b4613d73565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b341561150157600080fd5b61152d600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050613d99565b005b341561153a57600080fd5b611566600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050613ed1565b005b600d5481565b60078054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156116045780601f106115d957610100808354040283529160200191611604565b820191906000526020600020905b8154815290600101906020018083116115e757829003601f168201915b505050505081565b60008073ffffffffffffffffffffffffffffffffffffffff16600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1614156116755761166e8383613f70565b9050611794565b600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1663296f40008484336000604051602001526040518463ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018381526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019350505050602060405180830381600087803b151561177657600080fd5b6102c65a03f1151561178757600080fd5b5050506040518051905090505b92915050565b60006117a68383613bed565b905092915050565b60135481565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561180f57600080fd5b83600960006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555082600a60006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555081600b60006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555080600c60006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050505050565b60008060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561197657600080fd5b8173ffffffffffffffffffffffffffffffffffffffff166370a08231306000604051602001526040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050602060405180830381600087803b1515611a1957600080fd5b6102c65a03f11515611a2a57600080fd5b505050604051805190509050611a826000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff16828473ffffffffffffffffffffffffffffffffffffffff16613fa09092919063ffffffff16565b5050565b60008073ffffffffffffffffffffffffffffffffffffffff16600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415611aed57611ae6614073565b9050611b99565b600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166376e71dd86000604051602001526040518163ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401602060405180830381600087803b1515611b7b57600080fd5b6102c65a03f11515611b8c57600080fd5b5050506040518051905090505b90565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515611bf757600080fd5b80600660006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff167feef3c91406f155f6bf1d8754e73003590b8bfa5cfa5472ee9ea936761864ea3060405160405180910390a250565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515611cfb57600080fd5b60008769ffffffffffffffffffff1614151515611d1757600080fd5b60008569ffffffffffffffffffff1614151515611d3357600080fd5b60008269ffffffffffffffffffff1614151515611d4f57600080fd5b87600f60006101000a81548169ffffffffffffffffffff021916908369ffffffffffffffffffff16021790555086600f600a6101000a81548169ffffffffffffffffffff021916908369ffffffffffffffffffff16021790555085600f60146101000a81548169ffffffffffffffffffff021916908369ffffffffffffffffffff16021790555084601060006101000a81548169ffffffffffffffffffff021916908369ffffffffffffffffffff1602179055508360118190555082601260006101000a81548169ffffffffffffffffffff021916908369ffffffffffffffffffff160217905550816012600a6101000a81548169ffffffffffffffffffff021916908369ffffffffffffffffffff160217905550806013819055505050505050505050565b600960009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60008073ffffffffffffffffffffffffffffffffffffffff16600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415611f0557611efe84848461407d565b9050612058565b600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16634df6b45d858585336000604051602001526040518563ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018381526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001945050505050602060405180830381600087803b151561203a57600080fd5b6102c65a03f1151561204b57600080fd5b5050506040518051905090505b9392505050565b600560009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000600560009054906101000a900473ffffffffffffffffffffffffffffffffffffffff168073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161415156120e457600080fd5b6120ef8585856140af565b60019150509392505050565b60008060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561215857600080fd5b8190508073ffffffffffffffffffffffffffffffffffffffff1663f2fde38b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff166040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050600060405180830381600087803b151561221657600080fd5b6102c65a03f1151561222757600080fd5b5050505050565b601281565b600460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161415156122b457600080fd5b600560149054906101000a900460ff1615156122cf57600080fd5b6000600560146101000a81548160ff0219169083151502179055507f7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b3360405160405180910390a1565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561237357600080fd5b600960009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16636f626eb3836000604051602001526040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050602060405180830381600087803b151561243857600080fd5b6102c65a03f1151561244957600080fd5b50505060405180519050151561245e57600080fd5b6124738160035461422590919063ffffffff16565b600381905550600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166321e5383a83836040518363ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200182815260200192505050600060405180830381600087803b151561253d57600080fd5b6102c65a03f1151561254e57600080fd5b5050508173ffffffffffffffffffffffffffffffffffffffff167f0f6798a560793a54c3bcfe86a93cde1e73087d944c0ea20544137d4121396885826040518082815260200191505060405180910390a28173ffffffffffffffffffffffffffffffffffffffff16600073ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef836040518082815260200191505060405180910390a36126468282600f60149054906101000a900469ffffffffffffffffffff16601060009054906101000a900469ffffffffffffffffffff166011546000614243565b505050565b600080600a60009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16636f626eb3336000604051602001526040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050602060405180830381600087803b151561271357600080fd5b6102c65a03f1151561272457600080fd5b50505060405180519050151561273957600080fd5b600d54831015151561274a57600080fd5b600e54831115151561275b57600080fd5b61279c3384601260009054906101000a900469ffffffffffffffffffff166012600a9054906101000a900469ffffffffffffffffffff166013546000614243565b91506127b182846144b290919063ffffffff16565b90506127bc816144cb565b505050565b60006127cc82612e88565b9050919050565b6000600560009054906101000a900473ffffffffffffffffffffffffffffffffffffffff168073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561283257600080fd5b61283e86868686614769565b6001915050949350505050565b600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161415156128a757600080fd5b600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff167f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e060405160405180910390a3600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff166000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506000600160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515612a4557600080fd5b80600260006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16634e71e0c86040518163ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401600060405180830381600087803b1515612b0b57600080fd5b6102c65a03f11515612b1c57600080fd5b50505050565b6000600560009054906101000a900473ffffffffffffffffffffffffffffffffffffffff168073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515612b8157600080fd5b612b8c8585856149a8565b60019150509392505050565b601260009054906101000a900469ffffffffffffffffffff1681565b600a60009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b600e5481565b600560149054906101000a900460ff1681565b601060009054906101000a900469ffffffffffffffffffff1681565b601460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515612c9057600080fd5b80600560006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b60008073ffffffffffffffffffffffffffffffffffffffff16600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415612d3d57612d368383614c31565b9050612e5c565b600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166393d3173a8484336000604051602001526040518463ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018381526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019350505050602060405180830381600087803b1515612e3e57600080fd5b6102c65a03f11515612e4f57600080fd5b5050506040518051905090505b92915050565b600c60009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60008073ffffffffffffffffffffffffffffffffffffffff16600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415612ef057612ee982614c61565b9050612fd3565b600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166343a468c8836000604051602001526040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050602060405180830381600087803b1515612fb557600080fd5b6102c65a03f11515612fc657600080fd5b5050506040518051905090505b919050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561303357600080fd5b80821115151561304257600080fd5b81600d8190555080600e819055507ff8f7312d8aa9257dcfe43287f24cacc0f267875658809b6c7953b277565625228282604051808381526020018281526020019250505060405180910390a15050565b600061309d611a86565b905090565b600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561312357600080fd5b600560149054906101000a900460ff1615151561313f57600080fd5b6001600560146101000a81548160ff0219169083151502179055507f6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff62560405160405180910390a1565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161415156131e357600080fd5b81600790805190602001906131f99291906159f8565b5080600890805190602001906132109291906159f8565b505050565b600f60149054906101000a900469ffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b600f60009054906101000a900469ffffffffffffffffffff1681565b6000600560009054906101000a900473ffffffffffffffffffffffffffffffffffffffff168073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161415156132d157600080fd5b6132dc858585614d4a565b60019150509392505050565b60088054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561337e5780601f106133535761010080835404028352916020019161337e565b820191906000526020600020905b81548152906001019060200180831161336157829003601f168201915b505050505081565b6000600560009054906101000a900473ffffffffffffffffffffffffffffffffffffffff168073ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161415156133e557600080fd5b6133f0838686615207565b60019150509392505050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561345757600080fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166108fc3073ffffffffffffffffffffffffffffffffffffffff16319081150290604051600060405180830381858888f1935050505015156134cc57fe5b565b60008073ffffffffffffffffffffffffffffffffffffffff16600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16141561353757613530838361542f565b9050613656565b600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16639cd1a1218484336000604051602001526040518463ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018381526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019350505050602060405180830381600087803b151561363857600080fd5b6102c65a03f1151561364957600080fd5b5050506040518051905090505b92915050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161415156136b757600080fd5b600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff16141515156136f357600080fd5b80601460006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b60008060009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614151561379457600080fd5b600b60009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16636f626eb3836000604051602001526040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050602060405180830381600087803b151561385957600080fd5b6102c65a03f1151561386a57600080fd5b50505060405180519050151561387f57600080fd5b61388882612e88565b9050600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1663e30443bc8360006040518363ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200182815260200192505050600060405180830381600087803b151561394f57600080fd5b6102c65a03f1151561396057600080fd5b505050613978816003546144b290919063ffffffff16565b6003819055508173ffffffffffffffffffffffffffffffffffffffff167fdf58d2368c06216a398f05a7a88c8edc64a25c33f33fd2bd8b56fbc8822c02d8826040518082815260200191505060405180910390a25050565b600080fd5b6012600a9054906101000a900469ffffffffffffffffffff1681565b600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b600b60009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b600f600a9054906101000a900469ffffffffffffffffffff1681565b60115481565b60008073ffffffffffffffffffffffffffffffffffffffff16600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415613ac857613ac1838361545f565b9050613be7565b600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1663554249b38484336000604051602001526040518463ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018381526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019350505050602060405180830381600087803b1515613bc957600080fd5b6102c65a03f11515613bda57600080fd5b5050506040518051905090505b92915050565b60008073ffffffffffffffffffffffffffffffffffffffff16600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415613c5657613c4f838361548f565b9050613d6d565b600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166309ab8bba84846000604051602001526040518363ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200192505050602060405180830381600087803b1515613d4f57600080fd5b6102c65a03f11515613d6057600080fd5b5050506040518051905090505b92915050565b600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515613df457600080fd5b80600460006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550600460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16634e71e0c86040518163ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401600060405180830381600087803b1515613eba57600080fd5b6102c65a03f11515613ecb57600080fd5b50505050565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141515613f2c57600080fd5b80600160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b6000600560149054906101000a900460ff16151515613f8e57600080fd5b613f9883836155ad565b905092915050565b8273ffffffffffffffffffffffffffffffffffffffff1663a9059cbb83836000604051602001526040518363ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200182815260200192505050602060405180830381600087803b151561404b57600080fd5b6102c65a03f1151561405c57600080fd5b50505060405180519050151561406e57fe5b505050565b6000600354905090565b6000600560149054906101000a900460ff1615151561409b57600080fd5b6140a68484846155c4565b90509392505050565b600460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1663da46098c8285856040518463ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019350505050600060405180830381600087803b15156141a757600080fd5b6102c65a03f115156141b857600080fd5b5050508273ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925846040518082815260200191505060405180910390a3505050565b600080828401905083811015151561423957fe5b8091505092915050565b600080600c60009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16636f626eb3896000604051602001526040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050602060405180830381600087803b151561430b57600080fd5b6102c65a03f1151561431c57600080fd5b505050604051805190508061440d5750600c60009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16636f626eb3846000604051602001526040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050602060405180830381600087803b15156143f157600080fd5b6102c65a03f1151561440257600080fd5b505050604051805190505b1561441b57600091506144a7565b61446a8461445c8769ffffffffffffffffffff1661444e8a69ffffffffffffffffffff168c6155dd90919063ffffffff16565b61561890919063ffffffff16565b61422590919063ffffffff16565b905060008111156144a3576144a288601460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1683615633565b5b8091505b509695505050505050565b60008282111515156144c057fe5b818303905092915050565b6000600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166370a08231336000604051602001526040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050602060405180830381600087803b151561459257600080fd5b6102c65a03f115156145a357600080fd5b5050506040518051905082111515156145bb57600080fd5b339050600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1663cf8eeb7e82846040518363ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200182815260200192505050600060405180830381600087803b151561468257600080fd5b6102c65a03f1151561469357600080fd5b5050506146ab826003546144b290919063ffffffff16565b6003819055508073ffffffffffffffffffffffffffffffffffffffff167fcc16f5dbb4873280815c1ee09dbd06736cffcc184412cf7a71a0fdb75d397ca5836040518082815260200191505060405180910390a2600073ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef846040518082815260200191505060405180910390a35050565b600460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16631a46ec8285836000604051602001526040518363ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200192505050602060405180830381600087803b151561486257600080fd5b6102c65a03f1151561487357600080fd5b50505060405180519050821115151561488b57600080fd5b600460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166397d88cd28583856040518463ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019350505050600060405180830381600087803b151561498357600080fd5b6102c65a03f1151561499457600080fd5b5050506149a2848484615207565b50505050565b600460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16635fd72d168285856040518463ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019350505050600060405180830381600087803b1515614aa057600080fd5b6102c65a03f11515614ab157600080fd5b5050508273ffffffffffffffffffffffffffffffffffffffff168173ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925600460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16631a46ec8285886000604051602001526040518363ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200192505050602060405180830381600087803b1515614bfc57600080fd5b6102c65a03f11515614c0d57600080fd5b505050604051805190506040518082815260200191505060405180910390a3505050565b6000600560149054906101000a900460ff16151515614c4f57600080fd5b614c5983836159b3565b905092915050565b6000600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166370a08231836000604051602001526040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050602060405180830381600087803b1515614d2857600080fd5b6102c65a03f11515614d3957600080fd5b505050604051805190509050919050565b6000600460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16631a46ec8283866000604051602001526040518363ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200192505050602060405180830381600087803b1515614e4557600080fd5b6102c65a03f11515614e5657600080fd5b50505060405180519050905080831115614f7c57600460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1663da46098c838660006040518463ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019350505050600060405180830381600087803b1515614f6357600080fd5b6102c65a03f11515614f7457600080fd5b505050615089565b600460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166397d88cd28386866040518463ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019350505050600060405180830381600087803b151561507457600080fd5b6102c65a03f1151561508557600080fd5b5050505b8373ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925600460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16631a46ec8286896000604051602001526040518363ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200192505050602060405180830381600087803b15156151d157600080fd5b6102c65a03f115156151e257600080fd5b505050604051805190506040518082815260200191505060405180910390a350505050565b600b60009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16636f626eb3846000604051602001526040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050602060405180830381600087803b15156152cc57600080fd5b6102c65a03f115156152dd57600080fd5b505050604051805190501515156152f357600080fd5b600b60009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16636f626eb3836000604051602001526040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050602060405180830381600087803b15156153b857600080fd5b6102c65a03f115156153c957600080fd5b505050604051805190501515156153df57600080fd5b6153ea838383615633565b6154298282600f60009054906101000a900469ffffffffffffffffffff16600f600a9054906101000a900469ffffffffffffffffffff16600088614243565b50505050565b6000600560149054906101000a900460ff1615151561544d57600080fd5b61545783836159ca565b905092915050565b6000600560149054906101000a900460ff1615151561547d57600080fd5b61548783836159e1565b905092915050565b6000600460009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16631a46ec8284846000604051602001526040518363ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200192505050602060405180830381600087803b151561558a57600080fd5b6102c65a03f1151561559b57600080fd5b50505060405180519050905092915050565b60006155ba8383336140af565b6001905092915050565b60006155d284848433614769565b600190509392505050565b60008060008414156155f25760009150615611565b828402905082848281151561560357fe5b0414151561560d57fe5b8091505b5092915050565b600080828481151561562657fe5b0490508091505092915050565b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff161415151561566f57600080fd5b600073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff16141515156156ab57600080fd5b600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166370a08231846000604051602001526040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050602060405180830381600087803b151561577057600080fd5b6102c65a03f1151561578157600080fd5b50505060405180519050811115151561579957600080fd5b600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1663cf8eeb7e84836040518363ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200182815260200192505050600060405180830381600087803b151561585d57600080fd5b6102c65a03f1151561586e57600080fd5b505050600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166321e5383a83836040518363ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200182815260200192505050600060405180830381600087803b151561593557600080fd5b6102c65a03f1151561594657600080fd5b5050508173ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef836040518082815260200191505060405180910390a3505050565b60006159c0838333614d4a565b6001905092915050565b60006159d7338484615207565b6001905092915050565b60006159ee8383336149a8565b6001905092915050565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10615a3957805160ff1916838001178555615a67565b82800160010185558215615a67579182015b82811115615a66578251825591602001919060010190615a4b565b5b509050615a749190615a78565b5090565b615a9a91905b80821115615a96576000816000905550600101615a7e565b5090565b905600a165627a7a72305820d4bc7a14adbd2d56173eee53555b6bbb56664126bd6cbb69d78117da52fc841a0029",
			amount:              0,
			gasLimit:            uint64(2100000),
			gasPrice:            7,
			hasReturnValue:      false,
			expectedReturnValue: "",
			expectedBalances:    map[string]*big.Int{},
		},
		executions: []execCfg{},
	}
	sct.run(require.New(t))
}
