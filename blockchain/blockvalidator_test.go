// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestWrongRootHash(t *testing.T) {
	require := require.New(t)
	val := validator{sf: nil, validatorAddr: ""}

	tsf1, err := testutil.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf2, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash := tsf1.Hash()
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf2).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	ctx := protocol.WithBlockchainCtx(
		context.Background(),
		protocol.BlockchainCtx{
			Genesis: config.Default.Genesis,
			Tip: protocol.TipInfo{
				Height: 0,
				Hash:   blkhash,
			},
		},
	)

	require.NoError(val.Validate(ctx, &blk))
	blk.Actions[0], blk.Actions[1] = blk.Actions[1], blk.Actions[0]
	require.Error(val.Validate(ctx, &blk))
}

func TestSignBlock(t *testing.T) {
	require := require.New(t)
	val := validator{sf: nil, validatorAddr: ""}

	tsf1, err := testutil.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf2, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash := tsf1.Hash()
	blk, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf2).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	ctx := protocol.WithBlockchainCtx(
		context.Background(),
		protocol.BlockchainCtx{
			Genesis: config.Default.Genesis,
			Tip: protocol.TipInfo{
				Height: 2,
				Hash:   blkhash,
			},
		},
	)

	require.NoError(val.Validate(ctx, &blk))
}

func TestWrongNonce(t *testing.T) {
	cfg := config.Default

	require := require.New(t)
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	cfg.Chain.TrieDBPath = testTriePath
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()
	cfg.Chain.ChainDBPath = testDBPath
	testIndexFile, _ := ioutil.TempFile(os.TempDir(), "index")
	testIndexPath := testIndexFile.Name()
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()

	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, nil, sf, BoltDBDaoOption(), RegistryOption(registry))
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
	}()

	val := &validator{sf: sf, validatorAddr: ""}
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf.AccountState))

	// correct nonce

	tsf1, err := testutil.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	accMap := make(map[string][]action.SealedEnvelope)
	accMap[identityset.Address(27).String()] = []action.SealedEnvelope{tsf1}

	blk, err := bc.MintNewBlock(
		accMap,
		testutil.TimestampNow(),
	)
	require.NoError(err)
	require.NotNil(blk)
	ctx := protocol.WithBlockchainCtx(
		context.Background(),
		protocol.BlockchainCtx{
			Genesis: cfg.Genesis,
			Tip: protocol.TipInfo{
				Height: blk.Height() - 1,
				Hash:   cfg.Genesis.Hash(),
			},
			Registry: registry,
		},
	)

	require.NoError(val.Validate(ctx, blk))
	require.NoError(bc.CommitBlock(blk))

	// low nonce
	tsf2, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	accMap2 := make(map[string][]action.SealedEnvelope)
	accMap2[identityset.Address(27).String()] = []action.SealedEnvelope{tsf2}

	prevHash := bc.TipHash()
	blk2, err := bc.MintNewBlock(
		accMap2,
		testutil.TimestampNow(),
	)
	require.NoError(err)
	require.NotNil(blk)

	ctx = protocol.WithBlockchainCtx(
		ctx,
		protocol.BlockchainCtx{
			Genesis: cfg.Genesis,
			Tip: protocol.TipInfo{
				Height: blk2.Height() - 1,
				Hash:   prevHash,
			},
			Registry: registry,
		},
	)
	err = val.Validate(ctx, blk2)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	// duplicate nonce
	tsf3, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf4, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	accMap3 := make(map[string][]action.SealedEnvelope)
	accMap3[identityset.Address(27).String()] = []action.SealedEnvelope{tsf3, tsf4}

	blk3, err := bc.MintNewBlock(
		accMap3,
		testutil.TimestampNow(),
	)
	require.NoError(err)
	require.NotNil(blk3)

	ctx = protocol.WithBlockchainCtx(
		ctx,
		protocol.BlockchainCtx{
			Genesis: cfg.Genesis,
			Tip: protocol.TipInfo{
				Height: blk3.Height() - 1,
				Hash:   prevHash,
			},
			Registry: registry,
		},
	)

	err = val.Validate(ctx, blk3)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	// non consecutive nonce
	tsf5, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	tsf6, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 4, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	accMap4 := make(map[string][]action.SealedEnvelope)
	accMap4[identityset.Address(27).String()] = []action.SealedEnvelope{tsf5, tsf6}

	blk4, err := bc.MintNewBlock(
		accMap4,
		testutil.TimestampNow(),
	)
	require.NoError(err)
	require.NotNil(blk4)

	ctx = protocol.WithBlockchainCtx(
		ctx,
		protocol.BlockchainCtx{
			Genesis: cfg.Genesis,
			Tip: protocol.TipInfo{
				Height: blk4.Height() - 1,
				Hash:   prevHash,
			},
			Registry: registry,
		},
	)

	err = val.Validate(ctx, blk4)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))
}

