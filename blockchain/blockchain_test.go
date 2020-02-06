// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
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
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockcreationsubscriber"
	"github.com/iotexproject/iotex-core/testutil"
)

var (
	deployHash   hash.Hash256                                                                           // in block 2
	setHash      hash.Hash256                                                                           // in block 3
	shrHash      hash.Hash256                                                                           // in block 4
	shlHash      hash.Hash256                                                                           // in block 5
	sarHash      hash.Hash256                                                                           // in block 6
	extHash      hash.Hash256                                                                           // in block 7
	crt2Hash     hash.Hash256                                                                           // in block 8
	setTopic, _  = hex.DecodeString("fe00000000000000000000000000000000000000000000000000000000001f40") // in block 3
	getTopic, _  = hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001") // in block 4
	shrTopic, _  = hex.DecodeString("00fe00000000000000000000000000000000000000000000000000000000001f") // in block 4
	shlTopic, _  = hex.DecodeString("fe00000000000000000000000000000000000000000000000000000000001f00") // in block 5
	sarTopic, _  = hex.DecodeString("fffe00000000000000000000000000000000000000000000000000000000001f") // in block 6
	extTopic, _  = hex.DecodeString("4a98ce81a2fd5177f0f42b49cb25b01b720f9ce8019f3937f63b789766c938e2") // in block 7
	crt2Topic, _ = hex.DecodeString("0000000000000000000000001895e6033cd1081f18e0bd23a4501d9376028523") // in block 8
)

