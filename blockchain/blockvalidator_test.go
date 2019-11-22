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
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
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
	ctx := protocol.WithValidateActionsCtx(
		context.Background(),
		protocol.ValidateActionsCtx{
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
	ctx := protocol.WithValidateActionsCtx(
		context.Background(),
		protocol.ValidateActionsCtx{
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
	registry := protocol.Registry{}
	require.NoError(registry.Register(account.ProtocolID, account.NewProtocol()))

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	cfg.Chain.TrieDBPath = testTriePath
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()
	cfg.Chain.ChainDBPath = testDBPath
	testIndexFile, _ := ioutil.TempFile(os.TempDir(), "index")
	testIndexPath := testIndexFile.Name()
	cfg.Chain.IndexDBPath = testIndexPath

	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, nil, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
	}()

	require.NoError(addCreatorToFactory(cfg, sf, &registry))

	val := &validator{sf: sf, validatorAddr: ""}
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc.Factory().Nonce))

	// correct nonce

	tsf1, err := testutil.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash := tsf1.Hash()
	blk, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	ctx := protocol.WithValidateActionsCtx(
		context.Background(),
		protocol.ValidateActionsCtx{
			Genesis: cfg.Genesis,
			Tip: protocol.TipInfo{
				Height: 2,
				Hash:   blkhash,
			},
			Registry: &registry,
		},
	)
	require.NoError(val.Validate(ctx, &blk))
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx = protocol.WithRunActionsCtx(
		ctx,
		protocol.RunActionsCtx{
			Producer: identityset.Address(27),
			GasLimit: gasLimit,
			Genesis:  config.Default.Genesis,
			Registry: &registry,
		},
	)
	_, err = ws.RunActions(ctx, 1, []action.SealedEnvelope{tsf1})
	require.NoError(err)
	require.Nil(sf.Commit(ws))

	// low nonce
	tsf2, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blk, err = block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf2).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	ctx = protocol.WithValidateActionsCtx(
		ctx,
		protocol.ValidateActionsCtx{
			Genesis: cfg.Genesis,
			Tip: protocol.TipInfo{
				Height: 2,
				Hash:   blkhash,
			},
			Registry: &registry,
		},
	)
	err = val.Validate(ctx, &blk)
	require.Equal(action.ErrNonce, errors.Cause(err))

	tsf3, err := testutil.SignedTransfer(identityset.Address(27).String(), identityset.PrivateKey(27), 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf3).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	ctx = protocol.WithValidateActionsCtx(
		ctx,
		protocol.ValidateActionsCtx{
			Genesis: cfg.Genesis,
			Tip: protocol.TipInfo{
				Height: 2,
				Hash:   blkhash,
			},
			Registry: &registry,
		},
	)
	err = val.Validate(ctx, &blk)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	// duplicate nonce
	tsf4, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf5, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf4, tsf5).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	ctx = protocol.WithValidateActionsCtx(
		ctx,
		protocol.ValidateActionsCtx{
			Genesis: cfg.Genesis,
			Tip: protocol.TipInfo{
				Height: 2,
				Hash:   blkhash,
			},
			Registry: &registry,
		},
	)
	err = val.Validate(ctx, &blk)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	tsf6, err := testutil.SignedTransfer(identityset.Address(27).String(), identityset.PrivateKey(27), 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf7, err := testutil.SignedTransfer(identityset.Address(27).String(), identityset.PrivateKey(27), 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf6, tsf7).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	ctx = protocol.WithValidateActionsCtx(
		ctx,
		protocol.ValidateActionsCtx{
			Genesis: cfg.Genesis,
			Tip: protocol.TipInfo{
				Height: 2,
				Hash:   blkhash,
			},
			Registry: &registry,
		},
	)
	err = val.Validate(ctx, &blk)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	// non consecutive nonce
	tsf8, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	tsf9, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 4, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf8, tsf9).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	ctx = protocol.WithValidateActionsCtx(
		ctx,
		protocol.ValidateActionsCtx{
			Genesis: cfg.Genesis,
			Tip: protocol.TipInfo{
				Height: 2,
				Hash:   blkhash,
			},
			Registry: &registry,
		},
	)
	err = val.Validate(ctx, &blk)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	tsf10, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	tsf11, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 4, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf10, tsf11).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(err)
	ctx = protocol.WithValidateActionsCtx(
		ctx,
		protocol.ValidateActionsCtx{
			Genesis: cfg.Genesis,
			Tip: protocol.TipInfo{
				Height: 2,
				Hash:   blkhash,
			},
			Registry: &registry,
		},
	)
	err = val.Validate(ctx, &blk)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))
}

func TestWrongAddress(t *testing.T) {
	cfg := config.Default

	ctx := context.Background()
	bc := NewBlockchain(cfg, nil, InMemStateFactoryOption(), InMemDaoOption())
	require.NoError(t, bc.Start(ctx))
	require.NotNil(t, bc)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(t, err)
	}()
	registry := protocol.Registry{}
	require.NoError(t, registry.Register(account.ProtocolID, account.NewProtocol()))
	require.NoError(t, registry.Register(execution.ProtocolID, execution.NewProtocol(bc.BlockDAO().GetBlockHash)))

	ctx = protocol.WithValidateActionsCtx(
		ctx,
		protocol.ValidateActionsCtx{Genesis: cfg.Genesis, Registry: &registry},
	)

	val := &validator{sf: bc.Factory(), validatorAddr: ""}
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc.Factory().Nonce))

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
	bc := NewBlockchain(cfg, nil, InMemStateFactoryOption(), InMemDaoOption())
	require.NoError(t, bc.Start(ctx))
	require.NotNil(t, bc)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(t, err)
	}()

	registry := protocol.Registry{}
	require.NoError(t, registry.Register(account.ProtocolID, account.NewProtocol()))
	require.NoError(t, registry.Register(execution.ProtocolID, execution.NewProtocol(bc.BlockDAO().GetBlockHash)))

	ctx = protocol.WithValidateActionsCtx(
		ctx,
		protocol.ValidateActionsCtx{Genesis: cfg.Genesis, Registry: &registry},
	)

	senderBlackList := make(map[string]bool)
	for _, bannedSender := range cfg.ActPool.BlackList {
		senderBlackList[bannedSender] = true
	}
	val := &validator{sf: bc.Factory(), validatorAddr: "", senderBlackList: senderBlackList}
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc.Factory().Nonce))
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
