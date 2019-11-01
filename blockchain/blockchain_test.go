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

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
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
	registry := protocol.Registry{}
	hu := config.NewHeightUpgrade(cfg)
	acc := account.NewProtocol(hu)
	require.NoError(registry.Register(account.ProtocolID, acc))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(registry.Register(rolldpos.ProtocolID, rp))
	bc := NewBlockchain(cfg, nil, InMemStateFactoryOption(), InMemDaoOption(), RegistryOption(&registry))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	exec := execution.NewProtocol(bc, hu)
	require.NoError(registry.Register(execution.ProtocolID, exec))
	bc.Validator().AddActionValidators(acc, exec)
	bc.GetFactory().AddActionHandlers(acc, exec)
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	height := bc.TipHeight()
	require.Equal(0, int(height))
	fmt.Printf("Create blockchain pass, height = %d\n", height)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(err)
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
	registry := protocol.Registry{}
	hu := config.NewHeightUpgrade(cfg)
	acc := account.NewProtocol(hu)
	require.NoError(t, registry.Register(account.ProtocolID, acc))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, registry.Register(rolldpos.ProtocolID, rp))
	bc := NewBlockchain(cfg, nil, InMemStateFactoryOption(), InMemDaoOption(), RegistryOption(&registry))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	exec := execution.NewProtocol(bc, hu)
	require.NoError(t, registry.Register(execution.ProtocolID, exec))
	bc.Validator().AddActionValidators(acc, exec)
	bc.GetFactory().AddActionHandlers(acc, exec)
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
	require.Equal(t, 1, len(blk.Receipts))
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
	registry := protocol.Registry{}
	hu := config.NewHeightUpgrade(cfg)
	acc := account.NewProtocol(hu)
	require.NoError(t, registry.Register(account.ProtocolID, acc))
	bc := NewBlockchain(cfg, nil, InMemStateFactoryOption(), InMemDaoOption(), RegistryOption(&registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, registry.Register(rolldpos.ProtocolID, rp))
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	exec := execution.NewProtocol(bc, hu)
	require.NoError(t, registry.Register(execution.ProtocolID, exec))
	bc.Validator().AddActionValidators(acc, exec)
	bc.GetFactory().AddActionHandlers(acc, exec)
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

func (ms *MockSubscriber) HandleBlock(blk *block.Block) error {
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
		hc := config.NewHeightUpgrade(cfg)
		acc := account.NewProtocol(hc)
		sf.AddActionHandlers(acc)
		registry := protocol.Registry{}
		require.NoError(registry.Register(account.ProtocolID, acc))
		rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
		require.NoError(registry.Register(rolldpos.ProtocolID, rp))
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
			PrecreatedStateFactoryOption(sf),
			RegistryOption(&registry),
		)
		bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
		exec := execution.NewProtocol(bc, hc)
		require.NoError(registry.Register(execution.ProtocolID, exec))
		bc.Validator().AddActionValidators(acc, exec)
		sf.AddActionHandlers(exec)
		require.NoError(bc.Start(ctx))
		require.NoError(addCreatorToFactory(sf))
		defer func() {
			require.NoError(bc.Stop(ctx))
		}()

		require.NoError(addTestingConstantinopleBlocks(bc, dao))

		hashTopic := []struct {
			h       hash.Hash256
			blkHash string
			topic   []byte
		}{
			{
				deployHash,
				"d1ff0e7fe2a54600a171d3bcc9e222c656d584b3a0e7b33373e634de3f8cd010",
				nil,
			},
			{
				setHash,
				"24667a8d9ca9f4d8c1bc651b9be205cc8422aca36dba8895aa39c50a8937be09",
				setTopic,
			},
			{
				shrHash,
				"fd8ef98e94689d4a69fc828693dc931c48767b53dec717329bbac043c21fa78c",
				shrTopic,
			},
			{
				shlHash,
				"77d0861e5e7164691c71fe5031087dda5ea20039bd096feaae9d8166bdf6a6a9",
				shlTopic,
			},
			{
				sarHash,
				"7946fa90bd7c25f84bf83f727cc4589abc690d488ec8fa4f4af2ec9d19c71e74",
				sarTopic,
			},
			{
				extHash,
				"0d35e9623375411f39c701ddf78f743abf3615f732977c01966a2fe359ae46f9",
				extTopic,
			},
			{
				crt2Hash,
				"63f147cfecd0a58a9d6211886b53533cfe3ae57a539a2fecab05b27beab04e69",
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
			blkHash, err := bc.GetHashByHeight(actIndex.BlockHeight())
			require.NoError(err)
			require.Equal(hashTopic[i].blkHash, hex.EncodeToString(blkHash[:]))

			if hashTopic[i].topic != nil {
				funcSig := hash.Hash256b([]byte("Set(uint256)"))
				blk, err := bc.GetBlockByHeight(1 + uint64(i))
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
		hu := config.NewHeightUpgrade(cfg)
		acc := account.NewProtocol(hu)
		sf.AddActionHandlers(acc)
		registry := protocol.Registry{}
		require.NoError(registry.Register(account.ProtocolID, acc))
		rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
		require.NoError(registry.Register(rolldpos.ProtocolID, rp))
		var indexer blockindex.Indexer
		if _, gateway := cfg.Plugins[config.GatewayPlugin]; gateway && !cfg.Chain.EnableAsyncIndexWrite {
			// create indexer
			cfg.DB.DbPath = cfg.Chain.IndexDBPath
			indexer, err = blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
			require.NoError(err)
		}
		// create BlockDAO
		cfg.DB.DbPath = cfg.Chain.ChainDBPath
		dao := blockdao.NewBlockDAO(db.NewBoltDB(cfg.DB), indexer, cfg.Chain.CompressBlock, cfg.DB)
		require.NotNil(dao)
		bc := NewBlockchain(
			cfg,
			dao,
			PrecreatedStateFactoryOption(sf),
			RegistryOption(&registry),
		)
		bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
		exec := execution.NewProtocol(bc, hu)
		require.NoError(registry.Register(execution.ProtocolID, exec))
		bc.Validator().AddActionValidators(acc, exec)
		sf.AddActionHandlers(exec)
		require.NoError(bc.Start(ctx))
		require.NoError(addCreatorToFactory(sf))

		ms := &MockSubscriber{counter: 0}
		require.NoError(bc.AddSubscriber(ms))
		require.Equal(0, ms.Counter())

		height := bc.TipHeight()
		fmt.Printf("Open blockchain pass, height = %d\n", height)
		require.Nil(addTestingTsfBlocks(bc, dao))
		require.NoError(bc.Stop(ctx))
		require.Equal(24, ms.Counter())

		// Load a blockchain from DB
		accountProtocol := account.NewProtocol(hu)
		registry = protocol.Registry{}
		require.NoError(registry.Register(account.ProtocolID, accountProtocol))
		bc = NewBlockchain(
			cfg,
			dao,
			PrecreatedStateFactoryOption(sf),
			RegistryOption(&registry),
		)
		rolldposProtocol := rolldpos.NewProtocol(
			genesis.Default.NumCandidateDelegates,
			genesis.Default.NumDelegates,
			genesis.Default.NumSubEpochs,
		)
		require.NoError(registry.Register(rolldpos.ProtocolID, rolldposProtocol))
		rewardingProtocol := rewarding.NewProtocol(bc, rolldposProtocol)
		require.NoError(registry.Register(rewarding.ProtocolID, rewardingProtocol))
		bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
		bc.Validator().AddActionValidators(accountProtocol)
		require.NoError(bc.Start(ctx))
		defer func() {
			require.NoError(bc.Stop(ctx))
		}()

		// verify block header hash
		for i := uint64(1); i <= 5; i++ {
			hash, err := bc.GetHashByHeight(i)
			require.NoError(err)
			height, err = bc.GetHeightByHash(hash)
			require.NoError(err)
			require.Equal(i, height)
			header, err := bc.BlockHeaderByHash(hash)
			require.NoError(err)
			require.Equal(hash, header.HashBlock())
			header, err = bc.BlockHeaderByHeight(height)
			require.NoError(err)
			require.Equal(height, header.Height())

			// bloomfilter only exists after aleutian height
			require.Equal(height >= cfg.Genesis.AleutianBlockHeight, header.LogsBloomfilter() != nil)
		}

		empblk, err := bc.GetBlockByHash(hash.ZeroHash256)
		require.Nil(empblk)
		require.NotNil(err.Error())

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

		err = bc.ValidateBlock(&nblk)
		require.Error(err)
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
		blk, err := bc.GetBlockByHeight(3)
		require.NotNil(blk)
		require.NoError(err)
		require.NoError(bc.(*blockchain).commitBlock(blk))
		fmt.Printf("Cannot add block 3 again: %v\n", err)

		// invalid address returns error
		act, err := bc.StateByAddr("")
		require.Equal("invalid bech32 string length 0", errors.Cause(err).Error())
		require.Nil(act)

		// valid but unused address should return empty account
		act, err = bc.StateByAddr("io1066kus4vlyvk0ljql39fzwqw0k22h7j8wmef3n")
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
			blk, err := bc.GetBlockByHeight(3)
			require.NoError(err)
			f := blk.Header.LogsBloomfilter()
			require.NotNil(f)
			require.True(f.Exist(funcSig[:]))
			require.True(f.Exist(setTopic))

			// 3 topics in block 4 calling get()
			funcSig = hash.Hash256b([]byte("Get(address,uint256)"))
			blk, err = bc.GetBlockByHeight(4)
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
				blk, err = bc.GetBlockByHeight(h)
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
	bc := NewBlockchain(cfg, nil, InMemDaoOption(), InMemStateFactoryOption())
	require.NoError(t, bc.Start(ctx))
	defer func() {
		err := bc.Stop(ctx)
		require.Nil(t, err)
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
	accountProtocol := account.NewProtocol(config.NewHeightUpgrade(cfg))
	sf.AddActionHandlers(accountProtocol)
	registry := protocol.Registry{}
	require.NoError(registry.Register(account.ProtocolID, accountProtocol))
	bc := NewBlockchain(
		cfg,
		nil,
		PrecreatedStateFactoryOption(sf),
		BoltDBDaoOption(),
		RegistryOption(&registry),
	)
	rolldposProtocol := rolldpos.NewProtocol(
		genesis.Default.NumCandidateDelegates,
		genesis.Default.NumDelegates,
		genesis.Default.NumSubEpochs,
	)
	require.NoError(registry.Register(rolldpos.ProtocolID, rolldposProtocol))
	rewardingProtocol := rewarding.NewProtocol(bc, rolldposProtocol)
	require.NoError(registry.Register(rewarding.ProtocolID, rewardingProtocol))
	require.NoError(registry.Register(poll.ProtocolID, poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)))
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
	}()
	candidate, err := sf.CandidatesByHeight(1)
	require.NoError(err)
	require.Equal(24, len(candidate))
}

func TestBlockchain_StateByAddr(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	// disable account-based testing
	// create chain

	bc := NewBlockchain(cfg, nil, InMemDaoOption(), InMemStateFactoryOption())
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)
	_, err := bc.CreateState(identityset.Address(0).String(), big.NewInt(100))
	require.NoError(err)
	s, err := bc.StateByAddr(identityset.Address(0).String())
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

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath

	sf, _ := factory.NewFactory(cfg, factory.InMemTrieOption())

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, nil, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
	}()

	require.NoError(addCreatorToFactory(sf))

	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	c := identityset.Address(29).String()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	_, err = accountutil.LoadOrCreateAccount(ws, a, big.NewInt(100000))
	require.NoError(err)
	_, err = accountutil.LoadOrCreateAccount(ws, c, big.NewInt(100000))
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: identityset.Address(27),
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, nil)
	require.NoError(err)
	require.NoError(sf.Commit(ws))

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
		require.Nil(bc.ValidateBlock(blk))
		require.Nil(bc.CommitBlock(blk))
	}
}