func addTestingConstantinopleBlocks(bc Blockchain, dao blockdao.BlockDAO) error {
	// Add block 1
	addr0 := identityset.Address(27).String()
	priKey0 := identityset.PrivateKey(27)
	data, err := hex.DecodeString("608060405234801561001057600080fd5b506104d5806100206000396000f3fe608060405234801561001057600080fd5b50600436106100885760003560e01c806381ea44081161005b57806381ea440814610101578063a91b336214610159578063c2bc2efc14610177578063f5eacece146101cf57610088565b80635bec9e671461008d57806360fe47b1146100975780636bc8ecaa146100c5578063744f5f83146100e3575b600080fd5b6100956101ed565b005b6100c3600480360360208110156100ad57600080fd5b8101908080359060200190929190505050610239565b005b6100cd610270565b6040518082815260200191505060405180910390f35b6100eb6102b3565b6040518082815260200191505060405180910390f35b6101436004803603602081101561011757600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506102f6565b6040518082815260200191505060405180910390f35b61016161036a565b6040518082815260200191505060405180910390f35b6101b96004803603602081101561018d57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506103ad565b6040518082815260200191505060405180910390f35b6101d761045f565b6040518082815260200191505060405180910390f35b5b60011561020b5760008081548092919060010191905055506101ee565b7f8bfaa460932ccf8751604dd60efa3eafa220ec358fccb32ef703f91c509bc3ea60405160405180910390a1565b80600081905550807fdf7a95aebff315db1b7716215d602ab537373cdb769232aae6055c06e798425b60405160405180910390a250565b6000805460081d905080600081905550807fdf7a95aebff315db1b7716215d602ab537373cdb769232aae6055c06e798425b60405160405180910390a280905090565b6000805460081c905080600081905550807fdf7a95aebff315db1b7716215d602ab537373cdb769232aae6055c06e798425b60405160405180910390a280905090565b60008073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff16141561033157600080fd5b813f9050807fdf7a95aebff315db1b7716215d602ab537373cdb769232aae6055c06e798425b60405160405180910390a2809050919050565b6000805460081b905080600081905550807fdf7a95aebff315db1b7716215d602ab537373cdb769232aae6055c06e798425b60405160405180910390a280905090565b60008073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff1614156103e857600080fd5b7fbde7a70c2261170a87678200113c8e12f82f63d0a1d1cfa45681cbac328e87e382600054604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018281526020019250505060405180910390a16000549050919050565b60008080602060406000f59150817fdf7a95aebff315db1b7716215d602ab537373cdb769232aae6055c06e798425b60405160405180910390a2819150509056fea265627a7a72305820209a8ef04c4d621759f34878b27b238650e8605c8a71d6efc619a769a64aa9cc64736f6c634300050a0032")
	if err != nil {
		return err
	}
	ex1, err := testutil.SignedExecution(action.EmptyAddress, priKey0, 1, big.NewInt(0), 500000, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return err
	}
	deployHash = ex1.Hash()
	accMap := make(map[string][]action.SealedEnvelope)
	accMap[addr0] = []action.SealedEnvelope{ex1}
	blockTime := time.Unix(1546329600, 0)
	blk, err := bc.MintNewBlock(
		accMap,
		blockTime,
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// get deployed contract address
	var contract string
	if dao != nil {
		r, err := dao.GetReceiptByActionHash(deployHash, 1)
		if err != nil {
			return err
		}
		contract = r.ContractAddress
	}

	addOneBlock := func(nonce uint64, data []byte) (hash.Hash256, error) {
		ex1, err := testutil.SignedExecution(contract, priKey0, nonce, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data)
		if err != nil {
			return hash.ZeroHash256, err
		}
		accMap := make(map[string][]action.SealedEnvelope)
		accMap[addr0] = []action.SealedEnvelope{ex1}
		blockTime = blockTime.Add(time.Second)
		blk, err = bc.MintNewBlock(
			accMap,
			blockTime,
		)
		if err != nil {
			return hash.ZeroHash256, err
		}
		if err := bc.ValidateBlock(blk); err != nil {
			return hash.ZeroHash256, err
		}
		if err := bc.CommitBlock(blk); err != nil {
			return hash.ZeroHash256, err
		}
		return ex1.Hash(), nil
	}

	// Add block 2
	// call set() to set storedData = 0xfe...1f40
	funcSig := hash.Hash256b([]byte("set(uint256)"))
	data = append(funcSig[:4], setTopic...)
	setHash, err = addOneBlock(2, data)
	if err != nil {
		return err
	}

	// Add block 3
	// call shright() to test SHR opcode, storedData => 0x00fe...1f
	funcSig = hash.Hash256b([]byte("shright()"))
	shrHash, err = addOneBlock(3, funcSig[:4])
	if err != nil {
		return err
	}

	// Add block 4
	// call shleft() to test SHL opcode, storedData => 0xfe...1f00
	funcSig = hash.Hash256b([]byte("shleft()"))
	shlHash, err = addOneBlock(4, funcSig[:4])
	if err != nil {
		return err
	}

	// Add block 5
	// call saright() to test SAR opcode, storedData => 0xfffe...1f
	funcSig = hash.Hash256b([]byte("saright()"))
	sarHash, err = addOneBlock(5, funcSig[:4])
	if err != nil {
		return err
	}

	// Add block 6
	// call getCodeHash() to test EXTCODEHASH opcode
	funcSig = hash.Hash256b([]byte("getCodeHash(address)"))
	addr, _ := address.FromString(contract)
	ethaddr := hash.BytesToHash256(addr.Bytes())
	data = append(funcSig[:4], ethaddr[:]...)
	extHash, err = addOneBlock(6, data)
	if err != nil {
		return err
	}

	// Add block 7
	// call create2() to test CREATE2 opcode
	funcSig = hash.Hash256b([]byte("create2()"))
	crt2Hash, err = addOneBlock(7, funcSig[:4])
	if err != nil {
		return err
	}
	return nil
}

func addTestingTsfBlocks(bc Blockchain, dao blockdao.BlockDAO) error {
	// Add block 1
	addr0 := identityset.Address(27).String()
	tsf0, err := testutil.SignedTransfer(addr0, identityset.PrivateKey(0), 1, big.NewInt(90000000), nil, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	accMap := make(map[string][]action.SealedEnvelope)
	accMap[identityset.Address(0).String()] = []action.SealedEnvelope{tsf0}
	blk, err := bc.MintNewBlock(
		accMap,
		testutil.TimestampNow(),
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

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
	tsf1, err := testutil.SignedTransfer(addr1, priKey0, 1, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2, err := testutil.SignedTransfer(addr2, priKey0, 2, big.NewInt(30), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf3, err := testutil.SignedTransfer(addr3, priKey0, 3, big.NewInt(50), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf4, err := testutil.SignedTransfer(addr4, priKey0, 4, big.NewInt(70), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf5, err := testutil.SignedTransfer(addr5, priKey0, 5, big.NewInt(110), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf6, err := testutil.SignedTransfer(addr6, priKey0, 6, big.NewInt(50<<20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// deploy simple smart contract
	data, _ := hex.DecodeString("608060405234801561001057600080fd5b50610233806100206000396000f300608060405260043610610057576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680635bec9e671461005c57806360fe47b114610073578063c2bc2efc146100a0575b600080fd5b34801561006857600080fd5b506100716100f7565b005b34801561007f57600080fd5b5061009e60048036038101908080359060200190929190505050610143565b005b3480156100ac57600080fd5b506100e1600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061017a565b6040518082815260200191505060405180910390f35b5b6001156101155760008081548092919060010191905055506100f8565b7f8bfaa460932ccf8751604dd60efa3eafa220ec358fccb32ef703f91c509bc3ea60405160405180910390a1565b80600081905550807fdf7a95aebff315db1b7716215d602ab537373cdb769232aae6055c06e798425b60405160405180910390a250565b60008073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff16141515156101b757600080fd5b6000548273ffffffffffffffffffffffffffffffffffffffff167fbde7a70c2261170a87678200113c8e12f82f63d0a1d1cfa45681cbac328e87e360405160405180910390a360005490509190505600a165627a7a723058203198d0390613dab2dff2fa053c1865e802618d628429b01ab05b8458afc347eb0029")
	ex1, err := testutil.SignedExecution(action.EmptyAddress, priKey2, 1, big.NewInt(0), 200000, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return err
	}
	deployHash = ex1.Hash()
	accMap = make(map[string][]action.SealedEnvelope)
	accMap[addr0] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}
	accMap[addr2] = []action.SealedEnvelope{ex1}
	blk, err = bc.MintNewBlock(
		accMap,
		testutil.TimestampNow(),
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// get deployed contract address
	var contract string
	cfg := bc.(*blockchain).config
	_, gateway := cfg.Plugins[config.GatewayPlugin]
	if gateway && !cfg.Chain.EnableAsyncIndexWrite {
		r, err := dao.GetReceiptByActionHash(deployHash, 2)
		if err != nil {
			return err
		}
		contract = r.ContractAddress
	}

	// Add block 3
	// Charlie --> A, B, D, E, test
	tsf1, err = testutil.SignedTransfer(addr1, priKey3, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr2, priKey3, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr4, priKey3, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr5, priKey3, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf5, err = testutil.SignedTransfer(addr0, priKey3, 5, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// call set() to set storedData = 0x1f40
	data, _ = hex.DecodeString("60fe47b1")
	data = append(data, setTopic...)
	ex1, err = testutil.SignedExecution(contract, priKey2, 2, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return err
	}
	accMap = make(map[string][]action.SealedEnvelope)
	accMap[addr3] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5}
	accMap[addr2] = []action.SealedEnvelope{ex1}
	blk, err = bc.MintNewBlock(
		accMap,
		testutil.TimestampNow(),
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	// Delta --> B, E, F, test
	tsf1, err = testutil.SignedTransfer(addr2, priKey4, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr5, priKey4, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr6, priKey4, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr0, priKey4, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	data, _ = hex.DecodeString("c2bc2efc")
	data = append(data, getTopic...)
	ex1, err = testutil.SignedExecution(contract, priKey2, 3, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return err
	}
	accMap = make(map[string][]action.SealedEnvelope)
	accMap[addr4] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4}
	accMap[addr2] = []action.SealedEnvelope{ex1}
	blk, err = bc.MintNewBlock(
		accMap,
		testutil.TimestampNow(),
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 5
	// Delta --> A, B, C, D, F, test
	tsf1, err = testutil.SignedTransfer(addr1, priKey5, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr2, priKey5, 2, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr3, priKey5, 3, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr4, priKey5, 4, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf5, err = testutil.SignedTransfer(addr6, priKey5, 5, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf6, err = testutil.SignedTransfer(addr0, priKey5, 6, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf7, err := testutil.SignedTransfer(addr3, priKey3, 6, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf8, err := testutil.SignedTransfer(addr1, priKey1, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	accMap = make(map[string][]action.SealedEnvelope)
	accMap[addr5] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}
	accMap[addr3] = []action.SealedEnvelope{tsf7}
	accMap[addr1] = []action.SealedEnvelope{tsf8}
	blk, err = bc.MintNewBlock(
		accMap,
		testutil.TimestampNow(),
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
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
	// create chain
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(rp.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	dao := blockdao.NewBlockDAO(db.NewMemKVStore(), nil, cfg.Chain.CompressBlock, cfg.DB)
	bc := NewBlockchain(cfg, dao, sf, RegistryOption(registry))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	ep := execution.NewProtocol(dao.GetBlockHash)
	require.NoError(ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.KickoutIntensityRate, nil, nil)
	require.NoError(rewardingProtocol.Register(registry))
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	height := bc.TipHeight()
	require.Equal(0, int(height))
	fmt.Printf("Create blockchain pass, height = %d\n", height)
	defer func() {
		require.NoError(bc.Stop(ctx))
	}()

	// add 4 sample blocks
	require.NoError(addTestingTsfBlocks(bc, nil))
	height = bc.TipHeight()
	require.Equal(5, int(height))
}

func TestBlockchain_MintNewBlock(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.BlockGasLimit = uint64(100000)
	cfg.Genesis.EnableGravityChainVoting = false
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, rp.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(t, err)
	dao := blockdao.NewBlockDAO(db.NewMemKVStore(), nil, cfg.Chain.CompressBlock, cfg.DB)
	bc := NewBlockchain(cfg, dao, sf, RegistryOption(registry))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	ep := execution.NewProtocol(dao.GetBlockHash)
	require.NoError(t, ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.KickoutIntensityRate, nil, nil)
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
	// This execution should not be included in block because block is out of gas
	elp2 := bd.SetAction(execution).
		SetNonce(2).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()
	selp2, err := action.Sign(elp2, identityset.PrivateKey(0))
	require.NoError(t, err)

	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[identityset.Address(0).String()] = []action.SealedEnvelope{selp1, selp2}

	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
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
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(t, err)
	dao := blockdao.NewBlockDAO(db.NewMemKVStore(), nil, cfg.Chain.CompressBlock, cfg.DB)
	bc := NewBlockchain(cfg, dao, sf, RegistryOption(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, rp.Register(registry))
	ep := execution.NewProtocol(dao.GetBlockHash)
	require.NoError(t, ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.KickoutIntensityRate, nil, nil)
	require.NoError(t, rewardingProtocol.Register(registry))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	require.NoError(t, bc.Start(ctx))
	defer func() {
		require.NoError(t, bc.Stop(ctx))
	}()

	addr0 := identityset.Address(27).String()
	priKey0 := identityset.PrivateKey(27)
	addr1 := identityset.Address(28).String()
	addr3 := identityset.Address(30).String()
	priKey3 := identityset.PrivateKey(30)
	require.NoError(t, addTestingTsfBlocks(bc, nil))

	// test third block
	bytes := []byte{}
	for i := 0; i < 1000; i++ {
		bytes = append(bytes, 1)
	}
	actionMap := make(map[string][]action.SealedEnvelope)
	actions := make([]action.SealedEnvelope, 0)
	for i := uint64(0); i < 300; i++ {
		tsf, err := testutil.SignedTransfer(addr1, priKey0, i+7, big.NewInt(2), bytes,
			1000000, big.NewInt(testutil.TestGasPriceInt64))
		require.NoError(t, err)
		actions = append(actions, tsf)
	}
	actionMap[addr0] = actions
	transfer1, err := testutil.SignedTransfer(addr1, priKey3, 7, big.NewInt(2),
		[]byte{}, 100000, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(t, err)
	actionMap[addr3] = []action.SealedEnvelope{transfer1}

	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	require.NoError(t, err)
	require.NotNil(t, blk)
	require.Equal(t, 183, len(blk.Actions))
	whetherInclude := false
	for _, action := range blk.Actions {
		if transfer1.Hash() == action.Hash() {
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
	testValidateBlockchain := func(cfg config.Config, t *testing.T) {
		require := require.New(t)
		ctx := context.Background()

		// Create a blockchain from scratch
		sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
		require.NoError(err)
		acc := account.NewProtocol(rewarding.DepositGas)
		registry := protocol.NewRegistry()
		require.NoError(acc.Register(registry))
		rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
		require.NoError(rp.Register(registry))
		// create indexer
		cfg.DB.DbPath = cfg.Chain.IndexDBPath
		indexer, err := blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
		require.NoError(err)
		// create BlockDAO
		cfg.DB.DbPath = cfg.Chain.ChainDBPath
		dao := blockdao.NewBlockDAO(db.NewBoltDB(cfg.DB), indexer, cfg.Chain.CompressBlock, cfg.DB)
		require.NotNil(dao)
		bc := NewBlockchain(
			cfg,
			dao,
			sf,
			RegistryOption(registry),
		)
		bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
		ep := execution.NewProtocol(dao.GetBlockHash)
		require.NoError(ep.Register(registry))
		rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.KickoutIntensityRate, nil, nil)
		require.NoError(rewardingProtocol.Register(registry))
		require.NoError(bc.Start(ctx))
		defer func() {
			require.NoError(bc.Stop(ctx))
		}()

		require.NoError(addTestingConstantinopleBlocks(bc, dao))

		// TODO: don't use hard coded hash
		hashTopic := []struct {
			h       hash.Hash256
			blkHash string
			topic   []byte
		}{
			{
				deployHash,
				"7c83ca545dbf2527197dc87b866ebfc1797afa88701f6970ba69c2dd9e871c35",
				nil,
			},
			{
				setHash,
				"94efe3933fdb193308254607d53b2f2d58eb5e6de5507a079253e1042c600819",
				setTopic,
			},
			{
				shrHash,
				"c1a7c75b3b9acd3d943f56c637dcfe18839c577209b4234bef36ced6d96dff82",
				shrTopic,
			},
			{
				shlHash,
				"ad47c75133b09159746ec2341e4fabd563f48ef224444473a6ed64329ee2c412",
				shlTopic,
			},
			{
				sarHash,
				"d9ce02764a2b425d62e693acc85e0701b135075987b09bb4f06e22d2924d2093",
				sarTopic,
			},
			{
				extHash,
				"f553dbd4d76e6a8324bd58cf14e0e615ea71523a98ae2b4704653d778e9fd3ef",
				extTopic,
			},
			{
				crt2Hash,
				"382b34f18894ea335964b9f061adc8f0b34c8b44e333a782f624b3cf1c2ba3d8",
				crt2Topic,
			},
		}

		// test getReceipt
		for i := range hashTopic {
			actHash := hashTopic[i].h
			ai, err := indexer.GetActionIndex(actHash[:])
			require.NoError(err)
			r, err := dao.GetReceiptByActionHash(actHash, ai.BlockHeight())
			require.NoError(err)
			require.NotNil(r)
			require.Equal(uint64(1), r.Status)
			require.Equal(actHash, r.ActionHash)
			require.Equal(uint64(i)+1, r.BlockHeight)
			a, err := dao.GetActionByActionHash(actHash, ai.BlockHeight())
			require.NoError(err)
			require.NotNil(a)
			require.Equal(actHash, a.Hash())

			actIndex, err := indexer.GetActionIndex(actHash[:])
			require.NoError(err)
			blkHash, err := dao.GetBlockHash(actIndex.BlockHeight())
			require.NoError(err)
			require.Equal(hashTopic[i].blkHash, hex.EncodeToString(blkHash[:]))

			if hashTopic[i].topic != nil {
				funcSig := hash.Hash256b([]byte("Set(uint256)"))
				blk, err := dao.GetBlockByHeight(1 + uint64(i))
				require.NoError(err)
				f := blk.Header.LogsBloomfilter()
				require.NotNil(f)
				require.True(f.Exist(funcSig[:]))
				require.True(f.Exist(hashTopic[i].topic))
			}
		}

		// test getActions
		addr27 := hash.BytesToHash160(identityset.Address(27).Bytes())
		total, err := indexer.GetActionCountByAddress(addr27)
		require.NoError(err)
		require.EqualValues(7, total)
		actions, err := indexer.GetActionsByAddress(addr27, 0, total)
		require.NoError(err)
		require.EqualValues(total, len(actions))
		for i := range actions {
			require.Equal(hashTopic[i].h[:], actions[i])
		}
	}

	cfg := config.Default
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()
	testIndexFile, _ := ioutil.TempFile(os.TempDir(), "index")
	testIndexPath := testIndexFile.Name()
	defer func() {
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
		testutil.CleanupPath(t, testIndexPath)
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
	cfg.Genesis.AleutianBlockHeight = 2
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()

	t.Run("test Constantinople contract", func(t *testing.T) {
		testValidateBlockchain(cfg, t)
	})
}

func TestLoadBlockchainfromDB(t *testing.T) {
	testValidateBlockchain := func(cfg config.Config, t *testing.T) {
		require := require.New(t)
		ctx := context.Background()

		// Create a blockchain from scratch
		sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
		require.NoError(err)
		acc := account.NewProtocol(rewarding.DepositGas)
		registry := protocol.NewRegistry()
		require.NoError(acc.Register(registry))
		rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
		require.NoError(rp.Register(registry))
		var indexer blockindex.Indexer
		if _, gateway := cfg.Plugins[config.GatewayPlugin]; gateway && !cfg.Chain.EnableAsyncIndexWrite {
			// create indexer
			cfg.DB.DbPath = cfg.Chain.IndexDBPath
			indexer, err = blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
			require.NoError(err)
		}
		cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
		// create BlockDAO
		cfg.DB.DbPath = cfg.Chain.ChainDBPath
		dao := blockdao.NewBlockDAO(db.NewBoltDB(cfg.DB), indexer, cfg.Chain.CompressBlock, cfg.DB)
		require.NotNil(dao)
		bc := NewBlockchain(
			cfg,
			dao,
			sf,
			RegistryOption(registry),
		)
		ep := execution.NewProtocol(dao.GetBlockHash)
		require.NoError(ep.Register(registry))
		bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
		require.NoError(bc.Start(ctx))

		ms := &MockSubscriber{counter: 0}
		require.NoError(bc.AddSubscriber(ms))
		require.Equal(0, ms.Counter())

		height := bc.TipHeight()
		fmt.Printf("Open blockchain pass, height = %d\n", height)
		require.NoError(addTestingTsfBlocks(bc, dao))
		require.NoError(bc.Stop(ctx))
		require.Equal(24, ms.Counter())

		// Load a blockchain from DB
		accountProtocol := account.NewProtocol(rewarding.DepositGas)
		registry = protocol.NewRegistry()
		require.NoError(accountProtocol.Register(registry))
		bc = NewBlockchain(
			cfg,
			dao,
			sf,
			RegistryOption(registry),
		)
		rolldposProtocol := rolldpos.NewProtocol(
			genesis.Default.NumCandidateDelegates,
			genesis.Default.NumDelegates,
			genesis.Default.NumSubEpochs,
		)
		require.NoError(rolldposProtocol.Register(registry))
		rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.KickoutIntensityRate, nil, nil)
		require.NoError(rewardingProtocol.Register(registry))
		bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
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
		selp, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, big.NewInt(50), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
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
		selp2, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, big.NewInt(50), nil, genesis.Default.ActionGasLimit, big.NewInt(0))
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
		require.NoError(bc.(*blockchain).CommitBlock(blk))
		fmt.Printf("Cannot add block 3 again: %v\n", err)

		// invalid address returns error
		act, err := accountutil.AccountState(sf, "")
		require.Equal("invalid bech32 string length 0", errors.Cause(err).Error())
		require.Nil(act)

		// valid but unused address should return empty account
		act, err = accountutil.AccountState(sf, "io1066kus4vlyvk0ljql39fzwqw0k22h7j8wmef3n")
		require.NoError(err)
		require.Equal(uint64(0), act.Nonce)
		require.Equal(big.NewInt(0), act.Balance)

		_, gateway := cfg.Plugins[config.GatewayPlugin]
		if gateway && !cfg.Chain.EnableAsyncIndexWrite {
			// verify deployed contract
			ai, err := indexer.GetActionIndex(deployHash[:])
			require.NoError(err)
			r, err := dao.GetReceiptByActionHash(deployHash, ai.BlockHeight())
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
			require.True(f.Exist(setTopic))

			// 3 topics in block 4 calling get()
			funcSig = hash.Hash256b([]byte("Get(address,uint256)"))
			blk, err = dao.GetBlockByHeight(4)
			require.NoError(err)
			f = blk.Header.LogsBloomfilter()
			require.NotNil(f)
			require.True(f.Exist(funcSig[:]))
			require.True(f.Exist(setTopic))
			require.True(f.Exist(getTopic))

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

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()
	testIndexFile, _ := ioutil.TempFile(os.TempDir(), "index")
	testIndexPath := testIndexFile.Name()
	defer func() {
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
		testutil.CleanupPath(t, testIndexPath)
	}()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Genesis.EnableGravityChainVoting = false

	t.Run("load blockchain from DB w/o explorer", func(t *testing.T) {
		testValidateBlockchain(cfg, t)
	})

	testTrieFile, _ = ioutil.TempFile(os.TempDir(), "trie")
	testTriePath2 := testTrieFile.Name()
	testDBFile, _ = ioutil.TempFile(os.TempDir(), "db")
	testDBPath2 := testDBFile.Name()
	testIndexFile2, _ := ioutil.TempFile(os.TempDir(), "index")
	testIndexPath2 := testIndexFile2.Name()
	defer func() {
		testutil.CleanupPath(t, testTriePath2)
		testutil.CleanupPath(t, testDBPath2)
		testutil.CleanupPath(t, testIndexPath2)
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

	t.Run("load blockchain from DB", func(t *testing.T) {
		testValidateBlockchain(cfg, t)
	})
}

func TestBlockchain_Validator(t *testing.T) {
	cfg := config.Default
	// disable account-based testing
	cfg.Chain.TrieDBPath = ""

	ctx := context.Background()
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(t, err)
	bc := NewBlockchain(cfg, nil, sf, InMemDaoOption())
	require.NoError(t, bc.Start(ctx))
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(t, err)
	}()
	require.NotNil(t, bc)

	val := bc.Validator()
	require.NotNil(t, bc)
	bc.SetValidator(val)
	require.NotNil(t, bc.Validator())
}

func TestBlockchainInitialCandidate(t *testing.T) {
	require := require.New(t)

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()
	testIndexFile, _ := ioutil.TempFile(os.TempDir(), "index")
	testIndexPath := testIndexFile.Name()

	cfg := config.Default
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Consensus.Scheme = config.RollDPoSScheme
	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	accountProtocol := account.NewProtocol(rewarding.DepositGas)
	registry := protocol.NewRegistry()
	require.NoError(accountProtocol.Register(registry))
	bc := NewBlockchain(
		cfg,
		nil,
		sf,
		BoltDBDaoOption(),
		RegistryOption(registry),
	)
	rolldposProtocol := rolldpos.NewProtocol(
		genesis.Default.NumCandidateDelegates,
		genesis.Default.NumDelegates,
		genesis.Default.NumSubEpochs,
	)
	require.NoError(rolldposProtocol.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.KickoutIntensityRate, nil, nil)
	require.NoError(rewardingProtocol.Register(registry))
	pollProtocol := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
	require.NoError(pollProtocol.Register(registry))

	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
	}()
	candidate, err := candidatesutil.CandidatesByHeight(sf, 1)
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
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	bc := NewBlockchain(cfg, nil, sf, InMemDaoOption(), RegistryOption(registry))
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)
	s, err := accountutil.AccountState(sf, identityset.Address(0).String())
	require.NoError(err)
	require.Equal(uint64(0), s.Nonce)
	require.Equal(big.NewInt(100), s.Balance)
	require.Equal(hash.ZeroHash256, s.Root)
	require.Equal([]byte(nil), s.CodeHash)
}

func TestBlocks(t *testing.T) {
	// This test is used for committing block verify benchmark purpose
	t.Skip()
	require := require.New(t)
	cfg := config.Default

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()
	testIndexFile, _ := ioutil.TempFile(os.TempDir(), "index")
	testIndexPath := testIndexFile.Name()
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
	sf, _ := factory.NewFactory(cfg, factory.InMemTrieOption())

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, nil, sf, BoltDBDaoOption(), RegistryOption(registry))
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
	}()

	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			Producer: identityset.Address(27),
			GasLimit: gasLimit,
		})
	ctx = protocol.WithBlockchainCtx(ctx,
		protocol.BlockchainCtx{
			Genesis: cfg.Genesis,
		})

	for i := 0; i < 10; i++ {
		actionMap := make(map[string][]action.SealedEnvelope)
		actionMap[a] = []action.SealedEnvelope{}
		for i := 0; i < 1000; i++ {
			tsf, err := testutil.SignedTransfer(c, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
			require.NoError(err)
			actionMap[a] = append(actionMap[a], tsf)
		}
		blk, _ := bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
		)
		require.NoError(bc.ValidateBlock(blk))
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

	ctx := protocol.WithBlockchainCtx(
		context.Background(),
		protocol.BlockchainCtx{Genesis: cfg.Genesis, Registry: registry},
	)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()
	testIndexFile, _ := ioutil.TempFile(os.TempDir(), "index")
	testIndexPath := testIndexFile.Name()

	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	c := identityset.Address(29).String()

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	cfg.Genesis.InitBalanceMap[a] = "100000"
	cfg.Genesis.InitBalanceMap[c] = "100000"

	sf, _ := factory.NewFactory(cfg, factory.InMemTrieOption())

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, nil, sf, BoltDBDaoOption(), RegistryOption(registry))
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
	}()

	gasLimit := testutil.TestGasLimit
	ctx = protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			Producer: identityset.Address(27),
			GasLimit: gasLimit,
		})
	ctx = protocol.WithBlockchainCtx(ctx,
		protocol.BlockchainCtx{
			Genesis: cfg.Genesis,
		})

	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	actionMap := make(map[string][]action.SealedEnvelope)
	for i := 0; i < 5000; i++ {
		tsf, err := testutil.SignedTransfer(c, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		require.NoError(err)
		actionMap[a] = append(actionMap[a], tsf)

		tsf2, err := testutil.SignedTransfer(a, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		require.NoError(err)
		actionMap[a] = append(actionMap[a], tsf2)
	}
	blk, _ := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	val := &validator{sf: sf, validatorAddr: ""}
	ctx = protocol.WithBlockchainCtx(
		ctx,
		protocol.BlockchainCtx{
			Genesis: cfg.Genesis,
			Tip: protocol.TipInfo{
				Height: 0,
				Hash:   blk.PrevHash(),
			},
		},
	)
	require.NoError(val.Validate(ctx, blk))
}

func TestBlockchain_AddSubscriber(t *testing.T) {
	req := require.New(t)
	cfg := config.Default
	cfg.Genesis.BlockGasLimit = uint64(100000)
	cfg.Genesis.EnableGravityChainVoting = false
	// create chain
	registry := protocol.NewRegistry()
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	req.NoError(err)
	bc := NewBlockchain(cfg, nil, sf, InMemDaoOption(), RegistryOption(registry))
	// mock
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mb := mock_blockcreationsubscriber.NewMockBlockCreationSubscriber(ctrl)
	req.NoError(bc.AddSubscriber(mb))
	req.EqualError(bc.AddSubscriber(nil), "subscriber could not be nil")
}

func TestBlockchain_RemoveSubscriber(t *testing.T) {
	req := require.New(t)
	cfg := config.Default
	cfg.Genesis.BlockGasLimit = uint64(100000)
	cfg.Genesis.EnableGravityChainVoting = false
	// create chain
	registry := protocol.NewRegistry()
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	req.NoError(err)
	bc := NewBlockchain(cfg, nil, sf, InMemDaoOption(), RegistryOption(registry))
	// mock
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mb := mock_blockcreationsubscriber.NewMockBlockCreationSubscriber(ctrl)
	req.Error(bc.RemoveSubscriber(mb))
	req.NoError(bc.AddSubscriber(mb))
	req.NoError(bc.RemoveSubscriber(mb))
	req.EqualError(bc.RemoveSubscriber(nil), "cannot find subscription")
}

func TestHistoryForAccount(t *testing.T) {
	testHistoryForAccount(t, false)
	testHistoryForAccount(t, true)
}

func testHistoryForAccount(t *testing.T, statetx bool) {
	require := require.New(t)
	bc, sf, _ := newChain(t, statetx)
	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	b := identityset.Address(29).String()

	// check the original balance a and b before transfer
	AccountA, err := accountutil.AccountState(sf, a)
	require.NoError(err)
	AccountB, err := accountutil.AccountState(sf, b)
	require.NoError(err)
	require.Equal(big.NewInt(100), AccountA.Balance)
	require.Equal(big.NewInt(100), AccountB.Balance)

	// make a transfer from a to b
	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[a] = []action.SealedEnvelope{}
	tsf, err := testutil.SignedTransfer(b, priKeyA, 1, big.NewInt(10), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	actionMap[a] = append(actionMap[a], tsf)
	blk, _ := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
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
		_, err = accountutil.AccountStateAtHeight(sf, a, bc.TipHeight()-1)
		require.Equal(factory.ErrNotSupported, errors.Cause(err))
		_, err = accountutil.AccountStateAtHeight(sf, b, bc.TipHeight()-1)
		require.Equal(factory.ErrNotSupported, errors.Cause(err))
	} else {
		AccountA, err = accountutil.AccountStateAtHeight(sf, a, bc.TipHeight()-1)
		require.NoError(err)
		AccountB, err = accountutil.AccountStateAtHeight(sf, b, bc.TipHeight()-1)
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
	bc, sf, dao := newChain(t, statetx)
	genesisAccount := identityset.Address(27).String()
	// deploy and get contract address
	contract := deployXrc20(bc, dao, t)

	account, err := accountutil.AccountState(sf, contract)
	require.NoError(err)
	// check the original balance
	balance := BalanceOfContract(contract, genesisAccount, sf, t, account.Root)
	expect, ok := big.NewInt(0).SetString("2000000000000000000000000000", 10)
	require.True(ok)
	require.Equal(expect, balance)
	// make a transfer for contract
	makeTransfer(contract, bc, t)
	account, err = accountutil.AccountState(sf, contract)
	require.NoError(err)
	// check the balance after transfer
	balance = BalanceOfContract(contract, genesisAccount, sf, t, account.Root)
	expect, ok = big.NewInt(0).SetString("1999999999999999999999999999", 10)
	require.True(ok)
	require.Equal(expect, balance)

	// check the the original balance again
	if statetx {
		_, err = accountutil.AccountStateAtHeight(sf, contract, bc.TipHeight()-1)
		require.True(errors.Cause(err) == factory.ErrNotSupported)
	} else {
		account, err = accountutil.AccountStateAtHeight(sf, contract, bc.TipHeight()-1)
		require.NoError(err)
		balance = BalanceOfContract(contract, genesisAccount, sf, t, account.Root)
		expect, ok = big.NewInt(0).SetString("2000000000000000000000000000", 10)
		require.True(ok)
		require.Equal(expect, balance)
	}
}

func deployXrc20(bc Blockchain, dao blockdao.BlockDAO, t *testing.T) string {
	require := require.New(t)
	genesisAccount := identityset.Address(27).String()
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

	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[genesisAccount] = []action.SealedEnvelope{selp}

	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	require.NoError(err)
	require.NoError(bc.ValidateBlock(blk))
	require.NoError(bc.CommitBlock(blk))
	r, err := dao.GetReceiptByActionHash(selp.Hash(), blk.Height())
	require.NoError(err)
	return r.ContractAddress
}

func BalanceOfContract(contract, genesisAccount string, sf factory.Factory, t *testing.T, root hash.Hash256) *big.Int {
	require := require.New(t)
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	kv := ws.GetDB()
	addr, err := address.FromString(contract)
	require.NoError(err)
	addrHash := hash.BytesToHash160(addr.Bytes())
	dbForTrie, err := db.NewKVStoreForTrie(evm.ContractKVNameSpace, kv)
	require.NoError(err)
	options := []trie.Option{
		trie.KVStoreOption(dbForTrie),
		trie.KeyLengthOption(len(hash.Hash256{})),
		trie.HashFuncOption(func(data []byte) []byte {
			return trie.DefaultHashFunc(append(addrHash[:], data...))
		}),
	}
	options = append(options, trie.RootHashOption(root[:]))
	tr, err := trie.NewTrie(options...)
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

func newChain(t *testing.T, statetx bool) (Blockchain, factory.Factory, blockdao.BlockDAO) {
	require := require.New(t)
	cfg := config.Default
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()
	testIndexFile, _ := ioutil.TempFile(os.TempDir(), "index")
	testIndexPath := testIndexFile.Name()
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.EnableArchiveMode = true
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Genesis.BlockGasLimit = uint64(1000000)
	cfg.Genesis.EnableGravityChainVoting = false
	var sf factory.Factory
	var err error
	if statetx {
		sf, err = factory.NewStateDB(cfg, factory.DefaultStateDBOption())
		require.NoError(err)
	} else {
		sf, err = factory.NewFactory(cfg, factory.DefaultTrieOption())
		require.NoError(err)
	}
	acc := account.NewProtocol(rewarding.DepositGas)
	registry := protocol.NewRegistry()
	require.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(rp.Register(registry))
	var indexer blockindex.Indexer
	if _, gateway := cfg.Plugins[config.GatewayPlugin]; gateway && !cfg.Chain.EnableAsyncIndexWrite {
		// create indexer
		cfg.DB.DbPath = cfg.Chain.IndexDBPath
		indexer, err = blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
		require.NoError(err)
	}
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	// create BlockDAO
	cfg.DB.DbPath = cfg.Chain.ChainDBPath
	dao := blockdao.NewBlockDAO(db.NewBoltDB(cfg.DB), indexer, cfg.Chain.CompressBlock, cfg.DB)
	require.NotNil(dao)
	bc := NewBlockchain(
		cfg,
		dao,
		sf,
		RegistryOption(registry),
	)
	require.NotNil(bc)
	ep := execution.NewProtocol(dao.GetBlockHash)
	require.NoError(ep.Register(registry))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))
	require.NoError(bc.Start(context.Background()))

	genesisAccount := identityset.Address(27).String()
	genesisPriKey := identityset.PrivateKey(27)
	a := identityset.Address(28).String()
	b := identityset.Address(29).String()
	// make a transfer from genesisAccount to a and b,because statetx cannot store data in height 0
	tsf, err := testutil.SignedTransfer(a, genesisPriKey, 1, big.NewInt(100), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	tsf2, err := testutil.SignedTransfer(b, genesisPriKey, 2, big.NewInt(100), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[genesisAccount] = []action.SealedEnvelope{tsf, tsf2}
	blk, _ := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	require.NoError(bc.ValidateBlock(blk))
	require.NoError(bc.CommitBlock(blk))
	return bc, sf, dao
}

func makeTransfer(contract string, bc Blockchain, t *testing.T) *block.Block {
	require := require.New(t)
	genesisAccount := identityset.Address(27).String()
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
	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[genesisAccount] = []action.SealedEnvelope{selp}
	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	require.NoError(err)
	require.NoError(bc.ValidateBlock(blk))
	require.NoError(bc.CommitBlock(blk))
	return blk
}
