// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state/factory"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestWrongRootHash(t *testing.T) {
	require := require.New(t)
	val := validator{sf: nil, validatorAddr: ""}

	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["alfa"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash := tsf1.Hash()
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf2).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	require.Nil(val.Validate(&blk, 0, blkhash))
	blk.Actions[0], blk.Actions[1] = blk.Actions[1], blk.Actions[0]
	require.NotNil(val.Validate(&blk, 0, blkhash))
}

func TestSignBlock(t *testing.T) {
	require := require.New(t)
	val := validator{sf: nil, validatorAddr: ""}

	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["alfa"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash := tsf1.Hash()
	blk, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf2).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	require.Nil(val.Validate(&blk, 2, blkhash))
}

func TestWrongNonce(t *testing.T) {
	cfg := config.Default
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	require := require.New(t)
	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	sf.AddActionHandlers(account.NewProtocol(), vote.NewProtocol(nil))

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
	}()

	require.NoError(addCreatorToFactory(sf))

	val := &validator{sf: sf, validatorAddr: ""}
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	val.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))

	// correct nonce

	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["alfa"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash := tsf1.Hash()
	blk, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	require.Nil(val.Validate(&blk, 2, blkhash))
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: ta.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 1, []action.SealedEnvelope{tsf1})
	require.NoError(err)
	require.Nil(sf.Commit(ws))

	// low nonce
	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blk, err = block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf2).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	err = val.Validate(&blk, 2, blkhash)
	require.Equal(action.ErrNonce, errors.Cause(err))

	vote, err := testutil.SignedVote(ta.Addrinfo["producer"].String(), ta.Keyinfo["producer"].PriKey, 1, uint64(100000), big.NewInt(10))
	require.NoError(err)

	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(vote).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)
	err = val.Validate(&blk, 2, blkhash)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	// duplicate nonce
	tsf3, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf4, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf3, tsf4).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)
	err = val.Validate(&blk, 2, blkhash)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	vote2, err := testutil.SignedVote(ta.Addrinfo["producer"].String(), ta.Keyinfo["producer"].PriKey, 2, uint64(100000), big.NewInt(10))
	require.NoError(err)

	vote3, err := testutil.SignedVote(ta.Addrinfo["producer"].String(), ta.Keyinfo["producer"].PriKey, 2, uint64(100000), big.NewInt(10))
	require.NoError(err)
	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(vote2, vote3).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)
	err = val.Validate(&blk, 2, blkhash)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	// non consecutive nonce
	tsf5, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	tsf6, err := testutil.SignedTransfer(ta.Addrinfo["bravo"].String(), ta.Keyinfo["producer"].PriKey, 4, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf5, tsf6).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)
	err = val.Validate(&blk, 2, blkhash)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	vote4, err := testutil.SignedVote(ta.Addrinfo["producer"].String(), ta.Keyinfo["producer"].PriKey, 2, uint64(100000), big.NewInt(10))
	require.NoError(err)
	vote5, err := testutil.SignedVote(ta.Addrinfo["producer"].String(), ta.Keyinfo["producer"].PriKey, 4, uint64(100000), big.NewInt(10))
	require.NoError(err)

	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(vote4, vote5).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)
	err = val.Validate(&blk, 2, blkhash)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))
}

func TestWrongAddress(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	bc := NewBlockchain(cfg, InMemStateFactoryOption(), InMemDaoOption())
	bc.GetFactory().AddActionHandlers(account.NewProtocol(), vote.NewProtocol(bc))
	require.NoError(t, bc.Start(ctx))
	require.NotNil(t, bc)
	defer func() {
		err := bc.Stop(ctx)
		require.NoError(t, err)
	}()
	val := &validator{sf: bc.GetFactory(), validatorAddr: ""}
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	val.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc),
		execution.NewProtocol(bc))

	invalidRecipient := "io1qyqsyqcyq5narhapakcsrhksfajfcpl24us3xp38zwvsep"
	tsf, err := action.NewTransfer(1, big.NewInt(1), invalidRecipient, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf).SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).
		SetNonce(1).Build()
	selp, err := action.Sign(elp, ta.Keyinfo["producer"].PriKey)
	require.NoError(t, err)
	blk1, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(t, err)
	err = val.validateActionsOnly(
		blk1.Actions,
		blk1.PublicKey(),
		blk1.Height(),
	)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "error when validating recipient's address"))

	invalidVotee := "ioaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	vote, err := action.NewVote(1, invalidVotee, uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote).SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).
		SetNonce(1).Build()
	selp, err = action.Sign(elp, ta.Keyinfo["producer"].PriKey)
	require.NoError(t, err)
	blk2, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(t, err)

	err = val.validateActionsOnly(
		blk2.Actions,
		blk2.PublicKey(),
		blk2.Height(),
	)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "error when validating votee's address"))

	invalidContract := "123"
	execution, err := action.NewExecution(invalidContract, 1, big.NewInt(1), uint64(100000), big.NewInt(10), []byte{})
	require.NoError(t, err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(execution).SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).
		SetNonce(1).Build()
	selp, err = action.Sign(elp, ta.Keyinfo["producer"].PriKey)
	require.NoError(t, err)
	blk3, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(t, err)
	err = val.validateActionsOnly(
		blk3.Actions,
		blk3.PublicKey(),
		blk3.Height(),
	)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "error when validating contract's address"))
}

func TestCoinbaseTransferValidation(t *testing.T) {
	t.Skip("It is skipped because testnet_actions.yaml doesn't match the chain ID")
	ctx := context.Background()
	cfg := config.Default
	cfg.Chain.ID = 1
	chain := NewBlockchain(cfg, InMemStateFactoryOption(), InMemDaoOption())
	require.NotNil(t, chain)
	require.NoError(t, chain.Start(ctx))
	defer require.NoError(t, chain.Stop(ctx))

	blk, err := chain.MintNewBlock(nil, testutil.TimestampNow())
	require.NoError(t, err)
	validator := validator{}
	require.NoError(t, validator.validateActionsOnly(
		blk.Actions,
		blk.PublicKey(),
		blk.Height(),
	))
}