func TestActions(t *testing.T) {
	// This test is used for block verify benchmark purpose
	t.Skip()
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

	sf, _ := factory.NewFactory(cfg, factory.InMemTrieOption())

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, nil, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
	}()

	require.NoError(addCreatorToFactory(sf))
	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	c := identityset.Address(29).String()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	_, err = accountutil.LoadOrCreateAccount(ws, a, big.NewInt(100000))
	require.NoError(err)
	_, err = accountutil.LoadOrCreateAccount(ws, c, big.NewInt(100000))
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: identityset.Address(27),
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, nil)
	require.NoError(err)
	require.NoError(sf.Commit(ws))

	val := &validator{sf: sf, validatorAddr: ""}
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(account.NewProtocol(config.NewHeightUpgrade(cfg)))
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
	require.Nil(val.Validate(blk, 0, blk.PrevHash()))
}

func addCreatorToFactory(sf factory.Factory) error {
	ws, err := sf.NewWorkingSet()
	if err != nil {
		return err
	}
	if _, err = accountutil.LoadOrCreateAccount(
		ws,
		identityset.Address(27).String(),
		unit.ConvertIotxToRau(10000000000),
	); err != nil {
		return err
	}
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: identityset.Address(27),
			GasLimit: gasLimit,
		})
	if _, err = ws.RunActions(ctx, 0, nil); err != nil {
		return err
	}
	return sf.Commit(ws)
}
