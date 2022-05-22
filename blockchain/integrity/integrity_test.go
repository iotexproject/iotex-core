// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package integrity

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/mock/gomock"
	iotexcrypto "github.com/iotexproject/go-pkgs/crypto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie/mptrie"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockcreationsubscriber"
	"github.com/iotexproject/iotex-core/testutil"
)

var (
	_deployHash      hash.Hash256                                                                           // in block 1
	_setHash         hash.Hash256                                                                           // in block 2
	_shrHash         hash.Hash256                                                                           // in block 3
	_shlHash         hash.Hash256                                                                           // in block 4
	_sarHash         hash.Hash256                                                                           // in block 5
	_extHash         hash.Hash256                                                                           // in block 6
	_crt2Hash        hash.Hash256                                                                           // in block 7
	_storeHash       hash.Hash256                                                                           // in block 8
	_store2Hash      hash.Hash256                                                                           // in block 9
	_setTopic, _     = hex.DecodeString("fe00000000000000000000000000000000000000000000000000000000001f40") // in block 2
	_getTopic, _     = hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001") // in block 2
	_shrTopic, _     = hex.DecodeString("00fe00000000000000000000000000000000000000000000000000000000001f") // in block 3
	_shlTopic, _     = hex.DecodeString("fe00000000000000000000000000000000000000000000000000000000001f00") // in block 4
	_sarTopic, _     = hex.DecodeString("fffe00000000000000000000000000000000000000000000000000000000001f") // in block 5
	_extTopic, _     = hex.DecodeString("4a98ce81a2fd5177f0f42b49cb25b01b720f9ce8019f3937f63b789766c938e2") // in block 6
	_crt2Topic, _    = hex.DecodeString("0000000000000000000000001895e6033cd1081f18e0bd23a4501d9376028523") // in block 7
	_preGrPreStore   *big.Int
	_preGrPostStore  *big.Int
	_postGrPostStore *big.Int
)

func addTestingConstantinopleBlocks(bc blockchain.Blockchain, dao blockdao.BlockDAO, sf factory.Factory, ap actpool.ActPool) error {
	// Add block 1
	priKey0 := identityset.PrivateKey(27)
	ex1, err := action.SignedExecution(action.EmptyAddress, priKey0, 1, big.NewInt(0), 500000, big.NewInt(testutil.TestGasPriceInt64), _constantinopleOpCodeContract)
	if err != nil {
		return err
	}
	_deployHash, err = ex1.Hash()
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), ex1); err != nil {
		return err
	}
	blockTime := time.Unix(1546329600, 0)
	blk, err := bc.MintNewBlock(blockTime)
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// get deployed contract address
	var contract string
	if dao != nil {
		r, err := dao.GetReceiptByActionHash(_deployHash, 1)
		if err != nil {
			return err
		}
		contract = r.ContractAddress
	}

	addOneBlock := func(contract string, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) (hash.Hash256, error) {
		ex1, err := action.SignedExecution(contract, priKey0, nonce, amount, gasLimit, gasPrice, data)
		if err != nil {
			return hash.ZeroHash256, err
		}
		blockTime = blockTime.Add(time.Second)
		if err := ap.Add(context.Background(), ex1); err != nil {
			return hash.ZeroHash256, err
		}
		blk, err = bc.MintNewBlock(blockTime)
		if err != nil {
			return hash.ZeroHash256, err
		}
		if err := bc.CommitBlock(blk); err != nil {
			return hash.ZeroHash256, err
		}
		ex1Hash, err := ex1.Hash()
		if err != nil {
			return hash.ZeroHash256, err
		}
		return ex1Hash, nil
	}

	var (
		zero     = big.NewInt(0)
		gasLimit = testutil.TestGasLimit * 5
		gasPrice = big.NewInt(testutil.TestGasPriceInt64)
	)

	// Add block 2
	// call set() to set storedData = 0xfe...1f40
	funcSig := hash.Hash256b([]byte("set(uint256)"))
	data := append(funcSig[:4], _setTopic...)
	_setHash, err = addOneBlock(contract, 2, zero, gasLimit, gasPrice, data)
	if err != nil {
		return err
	}

	// Add block 3
	// call shright() to test SHR opcode, storedData => 0x00fe...1f
	funcSig = hash.Hash256b([]byte("shright()"))
	_shrHash, err = addOneBlock(contract, 3, zero, gasLimit, gasPrice, funcSig[:4])
	if err != nil {
		return err
	}

	// Add block 4
	// call shleft() to test SHL opcode, storedData => 0xfe...1f00
	funcSig = hash.Hash256b([]byte("shleft()"))
	_shlHash, err = addOneBlock(contract, 4, zero, gasLimit, gasPrice, funcSig[:4])
	if err != nil {
		return err
	}

	// Add block 5
	// call saright() to test SAR opcode, storedData => 0xfffe...1f
	funcSig = hash.Hash256b([]byte("saright()"))
	_sarHash, err = addOneBlock(contract, 5, zero, gasLimit, gasPrice, funcSig[:4])
	if err != nil {
		return err
	}

	// Add block 6
	// call getCodeHash() to test EXTCODEHASH opcode
	funcSig = hash.Hash256b([]byte("getCodeHash(address)"))
	addr, _ := address.FromString(contract)
	ethaddr := hash.BytesToHash256(addr.Bytes())
	data = append(funcSig[:4], ethaddr[:]...)
	_extHash, err = addOneBlock(contract, 6, zero, gasLimit, gasPrice, data)
	if err != nil {
		return err
	}

	// Add block 7
	// call create2() to test CREATE2 opcode
	funcSig = hash.Hash256b([]byte("create2()"))
	_crt2Hash, err = addOneBlock(contract, 7, zero, gasLimit, gasPrice, funcSig[:4])
	if err != nil {
		return err
	}

	// Add block 8
	// test store out of gas
	var (
		caller     = state.NewEmptyAccount()
		callerAddr = hash.BytesToHash160(identityset.Address(27).Bytes())
	)
	_, err = sf.State(caller, protocol.LegacyKeyOption(callerAddr))
	if err != nil {
		return err
	}
	_preGrPreStore = new(big.Int).Set(caller.Balance)
	_storeHash, err = addOneBlock(action.EmptyAddress, 8, unit.ConvertIotxToRau(10000), 3000000, big.NewInt(unit.Qev), _codeStoreOutOfGasContract)
	if err != nil {
		return err
	}

	if dao != nil {
		r, err := dao.GetReceiptByActionHash(_storeHash, 8)
		if err != nil {
			return err
		}
		if r.Status != uint64(iotextypes.ReceiptStatus_ErrCodeStoreOutOfGas) {
			return blockchain.ErrBalance
		}
	}

	// Add block 9
	// test store out of gas
	_, err = sf.State(caller, protocol.LegacyKeyOption(callerAddr))
	if err != nil {
		return err
	}
	_preGrPostStore = new(big.Int).Set(caller.Balance)
	_store2Hash, err = addOneBlock(action.EmptyAddress, 9, unit.ConvertIotxToRau(10000), 3000000, big.NewInt(unit.Qev), _codeStoreOutOfGasContract)
	if err != nil {
		return err
	}

	if dao != nil {
		r, err := dao.GetReceiptByActionHash(_store2Hash, 9)
		if err != nil {
			return err
		}
		if r.Status != uint64(iotextypes.ReceiptStatus_ErrCodeStoreOutOfGas) {
			return blockchain.ErrBalance
		}
	}

	_, err = sf.State(caller, protocol.LegacyKeyOption(callerAddr))
	if err != nil {
		return err
	}
	_postGrPostStore = new(big.Int).Set(caller.Balance)
	return nil
}