func TestWrongAddress(t *testing.T) {
	cfg := config.Default

	ctx := context.Background()
	registry := protocol.NewRegistry()
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(t, err)
	dao := blockdao.NewBlockDAO(db.NewMemKVStore(), nil, cfg.Chain.CompressBlock, cfg.DB)
	bc := NewBlockchain(cfg, dao, sf, RegistryOption(registry))
	require.NoError(t, bc.Start(ctx))
	require.NotNil(t, bc)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(t, err)
	}()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	ep := execution.NewProtocol(dao.GetBlockHash)
	require.NoError(t, ep.Register(registry))

	ctx = protocol.WithBlockchainCtx(
		ctx,
		protocol.BlockchainCtx{Genesis: cfg.Genesis, Registry: registry},
	)

	val := &validator{sf: sf, validatorAddr: ""}
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf.AccountState))

	invalidRecipient := "io1qyqsyqcyq5narhapakcsrhksfajfcpl24us3xp38zwvsep"
	tsf, err := action.NewTransfer(1, big.NewInt(1), invalidRecipient, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf).SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).
		SetNonce(1).Build()
	selp, err := action.Sign(elp, identityset.PrivateKey(27))
	require.NoError(t, err)
	blk1, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)
	err = val.validateActionsOnly(ctx, &blk1)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "error when validating recipient's address"))

	invalidContract := "123"
	execution, err := action.NewExecution(invalidContract, 1, big.NewInt(1), uint64(100000), big.NewInt(10), []byte{})
	require.NoError(t, err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(execution).SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).
		SetNonce(1).Build()
	selp, err = action.Sign(elp, identityset.PrivateKey(27))
	require.NoError(t, err)
	blk3, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)
	err = val.validateActionsOnly(ctx, &blk3)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "error when validating contract's address"))
}

func TestBlackListAddress(t *testing.T) {
	cfg := config.Default

	ctx := context.Background()
	recipientAddr := identityset.Address(28)
	senderKey := identityset.PrivateKey(27)
	addr, err := address.FromBytes(senderKey.PublicKey().Hash())
	require.NoError(t, err)
	cfg.ActPool.BlackList = []string{addr.String()}
	registry := protocol.NewRegistry()
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(t, err)
	dao := blockdao.NewBlockDAO(db.NewMemKVStore(), nil, cfg.Chain.CompressBlock, cfg.DB)
	bc := NewBlockchain(cfg, dao, sf, RegistryOption(registry))
	require.NoError(t, bc.Start(ctx))
	require.NotNil(t, bc)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(t, err)
	}()

	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	ep := execution.NewProtocol(dao.GetBlockHash)
	require.NoError(t, ep.Register(registry))

	ctx = protocol.WithBlockchainCtx(
		ctx,
		protocol.BlockchainCtx{Genesis: cfg.Genesis, Registry: registry},
	)

	senderBlackList := make(map[string]bool)
	for _, bannedSender := range cfg.ActPool.BlackList {
		senderBlackList[bannedSender] = true
	}
	val := &validator{sf: sf, validatorAddr: "", senderBlackList: senderBlackList}
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf.AccountState))
	tsf, err := action.NewTransfer(1, big.NewInt(1), recipientAddr.String(), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf).SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).
		SetNonce(1).Build()
	selp, err := action.Sign(elp, senderKey)
	require.NoError(t, err)
	blk1, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).
		SignAndBuild(senderKey)
	require.NoError(t, err)
	err = val.validateActionsOnly(ctx, &blk1)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "action source address is blacklisted"))
}