func addTestingTsfBlocks(cfg config.Config, bc blockchain.Blockchain, dao blockdao.BlockDAO, ap actpool.ActPool) error {
	ctx := context.Background()
	addOneTsf := func(recipientAddr string, senderPriKey iotexcrypto.PrivateKey, nonce uint64, amount *big.Int, payload []byte, gasLimit uint64, gasPrice *big.Int) error {
		tx, err := action.SignedTransfer(recipientAddr, senderPriKey, nonce, amount, payload, gasLimit, gasPrice)
		if err != nil {
			return err
		}
		if err := ap.Add(ctx, tx); err != nil {
			return err
		}
		return nil
	}
	addOneExec := func(contractAddr string, executorPriKey iotexcrypto.PrivateKey, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) error {
		tx, err := action.SignedExecution(contractAddr, executorPriKey, nonce, amount, gasLimit, gasPrice, data)
		if err != nil {
			return err
		}
		if err := ap.Add(ctx, tx); err != nil {
			return err
		}
		return nil
	}
	// Add block 1
	addr0 := identityset.Address(27).String()
	if err := addOneTsf(addr0, identityset.PrivateKey(0), 1, big.NewInt(90000000), nil, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	ap.Reset()

	priKey0 := identityset.PrivateKey(27)
	addr1 := identityset.Address(28).String()
	priKey1 := identityset.PrivateKey(28)
	addr2 := identityset.Address(29).String()
	priKey2 := identityset.PrivateKey(29)
	addr3 := identityset.Address(30).String()
	priKey3 := identityset.PrivateKey(30)
	addr4 := identityset.Address(31).String()
	priKey4 := identityset.PrivateKey(31)
	addr5 := identityset.Address(32).String()
	priKey5 := identityset.PrivateKey(32)
	addr6 := identityset.Address(33).String()
	// Add block 2
	// test --> A, B, C, D, E, F
	if err := addOneTsf(addr1, priKey0, 1, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr2, priKey0, 2, big.NewInt(30), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr3, priKey0, 3, big.NewInt(50), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr4, priKey0, 4, big.NewInt(70), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr5, priKey0, 5, big.NewInt(110), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr6, priKey0, 6, big.NewInt(50<<20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	// deploy simple smart contract
	data, _ := hex.DecodeString("608060405234801561001057600080fd5b50610233806100206000396000f300608060405260043610610057576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680635bec9e671461005c57806360fe47b114610073578063c2bc2efc146100a0575b600080fd5b34801561006857600080fd5b506100716100f7565b005b34801561007f57600080fd5b5061009e60048036038101908080359060200190929190505050610143565b005b3480156100ac57600080fd5b506100e1600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061017a565b6040518082815260200191505060405180910390f35b5b6001156101155760008081548092919060010191905055506100f8565b7f8bfaa460932ccf8751604dd60efa3eafa220ec358fccb32ef703f91c509bc3ea60405160405180910390a1565b80600081905550807fdf7a95aebff315db1b7716215d602ab537373cdb769232aae6055c06e798425b60405160405180910390a250565b60008073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff16141515156101b757600080fd5b6000548273ffffffffffffffffffffffffffffffffffffffff167fbde7a70c2261170a87678200113c8e12f82f63d0a1d1cfa45681cbac328e87e360405160405180910390a360005490509190505600a165627a7a723058203198d0390613dab2dff2fa053c1865e802618d628429b01ab05b8458afc347eb0029")
	ex1, err := action.SignedExecution(action.EmptyAddress, priKey2, 1, big.NewInt(0), 200000, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), ex1); err != nil {
		return err
	}
	_deployHash, err = ex1.Hash()
	if err != nil {
		return err
	}
	blk, err = bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	ap.Reset()

	// get deployed contract address
	var contract string
	_, gateway := cfg.Plugins[config.GatewayPlugin]
	if gateway && !cfg.Chain.EnableAsyncIndexWrite {
		r, err := dao.GetReceiptByActionHash(_deployHash, 2)
		if err != nil {
			return err
		}
		contract = r.ContractAddress
	}

	// Add block 3
	// Charlie --> A, B, D, E, test
	if err := addOneTsf(addr1, priKey3, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr2, priKey3, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr4, priKey3, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr5, priKey3, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr0, priKey3, 5, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	// call set() to set storedData = 0x1f40
	data, _ = hex.DecodeString("60fe47b1")
	data = append(data, _setTopic...)
	ex1, err = action.SignedExecution(contract, priKey2, 2, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return err
	}
	_setHash, err = ex1.Hash()
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), ex1); err != nil {
		return err
	}
	blk, err = bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	ap.Reset()

	// Add block 4
	// Delta --> B, E, F, test
	if err := addOneTsf(addr2, priKey4, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr5, priKey4, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr6, priKey4, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr0, priKey4, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	data, _ = hex.DecodeString("c2bc2efc")
	data = append(data, _getTopic...)
	ex1, err = action.SignedExecution(contract, priKey2, 3, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return err
	}
	_sarHash, err = ex1.Hash()
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), ex1); err != nil {
		return err
	}
	blk, err = bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 5
	// Delta --> A, B, C, D, F, test
	if err := addOneTsf(addr1, priKey5, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr2, priKey5, 2, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr3, priKey5, 3, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr4, priKey5, 4, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr6, priKey5, 5, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr0, priKey5, 6, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr3, priKey3, 6, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr1, priKey1, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	// call set() to set storedData = 0x1f40
	data, _ = hex.DecodeString("60fe47b1")
	data = append(data, _setTopic...)
	if err := addOneExec(contract, priKey2, 4, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data); err != nil {
		return err
	}
	data, _ = hex.DecodeString("c2bc2efc")
	data = append(data, _getTopic...)
	if err := addOneExec(contract, priKey2, 5, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data); err != nil {
		return err
	}
	blk, err = bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}

func TestCreateBlockchain(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	cfg := config.Default
	// disable account-based testing
	cfg.Chain.TrieDBPath = ""
	cfg.Genesis.EnableGravityChainVoting = false
	cfg.ActPool.MinGasPriceStr = "0"
	// create chain
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(rp.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	require.NoError(err)
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
	bc := blockchain.NewBlockchain(
		cfg,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
	require.NoError(ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(rewardingProtocol.Register(registry))
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	height := bc.TipHeight()
	require.Equal(0, int(height))
	defer func() {
		require.NoError(bc.Stop(ctx))
	}()

	// add 4 sample blocks
	require.NoError(addTestingTsfBlocks(cfg, bc, nil, ap))
	height = bc.TipHeight()
	require.Equal(5, int(height))
}

func TestGetBlockHash(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	cfg := config.Default
	// disable account-based testing
	cfg.Chain.TrieDBPath = ""
	cfg.Genesis.EnableGravityChainVoting = false
	cfg.Genesis.HawaiiBlockHeight = 4
	cfg.Genesis.MidwayBlockHeight = 9
	cfg.ActPool.MinGasPriceStr = "0"
	genesis.SetGenesisTimestamp(cfg.Genesis.Timestamp)
	block.LoadGenesisHash(&config.Default.Genesis)
	// create chain
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(rp.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	require.NoError(err)
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
	bc := blockchain.NewBlockchain(
		cfg,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
	require.NoError(ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(rewardingProtocol.Register(registry))
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	height := bc.TipHeight()
	require.Equal(0, int(height))
	defer func() {
		require.NoError(bc.Stop(ctx))
	}()

	addTestingGetBlockHash(t, cfg.Genesis, bc, dao, ap)
}

func addTestingGetBlockHash(t *testing.T, g genesis.Genesis, bc blockchain.Blockchain, dao blockdao.BlockDAO, ap actpool.ActPool) {
	require := require.New(t)
	priKey0 := identityset.PrivateKey(27)

	// deploy simple smart contract
	/*
		pragma solidity <6.0 >=0.4.24;

		contract Test {
		    event GetBlockhash(bytes32 indexed hash);

		    function getBlockHash(uint256 blockNumber) public  returns (bytes32) {
		       bytes32 h = blockhash(blockNumber);
		        emit GetBlockhash(h);
		        return h;
		    }
		}
	*/
	data, _ := hex.DecodeString("6080604052348015600f57600080fd5b5060de8061001e6000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c8063ee82ac5e14602d575b600080fd5b605660048036036020811015604157600080fd5b8101908080359060200190929190505050606c565b6040518082815260200191505060405180910390f35b60008082409050807f2d93f7749862d33969fb261757410b48065a1bc86a56da5c47820bd063e2338260405160405180910390a28091505091905056fea265627a7a723158200a258cd08ea99ee11aa68c78b6d2bf7ea912615a1e64a81b90a2abca2dd59cfa64736f6c634300050c0032")

	ex1, err := action.SignedExecution(action.EmptyAddress, priKey0, 1, big.NewInt(0), 500000, big.NewInt(testutil.TestGasPriceInt64), data)
	require.NoError(err)
	require.NoError(ap.Add(context.Background(), ex1))
	_deployHash, err = ex1.Hash()
	require.NoError(err)
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(bc.CommitBlock(blk))

	ap.Reset()
	blockTime := time.Unix(1546329600, 0)
	// get deployed contract address
	var contract string
	if dao != nil {
		r, err := dao.GetReceiptByActionHash(_deployHash, 1)
		require.NoError(err)
		contract = r.ContractAddress
	}
	addOneBlock := func(contract string, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) (hash.Hash256, error) {
		ex1, err := action.SignedExecution(contract, priKey0, nonce, amount, gasLimit, gasPrice, data)
		if err != nil {
			return hash.ZeroHash256, err
		}
		blockTime = blockTime.Add(time.Second)
		if err := ap.Add(context.Background(), ex1); err != nil {
			return hash.ZeroHash256, err
		}
		blk, err = bc.MintNewBlock(blockTime)
		if err != nil {
			return hash.ZeroHash256, err
		}
		if err := bc.CommitBlock(blk); err != nil {
			return hash.ZeroHash256, err
		}
		ex1Hash, err := ex1.Hash()
		if err != nil {
			return hash.ZeroHash256, err
		}
		return ex1Hash, nil
	}

	getBlockHashCallData := func(x int64) []byte {
		funcSig := hash.Hash256b([]byte("getBlockHash(uint256)"))
		// convert block number to uint256 (32-bytes)
		blockNumber := hash.BytesToHash256(big.NewInt(x).Bytes())
		return append(funcSig[:4], blockNumber[:]...)
	}

	var (
		zero     = big.NewInt(0)
		nonce    = uint64(2)
		gasLimit = testutil.TestGasLimit * 5
		gasPrice = big.NewInt(testutil.TestGasPriceInt64)
		bcHash   hash.Hash256
	)
	tests := []struct {
		commitHeight  uint64
		getHashHeight uint64
	}{
		{2, 0},
		{3, 5},
		{4, 1},
		{5, 3},
		{6, 0},
		{7, 6},
		{8, 9},
		{9, 3},
		{10, 9},
		{11, 1},
		{12, 4},
		{13, 0},
		{14, 100},
		{15, 15},
	}
	for _, test := range tests {
		h, err := addOneBlock(contract, nonce, zero, gasLimit, gasPrice, getBlockHashCallData(int64(test.getHashHeight)))
		require.NoError(err)
		r, err := dao.GetReceiptByActionHash(h, test.commitHeight)
		require.NoError(err)
		if test.getHashHeight >= test.commitHeight {
			bcHash = hash.ZeroHash256
		} else if test.commitHeight < g.HawaiiBlockHeight {
			// before hawaii it mistakenly return zero hash
			// see https://github.com/iotexproject/iotex-core/commit/2585b444214f9009b6356fbaf59c992e8728fc01
			bcHash = hash.ZeroHash256
		} else {
			var targetHeight uint64
			if test.commitHeight < g.MidwayBlockHeight {
				targetHeight = test.commitHeight - (test.getHashHeight + 1)
			} else {
				targetHeight = test.getHashHeight
			}
			bcHash, err = dao.GetBlockHash(targetHeight)
			require.NoError(err)
		}
		require.Equal(r.Logs()[0].Topics[0], bcHash)
		nonce++
	}
}

func TestBlockchain_MintNewBlock(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.BlockGasLimit = uint64(100000)
	cfg.Genesis.EnableGravityChainVoting = false
	cfg.ActPool.MinGasPriceStr = "0"
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, rp.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(t, err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	require.NoError(t, err)
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
	bc := blockchain.NewBlockchain(
		cfg,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
	require.NoError(t, ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(t, rewardingProtocol.Register(registry))
	require.NoError(t, bc.Start(ctx))
	defer func() {
		require.NoError(t, bc.Stop(ctx))
	}()

	tsf, err := action.NewTransfer(
		1,
		big.NewInt(100000000),
		identityset.Address(27).String(),
		[]byte{}, uint64(100000),
		big.NewInt(10),
	)
	require.NoError(t, err)

	data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
	execution, err := action.NewExecution(action.EmptyAddress, 2, big.NewInt(0), uint64(100000), big.NewInt(0), data)
	require.NoError(t, err)

	bd := &action.EnvelopeBuilder{}
	elp1 := bd.SetAction(tsf).
		SetNonce(1).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()
	selp1, err := action.Sign(elp1, identityset.PrivateKey(0))
	require.NoError(t, err)
	require.NoError(t, ap.Add(context.Background(), selp1))
	// This execution should not be included in block because block is out of gas
	elp2 := bd.SetAction(execution).
		SetNonce(2).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()
	selp2, err := action.Sign(elp2, identityset.PrivateKey(0))
	require.NoError(t, err)
	require.NoError(t, ap.Add(context.Background(), selp2))

	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(t, err)
	require.Equal(t, 2, len(blk.Actions))
	require.Equal(t, 2, len(blk.Receipts))
	var gasConsumed uint64
	for _, receipt := range blk.Receipts {
		gasConsumed += receipt.GasConsumed
	}
	require.True(t, gasConsumed <= cfg.Genesis.BlockGasLimit)
}

func TestBlockchain_MintNewBlock_PopAccount(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.EnableGravityChainVoting = false
	cfg.ActPool.MinGasPriceStr = "0"
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(t, err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	require.NoError(t, err)
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
	bc := blockchain.NewBlockchain(
		cfg,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, rp.Register(registry))
	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
	require.NoError(t, ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(t, rewardingProtocol.Register(registry))
	require.NoError(t, bc.Start(ctx))
	defer func() {
		require.NoError(t, bc.Stop(ctx))
	}()

	priKey0 := identityset.PrivateKey(27)
	addr1 := identityset.Address(28).String()
	priKey3 := identityset.PrivateKey(30)
	require.NoError(t, addTestingTsfBlocks(cfg, bc, nil, ap))

	// test third block
	bytes := []byte{}
	for i := 0; i < 1000; i++ {
		bytes = append(bytes, 1)
	}
	for i := uint64(0); i < 300; i++ {
		tsf, err := action.SignedTransfer(addr1, priKey0, i+7, big.NewInt(2), bytes,
			19000, big.NewInt(testutil.TestGasPriceInt64))
		require.NoError(t, err)
		require.NoError(t, ap.Add(context.Background(), tsf))
	}
	transfer1, err := action.SignedTransfer(addr1, priKey3, 7, big.NewInt(2),
		[]byte{}, 10000, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(t, err)
	require.NoError(t, ap.Add(context.Background(), transfer1))

	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(t, err)
	require.NotNil(t, blk)
	require.Equal(t, 183, len(blk.Actions))
	whetherInclude := false
	for _, action := range blk.Actions {
		transfer1Hash, err := transfer1.Hash()
		require.NoError(t, err)
		actionHash, err := action.Hash()
		require.NoError(t, err)
		if transfer1Hash == actionHash {
			whetherInclude = true
			break
		}
	}
	require.True(t, whetherInclude)
}

type MockSubscriber struct {
	counter int
	mu      sync.RWMutex
}

func (ms *MockSubscriber) ReceiveBlock(blk *block.Block) error {
	ms.mu.Lock()
	tsfs, _ := action.ClassifyActions(blk.Actions)
	ms.counter += len(tsfs)
	ms.mu.Unlock()
	return nil
}

func (ms *MockSubscriber) Counter() int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.counter
}

func TestConstantinople(t *testing.T) {
	require := require.New(t)
	testValidateBlockchain := func(cfg config.Config, t *testing.T) {
		ctx := context.Background()

		registry := protocol.NewRegistry()
		// Create a blockchain from scratch
		sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption(), factory.RegistryOption(registry))
		require.NoError(err)
		ap, err := actpool.NewActPool(sf, cfg.ActPool)
		require.NoError(err)
		acc := account.NewProtocol(rewarding.DepositGas)
		require.NoError(acc.Register(registry))
		rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
		require.NoError(rp.Register(registry))
		// create indexer
		cfg.DB.DbPath = cfg.Chain.IndexDBPath
		indexer, err := blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
		require.NoError(err)
		// create BlockDAO
		cfg.DB.DbPath = cfg.Chain.ChainDBPath
		cfg.DB.CompressLegacy = cfg.Chain.CompressBlock
		dao := blockdao.NewBlockDAO([]blockdao.BlockIndexer{sf, indexer}, cfg.DB)
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
		ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
		require.NoError(ep.Register(registry))
		rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
		require.NoError(rewardingProtocol.Register(registry))
		require.NoError(bc.Start(ctx))
		defer func() {
			require.NoError(bc.Stop(ctx))
		}()

		require.NoError(addTestingConstantinopleBlocks(bc, dao, sf, ap))

		// reason to use hard-coded hash value here:
		// this test TestConstantinople() is added when we upgrade our EVM to enable Constantinople
		// at that time, the test is run with both EVM version (Byzantium vs. Constantinople), and it generates the
		// same exact block hash, so these values stood as gatekeeper for backward-compatibility
		hashTopic := []struct {
			height  uint64
			h       hash.Hash256
			blkHash string
			topic   []byte
		}{
			{
				1,
				_deployHash,
				"2861aecf2b3f91822de00c9f42ca44276e386ac693df363770783bfc133346c3",
				nil,
			},
			{
				2,
				_setHash,
				"cb0f7895c1fa4f179c0c109835b160d9d1852fce526e12c6b443e86257cadb48",
				_setTopic,
			},
			{
				3,
				_shrHash,
				"c1337e26e157426dd0af058ed37e329d25dd3e34ed606994a6776b59f988f458",
				_shrTopic,
			},
			{
				4,
				_shlHash,
				"cf5c2050a261fa7eca45f31a184c6cd1dc737c7fc3088a0983f659b08985521c",
				_shlTopic,
			},
			{
				5,
				_sarHash,
				"5d76bd9e4be3a60c00761fd141da6bd9c07ab73f472f537845b65679095b0570",
				_sarTopic,
			},
			{
				6,
				_extHash,
				"c5fd9f372b89265f2423737a6d7b680e9759a4a715b22b04ccf875460c310015",
				_extTopic,
			},
			{
				7,
				_crt2Hash,
				"53632287a97e4e118302f2d9b54b3f97f62d3533286c4d4eb955627b3602d3b0",
				_crt2Topic,
			},
		}

		// test getReceipt
		for _, v := range hashTopic {
			ai, err := indexer.GetActionIndex(v.h[:])
			require.NoError(err)
			require.Equal(v.height, ai.BlockHeight())
			r, err := dao.GetReceiptByActionHash(v.h, v.height)
			require.NoError(err)
			require.NotNil(r)
			require.Equal(uint64(1), r.Status)
			require.Equal(v.h, r.ActionHash)
			require.Equal(v.height, r.BlockHeight)
			if v.height == 1 {
				require.Equal("io1va03q4lcr608dr3nltwm64sfcz05czjuycsqgn", r.ContractAddress)
			} else {
				require.Empty(r.ContractAddress)
			}
			a, _, err := dao.GetActionByActionHash(v.h, v.height)
			require.NoError(err)
			require.NotNil(a)
			aHash, err := a.Hash()
			require.NoError(err)
			require.Equal(v.h, aHash)

			blkHash, err := dao.GetBlockHash(v.height)
			require.NoError(err)
			require.Equal(v.blkHash, hex.EncodeToString(blkHash[:]))

			if v.topic != nil {
				funcSig := hash.Hash256b([]byte("Set(uint256)"))
				blk, err := dao.GetBlockByHeight(v.height)
				require.NoError(err)
				f := blk.Header.LogsBloomfilter()
				require.NotNil(f)
				require.True(f.Exist(funcSig[:]))
				require.True(f.Exist(v.topic))
			}
		}

		storeOutGasTests := []struct {
			height      uint64
			actHash     hash.Hash256
			status      iotextypes.ReceiptStatus
			preBalance  *big.Int
			postBalance *big.Int
		}{
			{
				8, _storeHash, iotextypes.ReceiptStatus_ErrCodeStoreOutOfGas, _preGrPreStore, _preGrPostStore,
			},
			{
				9, _store2Hash, iotextypes.ReceiptStatus_ErrCodeStoreOutOfGas, _preGrPostStore, _postGrPostStore,
			},
		}
		caller := identityset.Address(27)
		for _, v := range storeOutGasTests {
			r, err := dao.GetReceiptByActionHash(v.actHash, v.height)
			require.NoError(err)
			require.EqualValues(v.status, r.Status)

			// verify transaction log
			bLog, err := dao.TransactionLogs(v.height)
			require.NoError(err)
			tLog := bLog.Logs[0]
			// first transaction log is gas fee
			tx := tLog.Transactions[0]
			require.Equal(tx.Sender, caller.String())
			require.Equal(tx.Recipient, address.RewardingPoolAddr)
			require.Equal(iotextypes.TransactionLogType_GAS_FEE, tx.Type)
			gasFee, ok := new(big.Int).SetString(tx.Amount, 10)
			require.True(ok)
			postBalance := new(big.Int).Sub(v.preBalance, gasFee)

			if !cfg.Genesis.IsGreenland(v.height) {
				// pre-Greenland contains a tx with status = ReceiptStatus_ErrCodeStoreOutOfGas
				// due to a bug the transfer is not reverted
				require.Equal(2, len(tLog.Transactions))
				// 2nd log is in-contract-transfer
				tx = tLog.Transactions[1]
				require.Equal(tx.Sender, caller.String())
				require.Equal(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, tx.Type)
				tsfAmount, ok := new(big.Int).SetString(tx.Amount, 10)
				require.True(ok)
				postBalance.Sub(postBalance, tsfAmount)
				// post = pre - gasFee - in_contract_transfer
				require.Equal(v.postBalance, postBalance)
			} else {
				// post-Greenland fixed that bug, the transfer is reverted so it only contains the gas fee
				require.Equal(1, len(tLog.Transactions))
				// post = pre - gasFee (transfer is reverted)
				require.Equal(v.postBalance, postBalance)
			}
		}

		// test getActions
		addr27 := hash.BytesToHash160(caller.Bytes())
		total, err := indexer.GetActionCountByAddress(addr27)
		require.NoError(err)
		require.EqualValues(len(hashTopic)+len(storeOutGasTests), total)
		actions, err := indexer.GetActionsByAddress(addr27, 0, total)
		require.NoError(err)
		require.EqualValues(total, len(actions))
		for i := range hashTopic {
			require.Equal(hashTopic[i].h[:], actions[i])
		}
		for i := range storeOutGasTests {
			require.Equal(storeOutGasTests[i].actHash[:], actions[i+len(hashTopic)])
		}
	}

	cfg := config.Default
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
		// clear the gateway
		delete(cfg.Plugins, config.GatewayPlugin)
	}()

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.ProducerPrivKey = "a000000000000000000000000000000000000000000000000000000000000000"
	cfg.Genesis.EnableGravityChainVoting = false
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Genesis.AleutianBlockHeight = 2
	cfg.Genesis.BeringBlockHeight = 8
	cfg.Genesis.GreenlandBlockHeight = 9
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()

	t.Run("test Constantinople contract", func(t *testing.T) {
		testValidateBlockchain(cfg, t)
	})
}

func TestLoadBlockchainfromDB(t *testing.T) {
	require := require.New(t)
	testValidateBlockchain := func(cfg config.Config, t *testing.T) {
		ctx := context.Background()

		registry := protocol.NewRegistry()
		// Create a blockchain from scratch
		sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption(), factory.RegistryOption(registry))
		require.NoError(err)
		ap, err := actpool.NewActPool(sf, cfg.ActPool)
		require.NoError(err)
		acc := account.NewProtocol(rewarding.DepositGas)
		require.NoError(acc.Register(registry))
		rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
		require.NoError(rp.Register(registry))
		var indexer blockindex.Indexer
		indexers := []blockdao.BlockIndexer{sf}
		if _, gateway := cfg.Plugins[config.GatewayPlugin]; gateway && !cfg.Chain.EnableAsyncIndexWrite {
			// create indexer
			cfg.DB.DbPath = cfg.Chain.IndexDBPath
			indexer, err = blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
			require.NoError(err)
			indexers = append(indexers, indexer)
		}
		cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
		// create BlockDAO
		cfg.DB.DbPath = cfg.Chain.ChainDBPath
		cfg.DB.CompressLegacy = cfg.Chain.CompressBlock
		dao := blockdao.NewBlockDAO(indexers, cfg.DB)
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
		ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
		require.NoError(ep.Register(registry))
		require.NoError(bc.Start(ctx))

		ms := &MockSubscriber{counter: 0}
		require.NoError(bc.AddSubscriber(ms))
		require.Equal(0, ms.Counter())

		height := bc.TipHeight()
		fmt.Printf("Open blockchain pass, height = %d\n", height)
		require.NoError(addTestingTsfBlocks(cfg, bc, dao, ap))
		require.NoError(bc.Stop(ctx))
		require.Equal(24, ms.Counter())

		// Load a blockchain from DB
		bc = blockchain.NewBlockchain(
			cfg,
			dao,
			factory.NewMinter(sf, ap),
			blockchain.BlockValidatorOption(block.NewValidator(
				sf,
				protocol.NewGenericValidator(sf, accountutil.AccountState),
			)),
		)
		require.NoError(bc.Start(ctx))
		defer func() {
			require.NoError(bc.Stop(ctx))
		}()

		// verify block header hash
		for i := uint64(1); i <= 5; i++ {
			hash, err := dao.GetBlockHash(i)
			require.NoError(err)
			height, err = dao.GetBlockHeight(hash)
			require.NoError(err)
			require.Equal(i, height)
			header, err := bc.BlockHeaderByHeight(height)
			require.NoError(err)
			require.Equal(height, header.Height())

			// bloomfilter only exists after aleutian height
			require.Equal(height >= cfg.Genesis.AleutianBlockHeight, header.LogsBloomfilter() != nil)
		}

		empblk, err := dao.GetBlock(hash.ZeroHash256)
		require.Nil(empblk)
		require.Error(err)

		header, err := bc.BlockHeaderByHeight(60000)
		require.Nil(header)
		require.Error(err)

		// add wrong blocks
		h := bc.TipHeight()
		blkhash := bc.TipHash()
		header, err = bc.BlockHeaderByHeight(h)
		require.NoError(err)
		require.Equal(blkhash, header.HashBlock())
		fmt.Printf("Current tip = %d hash = %x\n", h, blkhash)

		// add block with wrong height
		selp, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, big.NewInt(50), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
		require.NoError(err)

		nblk, err := block.NewTestingBuilder().
			SetHeight(h + 2).
			SetPrevBlockHash(blkhash).
			SetTimeStamp(testutil.TimestampNow()).
			AddActions(selp).SignAndBuild(identityset.PrivateKey(29))
		require.NoError(err)

		require.Error(bc.ValidateBlock(&nblk))
		fmt.Printf("Cannot validate block %d: %v\n", header.Height(), err)

		// add block with zero prev hash
		selp2, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, big.NewInt(50), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
		require.NoError(err)

		nblk, err = block.NewTestingBuilder().
			SetHeight(h + 1).
			SetPrevBlockHash(hash.ZeroHash256).
			SetTimeStamp(testutil.TimestampNow()).
			AddActions(selp2).SignAndBuild(identityset.PrivateKey(29))
		require.NoError(err)
		err = bc.ValidateBlock(&nblk)
		require.Error(err)
		fmt.Printf("Cannot validate block %d: %v\n", header.Height(), err)

		// add existing block again will have no effect
		blk, err := dao.GetBlockByHeight(3)
		require.NotNil(blk)
		require.NoError(err)
		require.NoError(bc.CommitBlock(blk))
		fmt.Printf("Cannot add block 3 again: %v\n", err)

		// invalid address returns error
		_, err = address.FromString("")
		require.Contains(err.Error(), "address length = 0, expecting 41")

		// valid but unused address should return empty account
		addr, err := address.FromString("io1066kus4vlyvk0ljql39fzwqw0k22h7j8wmef3n")
		require.NoError(err)
		act, err := accountutil.AccountState(sf, addr)
		require.NoError(err)
		require.Equal(uint64(1), act.PendingNonce())
		require.Equal(big.NewInt(0), act.Balance)

		_, gateway := cfg.Plugins[config.GatewayPlugin]
		if gateway && !cfg.Chain.EnableAsyncIndexWrite {
			// verify deployed contract
			ai, err := indexer.GetActionIndex(_deployHash[:])
			require.NoError(err)
			r, err := dao.GetReceiptByActionHash(_deployHash, ai.BlockHeight())
			require.NoError(err)
			require.NotNil(r)
			require.Equal(uint64(1), r.Status)
			require.Equal(uint64(2), r.BlockHeight)

			// 2 topics in block 3 calling set()
			funcSig := hash.Hash256b([]byte("Set(uint256)"))
			blk, err := dao.GetBlockByHeight(3)
			require.NoError(err)
			f := blk.Header.LogsBloomfilter()
			require.NotNil(f)
			require.True(f.Exist(funcSig[:]))
			require.True(f.Exist(_setTopic))
			r, err = dao.GetReceiptByActionHash(_setHash, 3)
			require.NoError(err)
			require.EqualValues(1, r.Status)
			require.EqualValues(3, r.BlockHeight)
			require.Empty(r.ContractAddress)

			// 3 topics in block 4 calling get()
			funcSig = hash.Hash256b([]byte("Get(address,uint256)"))
			blk, err = dao.GetBlockByHeight(4)
			require.NoError(err)
			f = blk.Header.LogsBloomfilter()
			require.NotNil(f)
			require.True(f.Exist(funcSig[:]))
			require.True(f.Exist(_setTopic))
			require.True(f.Exist(_getTopic))
			r, err = dao.GetReceiptByActionHash(_sarHash, 4)
			require.NoError(err)
			require.EqualValues(1, r.Status)
			require.EqualValues(4, r.BlockHeight)
			require.Empty(r.ContractAddress)

			// txIndex/logIndex corrected in block 5
			blk, err = dao.GetBlockByHeight(5)
			require.NoError(err)
			verifyTxLogIndex(require, dao, blk, 10, 2)

			// verify genesis block index
			bi, err := indexer.GetBlockIndex(0)
			require.NoError(err)
			require.Equal(cfg.Genesis.Hash(), hash.BytesToHash256(bi.Hash()))
			require.EqualValues(0, bi.NumAction())
			require.Equal(big.NewInt(0), bi.TsfAmount())

			for h := uint64(1); h <= 5; h++ {
				// verify getting number of actions
				blk, err = dao.GetBlockByHeight(h)
				require.NoError(err)
				blkIndex, err := indexer.GetBlockIndex(h)
				require.NoError(err)
				require.EqualValues(blkIndex.NumAction(), len(blk.Actions))

				// verify getting transfer amount
				tsfs, _ := action.ClassifyActions(blk.Actions)
				tsfa := big.NewInt(0)
				for _, tsf := range tsfs {
					tsfa.Add(tsfa, tsf.Amount())
				}
				require.Equal(blkIndex.TsfAmount(), tsfa)
			}
		}
	}

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

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Genesis.EnableGravityChainVoting = false
	cfg.ActPool.MinGasPriceStr = "0"
	genesis.SetGenesisTimestamp(cfg.Genesis.Timestamp)
	block.LoadGenesisHash(&config.Default.Genesis)

	t.Run("load blockchain from DB w/o explorer", func(t *testing.T) {
		testValidateBlockchain(cfg, t)
	})

	testTriePath2, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath2, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath2, err := testutil.PathOfTempFile("index")
	require.NoError(err)

	defer func() {
		testutil.CleanupPath(testTriePath2)
		testutil.CleanupPath(testDBPath2)
		testutil.CleanupPath(testIndexPath2)
		// clear the gateway
		delete(cfg.Plugins, config.GatewayPlugin)
	}()

	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	cfg.Chain.IndexDBPath = testIndexPath2
	// test using sm2 signature
	cfg.Chain.SignatureScheme = []string{config.SigP256sm2}
	cfg.Chain.ProducerPrivKey = "308193020100301306072a8648ce3d020106082a811ccf5501822d0479307702010104202d57ec7da578b98dad465997748ed02af0c69092ad809598073e5a2356c20492a00a06082a811ccf5501822da14403420004223356f0c6f40822ade24d47b0cd10e9285402cbc8a5028a8eec9efba44b8dfe1a7e8bc44953e557b32ec17039fb8018a58d48c8ffa54933fac8030c9a169bf6"
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.AleutianBlockHeight = 3
	cfg.Genesis.MidwayBlockHeight = 5

	t.Run("load blockchain from DB", func(t *testing.T) {
		testValidateBlockchain(cfg, t)
	})
}

// verify the block contains all tx/log indices up to txIndex and logIndex
func verifyTxLogIndex(r *require.Assertions, dao blockdao.BlockDAO, blk *block.Block, txIndex int, logIndex uint32) {
	r.Equal(txIndex, len(blk.Actions))
	receipts, err := dao.GetReceipts(blk.Height())
	r.NoError(err)
	r.Equal(txIndex, len(receipts))

	logs := make(map[uint32]bool)
	for i := uint32(0); i < logIndex; i++ {
		logs[i] = true
	}
	for i, v := range receipts {
		r.EqualValues(1, v.Status)
		r.EqualValues(i, v.TxIndex)
		h, err := blk.Actions[i].Hash()
		r.NoError(err)
		r.Equal(h, v.ActionHash)
		// verify log index
		for _, l := range v.Logs() {
			r.Equal(h, l.ActionHash)
			r.EqualValues(i, l.TxIndex)
			r.True(logs[l.Index])
			delete(logs, l.Index)
		}
	}
	r.Zero(len(logs))
}

func TestBlockchainInitialCandidate(t *testing.T) {
	require := require.New(t)

	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Consensus.Scheme = config.RollDPoSScheme
	registry := protocol.NewRegistry()
	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption(), factory.RegistryOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	require.NoError(err)
	accountProtocol := account.NewProtocol(rewarding.DepositGas)
	require.NoError(accountProtocol.Register(registry))
	bc := blockchain.NewBlockchain(
		cfg,
		nil,
		factory.NewMinter(sf, ap),
		blockchain.BoltDBDaoOption(sf),
		blockchain.BlockValidatorOption(sf),
	)
	rolldposProtocol := rolldpos.NewProtocol(
		genesis.Default.NumCandidateDelegates,
		genesis.Default.NumDelegates,
		genesis.Default.NumSubEpochs,
	)
	require.NoError(rolldposProtocol.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(rewardingProtocol.Register(registry))
	pollProtocol := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
	require.NoError(pollProtocol.Register(registry))

	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
	}()
	candidate, _, err := candidatesutil.CandidatesFromDB(sf, 1, true, false)
	require.NoError(err)
	require.Equal(24, len(candidate))
}

func TestBlockchain_AccountState(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	// disable account-based testing
	// create chain
	cfg.Genesis.InitBalanceMap[identityset.Address(0).String()] = "100"
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	require.NoError(err)
	bc := blockchain.NewBlockchain(cfg, nil, factory.NewMinter(sf, ap), blockchain.InMemDaoOption(sf))
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)
	s, err := accountutil.AccountState(sf, identityset.Address(0))
	require.NoError(err)
	require.Equal(uint64(1), s.PendingNonce())
	require.Equal(big.NewInt(100), s.Balance)
	require.Equal(hash.ZeroHash256, s.Root)
	require.Equal([]byte(nil), s.CodeHash)
}

func TestBlocks(t *testing.T) {
	// This test is used for committing block verify benchmark purpose
	t.Skip()
	require := require.New(t)
	cfg := config.Default

	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)

	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	c := identityset.Address(29).String()

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	cfg.Genesis.InitBalanceMap[a] = "100000"
	cfg.Genesis.InitBalanceMap[c] = "100000"

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	require.NoError(err)

	// Create a blockchain from scratch
	bc := blockchain.NewBlockchain(cfg, nil, factory.NewMinter(sf, ap), blockchain.BoltDBDaoOption(sf))
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
	}()

	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			Producer: identityset.Address(27),
			GasLimit: gasLimit,
		})
	ctx = genesis.WithGenesisContext(ctx, cfg.Genesis)

	for i := 0; i < 10; i++ {
		actionMap := make(map[string][]action.SealedEnvelope)
		actionMap[a] = []action.SealedEnvelope{}
		for i := 0; i < 1000; i++ {
			tsf, err := action.SignedTransfer(c, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
			require.NoError(err)
			require.NoError(ap.Add(context.Background(), tsf))
		}
		blk, _ := bc.MintNewBlock(testutil.TimestampNow())
		require.NoError(bc.CommitBlock(blk))
	}
}

func TestActions(t *testing.T) {
	// This test is used for block verify benchmark purpose
	t.Skip()
	require := require.New(t)
	cfg := config.Default

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))

	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), registry),
		cfg.Genesis,
	)

	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)

	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	c := identityset.Address(29).String()

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	cfg.Genesis.InitBalanceMap[a] = "100000"
	cfg.Genesis.InitBalanceMap[c] = "100000"

	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	require.NoError(err)
	// Create a blockchain from scratch
	bc := blockchain.NewBlockchain(
		cfg,
		nil,
		factory.NewMinter(sf, ap),
		blockchain.BoltDBDaoOption(sf),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
	}()

	gasLimit := testutil.TestGasLimit
	ctx = protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			Producer: identityset.Address(27),
			GasLimit: gasLimit,
		})
	ctx = genesis.WithGenesisContext(ctx, cfg.Genesis)

	for i := 0; i < 5000; i++ {
		tsf, err := action.SignedTransfer(c, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		require.NoError(err)
		require.NoError(ap.Add(context.Background(), tsf))

		tsf2, err := action.SignedTransfer(a, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		require.NoError(err)
		require.NoError(ap.Add(context.Background(), tsf2))
	}
	blk, _ := bc.MintNewBlock(testutil.TimestampNow())
	ctx = protocol.WithBlockchainCtx(
		ctx,
		protocol.BlockchainCtx{
			Tip: protocol.TipInfo{
				Height: 0,
				Hash:   blk.PrevHash(),
			},
		},
	)
	require.NoError(bc.ValidateBlock(blk))
}

func TestBlockchain_AddRemoveSubscriber(t *testing.T) {
	req := require.New(t)
	cfg := config.Default
	cfg.Genesis.BlockGasLimit = uint64(100000)
	cfg.Genesis.EnableGravityChainVoting = false
	// create chain
	registry := protocol.NewRegistry()
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	req.NoError(err)
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	req.NoError(err)
	bc := blockchain.NewBlockchain(cfg, nil, factory.NewMinter(sf, ap), blockchain.InMemDaoOption(sf))
	// mock
	ctrl := gomock.NewController(t)
	mb := mock_blockcreationsubscriber.NewMockBlockCreationSubscriber(ctrl)
	req.Error(bc.RemoveSubscriber(mb))
	req.NoError(bc.AddSubscriber(mb))
	req.EqualError(bc.AddSubscriber(nil), "subscriber could not be nil")
	req.NoError(bc.RemoveSubscriber(mb))
	req.EqualError(bc.RemoveSubscriber(nil), "cannot find subscription")
}

func TestHistoryForAccount(t *testing.T) {
	testHistoryForAccount(t, false)
	testHistoryForAccount(t, true)
}

func testHistoryForAccount(t *testing.T, statetx bool) {
	require := require.New(t)
	bc, sf, _, _, ap := newChain(t, statetx)
	a := identityset.Address(28)
	priKeyA := identityset.PrivateKey(28)
	b := identityset.Address(29)

	// check the original balance a and b before transfer
	AccountA, err := accountutil.AccountState(sf, a)
	require.NoError(err)
	AccountB, err := accountutil.AccountState(sf, b)
	require.NoError(err)
	require.Equal(big.NewInt(100), AccountA.Balance)
	require.Equal(big.NewInt(100), AccountB.Balance)

	// make a transfer from a to b
	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[a.String()] = []action.SealedEnvelope{}
	tsf, err := action.SignedTransfer(b.String(), priKeyA, 1, big.NewInt(10), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	require.NoError(ap.Add(context.Background(), tsf))
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(bc.ValidateBlock(blk))
	require.NoError(bc.CommitBlock(blk))

	// check balances after transfer
	AccountA, err = accountutil.AccountState(sf, a)
	require.NoError(err)
	AccountB, err = accountutil.AccountState(sf, b)
	require.NoError(err)
	require.Equal(big.NewInt(90), AccountA.Balance)
	require.Equal(big.NewInt(110), AccountB.Balance)

	// check history account's balance
	if statetx {
		_, err = accountutil.AccountState(factory.NewHistoryStateReader(sf, bc.TipHeight()-1), a)
		require.Equal(factory.ErrNotSupported, errors.Cause(err))
		_, err = accountutil.AccountState(factory.NewHistoryStateReader(sf, bc.TipHeight()-1), b)
		require.Equal(factory.ErrNotSupported, errors.Cause(err))
	} else {
		AccountA, err = accountutil.AccountState(factory.NewHistoryStateReader(sf, bc.TipHeight()-1), a)
		require.NoError(err)
		AccountB, err = accountutil.AccountState(factory.NewHistoryStateReader(sf, bc.TipHeight()-1), b)
		require.NoError(err)
		require.Equal(big.NewInt(100), AccountA.Balance)
		require.Equal(big.NewInt(100), AccountB.Balance)
	}
}

func TestHistoryForContract(t *testing.T) {
	testHistoryForContract(t, false)
	testHistoryForContract(t, true)
}

func testHistoryForContract(t *testing.T, statetx bool) {
	require := require.New(t)
	bc, sf, _, dao, ap := newChain(t, statetx)
	genesisAccount := identityset.Address(27).String()
	// deploy and get contract address
	contract := deployXrc20(bc, dao, ap, t)

	contractAddr, err := address.FromString(contract)
	require.NoError(err)
	account, err := accountutil.AccountState(sf, contractAddr)
	require.NoError(err)
	// check the original balance
	balance := BalanceOfContract(contract, genesisAccount, sf, t, account.Root)
	expect, ok := new(big.Int).SetString("2000000000000000000000000000", 10)
	require.True(ok)
	require.Equal(expect, balance)
	// make a transfer for contract
	makeTransfer(contract, bc, ap, t)
	account, err = accountutil.AccountState(sf, contractAddr)
	require.NoError(err)
	// check the balance after transfer
	balance = BalanceOfContract(contract, genesisAccount, sf, t, account.Root)
	expect, ok = new(big.Int).SetString("1999999999999999999999999999", 10)
	require.True(ok)
	require.Equal(expect, balance)

	// check the the original balance again
	if statetx {
		_, err = accountutil.AccountState(factory.NewHistoryStateReader(sf, bc.TipHeight()-1), contractAddr)
		require.True(errors.Cause(err) == factory.ErrNotSupported)
	} else {
		sr := factory.NewHistoryStateReader(sf, bc.TipHeight()-1)
		account, err = accountutil.AccountState(sr, contractAddr)
		require.NoError(err)
		balance = BalanceOfContract(contract, genesisAccount, sr, t, account.Root)
		expect, ok = new(big.Int).SetString("2000000000000000000000000000", 10)
		require.True(ok)
		require.Equal(expect, balance)
	}
}

func deployXrc20(bc blockchain.Blockchain, dao blockdao.BlockDAO, ap actpool.ActPool, t *testing.T) string {
	require := require.New(t)
	genesisPriKey := identityset.PrivateKey(27)
	// deploy a xrc20 contract with balance 2000000000000000000000000000
	data, err := hex.DecodeString("60806040526002805460ff1916601217905534801561001d57600080fd5b506040516107cd3803806107cd83398101604090815281516020808401518385015160025460ff16600a0a84026003819055336000908152600485529586205590850180519395909491019261007592850190610092565b508051610089906001906020840190610092565b5050505061012d565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106100d357805160ff1916838001178555610100565b82800160010185558215610100579182015b828111156101005782518255916020019190600101906100e5565b5061010c929150610110565b5090565b61012a91905b8082111561010c5760008155600101610116565b90565b6106918061013c6000396000f3006080604052600436106100ae5763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166306fdde0381146100b3578063095ea7b31461013d57806318160ddd1461017557806323b872dd1461019c578063313ce567146101c657806342966c68146101f1578063670d14b21461020957806370a082311461022a57806395d89b411461024b578063a9059cbb14610260578063dd62ed3e14610286575b600080fd5b3480156100bf57600080fd5b506100c86102ad565b6040805160208082528351818301528351919283929083019185019080838360005b838110156101025781810151838201526020016100ea565b50505050905090810190601f16801561012f5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561014957600080fd5b50610161600160a060020a036004351660243561033b565b604080519115158252519081900360200190f35b34801561018157600080fd5b5061018a610368565b60408051918252519081900360200190f35b3480156101a857600080fd5b50610161600160a060020a036004358116906024351660443561036e565b3480156101d257600080fd5b506101db6103dd565b6040805160ff9092168252519081900360200190f35b3480156101fd57600080fd5b506101616004356103e6565b34801561021557600080fd5b506100c8600160a060020a036004351661045e565b34801561023657600080fd5b5061018a600160a060020a03600435166104c6565b34801561025757600080fd5b506100c86104d8565b34801561026c57600080fd5b50610284600160a060020a0360043516602435610532565b005b34801561029257600080fd5b5061018a600160a060020a0360043581169060243516610541565b6000805460408051602060026001851615610100026000190190941693909304601f810184900484028201840190925281815292918301828280156103335780601f1061030857610100808354040283529160200191610333565b820191906000526020600020905b81548152906001019060200180831161031657829003601f168201915b505050505081565b336000908152600560209081526040808320600160a060020a039590951683529390529190912055600190565b60035481565b600160a060020a038316600090815260056020908152604080832033845290915281205482111561039e57600080fd5b600160a060020a03841660009081526005602090815260408083203384529091529020805483900390556103d384848461055e565b5060019392505050565b60025460ff1681565b3360009081526004602052604081205482111561040257600080fd5b3360008181526004602090815260409182902080548690039055600380548690039055815185815291517fcc16f5dbb4873280815c1ee09dbd06736cffcc184412cf7a71a0fdb75d397ca59281900390910190a2506001919050565b60066020908152600091825260409182902080548351601f6002600019610100600186161502019093169290920491820184900484028101840190945280845290918301828280156103335780601f1061030857610100808354040283529160200191610333565b60046020526000908152604090205481565b60018054604080516020600284861615610100026000190190941693909304601f810184900484028201840190925281815292918301828280156103335780601f1061030857610100808354040283529160200191610333565b61053d33838361055e565b5050565b600560209081526000928352604080842090915290825290205481565b6000600160a060020a038316151561057557600080fd5b600160a060020a03841660009081526004602052604090205482111561059a57600080fd5b600160a060020a038316600090815260046020526040902054828101116105c057600080fd5b50600160a060020a038083166000818152600460209081526040808320805495891680855282852080548981039091559486905281548801909155815187815291519390950194927fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef929181900390910190a3600160a060020a0380841660009081526004602052604080822054928716825290205401811461065f57fe5b505050505600a165627a7a723058207c03ad12a18902cfe387e684509d310abd583d862c11e3ee80c116af8b49ec5c00290000000000000000000000000000000000000000000000000000000077359400000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000004696f7478000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004696f747800000000000000000000000000000000000000000000000000000000")
	require.NoError(err)
	execution, err := action.NewExecution(action.EmptyAddress, 3, big.NewInt(0), 1000000, big.NewInt(testutil.TestGasPriceInt64), data)
	require.NoError(err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(execution).
		SetNonce(3).
		SetGasLimit(1000000).
		SetGasPrice(big.NewInt(testutil.TestGasPriceInt64)).Build()
	selp, err := action.Sign(elp, genesisPriKey)
	require.NoError(err)

	require.NoError(ap.Add(context.Background(), selp))

	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(bc.CommitBlock(blk))
	selpHash, err := selp.Hash()
	require.NoError(err)
	r, err := dao.GetReceiptByActionHash(selpHash, blk.Height())
	require.NoError(err)
	return r.ContractAddress
}

func BalanceOfContract(contract, genesisAccount string, sr protocol.StateReader, t *testing.T, root hash.Hash256) *big.Int {
	require := require.New(t)
	addr, err := address.FromString(contract)
	require.NoError(err)
	addrHash := hash.BytesToHash160(addr.Bytes())
	dbForTrie := protocol.NewKVStoreForTrieWithStateReader(evm.ContractKVNameSpace, sr)
	options := []mptrie.Option{
		mptrie.KVStoreOption(dbForTrie),
		mptrie.KeyLengthOption(len(hash.Hash256{})),
		mptrie.HashFuncOption(func(data []byte) []byte {
			h := hash.Hash256b(append(addrHash[:], data...))
			return h[:]
		}),
	}
	options = append(options, mptrie.RootHashOption(root[:]))
	tr, err := mptrie.New(options...)
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	defer tr.Stop(context.Background())
	// get producer's xrc20 balance
	addr, err = address.FromString(genesisAccount)
	require.NoError(err)
	addrHash = hash.BytesToHash160(addr.Bytes())
	checkData := "000000000000000000000000" + hex.EncodeToString(addrHash[:]) + "0000000000000000000000000000000000000000000000000000000000000004"
	hb, err := hex.DecodeString(checkData)
	require.NoError(err)
	out2 := crypto.Keccak256(hb)
	ret, err := tr.Get(out2[:])
	require.NoError(err)
	return big.NewInt(0).SetBytes(ret)
}

func newChain(t *testing.T, stateTX bool) (blockchain.Blockchain, factory.Factory, db.KVStore, blockdao.BlockDAO, actpool.ActPool) {
	require := require.New(t)
	cfg := config.Default

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

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.EnableArchiveMode = true
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Genesis.BlockGasLimit = uint64(1000000)
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Genesis.EnableGravityChainVoting = false
	registry := protocol.NewRegistry()
	var sf factory.Factory
	kv := db.NewMemKVStore()
	if stateTX {
		sf, err = factory.NewStateDB(cfg, factory.PrecreatedStateDBOption(kv), factory.RegistryStateDBOption(registry))
		require.NoError(err)
	} else {
		sf, err = factory.NewFactory(cfg, factory.PrecreatedTrieDBOption(kv), factory.RegistryOption(registry))
		require.NoError(err)
	}
	ap, err := actpool.NewActPool(sf, cfg.ActPool)
	require.NoError(err)
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(rp.Register(registry))
	var indexer blockindex.Indexer
	indexers := []blockdao.BlockIndexer{sf}
	if _, gateway := cfg.Plugins[config.GatewayPlugin]; gateway && !cfg.Chain.EnableAsyncIndexWrite {
		// create indexer
		cfg.DB.DbPath = cfg.Chain.IndexDBPath
		indexer, err = blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
		require.NoError(err)
		indexers = append(indexers, indexer)
	}
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	// create BlockDAO
	cfg.DB.DbPath = cfg.Chain.ChainDBPath
	cfg.DB.CompressLegacy = cfg.Chain.CompressBlock
	dao := blockdao.NewBlockDAO(indexers, cfg.DB)
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
	require.NotNil(bc)
	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
	require.NoError(ep.Register(registry))
	require.NoError(bc.Start(context.Background()))

	genesisPriKey := identityset.PrivateKey(27)
	a := identityset.Address(28).String()
	b := identityset.Address(29).String()
	// make a transfer from genesisAccount to a and b,because stateTX cannot store data in height 0
	tsf, err := action.SignedTransfer(a, genesisPriKey, 1, big.NewInt(100), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(b, genesisPriKey, 2, big.NewInt(100), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	require.NoError(ap.Add(context.Background(), tsf))
	require.NoError(ap.Add(context.Background(), tsf2))
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(bc.CommitBlock(blk))
	return bc, sf, kv, dao, ap
}

func makeTransfer(contract string, bc blockchain.Blockchain, ap actpool.ActPool, t *testing.T) *block.Block {
	require := require.New(t)
	genesisPriKey := identityset.PrivateKey(27)
	// make a transfer for contract,transfer 1 to io16eur00s9gdvak4ujhpuk9a45x24n60jgecgxzz
	bytecode, err := hex.DecodeString("a9059cbb0000000000000000000000004867c4bada9553216bf296c4c64e9ff0749206490000000000000000000000000000000000000000000000000000000000000001")
	require.NoError(err)
	execution, err := action.NewExecution(contract, 4, big.NewInt(0), 1000000, big.NewInt(testutil.TestGasPriceInt64), bytecode)
	require.NoError(err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(execution).
		SetNonce(4).
		SetGasLimit(1000000).
		SetGasPrice(big.NewInt(testutil.TestGasPriceInt64)).Build()
	selp, err := action.Sign(elp, genesisPriKey)
	require.NoError(err)
	require.NoError(ap.Add(context.Background(), selp))
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(bc.CommitBlock(blk))
	return blk
}

// TODO: add func TestValidateBlock()
