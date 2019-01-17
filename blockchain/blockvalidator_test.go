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
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/state/factory"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestWrongRootHash(t *testing.T) {
	require := require.New(t)
	val := validator{sf: nil, validatorAddr: ""}

	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["alfa"].Bech32(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash := tsf1.Hash()
	blk, err := block.NewTestingBuilder().
		SetChainID(1).
		SetHeight(1).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf2).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	require.Nil(val.Validate(&blk, 0, blkhash, true))
	blk.Actions[0], blk.Actions[1] = blk.Actions[1], blk.Actions[0]
	require.NotNil(val.Validate(&blk, 0, blkhash, true))
}

func TestSignBlock(t *testing.T) {
	require := require.New(t)
	val := validator{sf: nil, validatorAddr: ""}

	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["alfa"].Bech32(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash := tsf1.Hash()
	blk, err := block.NewTestingBuilder().
		SetChainID(1).
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf2).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	require.Nil(val.Validate(&blk, 2, blkhash, true))
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
	require.NoError(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))

	val := &validator{sf: sf, validatorAddr: ""}
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	val.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc))

	// correct nonce
	cbTsf := action.NewCoinBaseTransfer(1, Gen.BlockReward, ta.Addrinfo["producer"].Bech32())
	bd := action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetDestinationAddress(ta.Addrinfo["producer"].Bech32()).
		SetGasLimit(genesis.ActionGasLimit).
		SetAction(cbTsf).Build()
	cbselp, err := action.Sign(elp, ta.Addrinfo["producer"].Bech32(), ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["alfa"].Bech32(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash := tsf1.Hash()
	blk, err := block.NewTestingBuilder().
		SetChainID(cfg.Chain.ID).
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(cbselp, tsf1).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	require.Nil(val.Validate(&blk, 2, blkhash, true))
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			ProducerAddr:    ta.Addrinfo["producer"].Bech32(),
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 1, []action.SealedEnvelope{tsf1})
	require.NoError(err)
	require.Nil(sf.Commit(ws))

	// low nonce
	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blk, err = block.NewTestingBuilder().
		SetChainID(cfg.Chain.ID).
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(cbselp, tsf1, tsf2).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	err = val.Validate(&blk, 2, blkhash, true)
	require.Equal(action.ErrNonce, errors.Cause(err))

	vote, err := testutil.SignedVote(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["producer"].Bech32(), ta.Keyinfo["producer"].PriKey, 1, uint64(100000), big.NewInt(10))
	require.NoError(err)

	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetChainID(cfg.Chain.ID).
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(cbselp, vote).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)
	err = val.Validate(&blk, 2, blkhash, true)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	// duplicate nonce
	tsf3, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["producer"].PriKey, 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	tsf4, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["producer"].PriKey, 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetChainID(cfg.Chain.ID).
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(cbselp, tsf3, tsf4).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)
	err = val.Validate(&blk, 2, blkhash, true)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	vote2, err := testutil.SignedVote(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["producer"].Bech32(), ta.Keyinfo["producer"].PriKey, 2, uint64(100000), big.NewInt(10))
	require.NoError(err)

	vote3, err := testutil.SignedVote(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["producer"].Bech32(), ta.Keyinfo["producer"].PriKey, 2, uint64(100000), big.NewInt(10))
	require.NoError(err)
	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetChainID(cfg.Chain.ID).
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(cbselp, vote2, vote3).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)
	err = val.Validate(&blk, 2, blkhash, true)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	// non consecutive nonce
	tsf5, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["producer"].PriKey, 2, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	tsf6, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["bravo"].Bech32(), ta.Keyinfo["producer"].PriKey, 4, big.NewInt(30), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)
	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetChainID(cfg.Chain.ID).
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(cbselp, tsf5, tsf6).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)
	err = val.Validate(&blk, 2, blkhash, true)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))

	vote4, err := testutil.SignedVote(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["producer"].Bech32(), ta.Keyinfo["producer"].PriKey, 2, uint64(100000), big.NewInt(10))
	require.NoError(err)
	vote5, err := testutil.SignedVote(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["producer"].Bech32(), ta.Keyinfo["producer"].PriKey, 4, uint64(100000), big.NewInt(10))
	require.NoError(err)

	blkhash = tsf1.Hash()
	blk, err = block.NewTestingBuilder().
		SetChainID(cfg.Chain.ID).
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(cbselp, vote4, vote5).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)
	err = val.Validate(&blk, 2, blkhash, true)
	require.Error(err)
	require.Equal(action.ErrNonce, errors.Cause(err))
}

func TestWrongCoinbaseTsf(t *testing.T) {
	cfg := config.Default
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	require := require.New(t)
	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	require.NoError(addCreatorToFactory(sf))

	// Create a blockchain from scratch
	bc := NewBlockchain(cfg, PrecreatedStateFactoryOption(sf), BoltDBDaoOption())
	require.NoError(bc.Start(context.Background()))

	val := &validator{sf: sf, validatorAddr: ""}
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	val.AddActionValidators(account.NewProtocol())

	// no coinbase tsf
	coinbaseTsf := action.NewCoinBaseTransfer(1, Gen.BlockReward, ta.Addrinfo["producer"].Bech32())
	bd := action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetDestinationAddress(ta.Addrinfo["producer"].Bech32()).
		SetGasLimit(genesis.ActionGasLimit).
		SetAction(coinbaseTsf).Build()
	cb, err := action.Sign(elp, ta.Addrinfo["producer"].Bech32(), ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["producer"].Bech32(), ta.Addrinfo["alfa"].Bech32(), ta.Keyinfo["producer"].PriKey, 1, big.NewInt(20), []byte{}, 100000, big.NewInt(10))
	require.NoError(err)

	blkhash := tsf1.Hash()
	blk, err := block.NewTestingBuilder().
		SetChainID(1).
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	err = val.Validate(&blk, 2, blkhash, true)
	require.Error(err)
	require.True(
		strings.Contains(err.Error(), "wrong number of coinbase transfers"),
	)

	// extra coinbase transfer
	blk, err = block.NewTestingBuilder().
		SetChainID(1).
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(cb, cb, tsf1).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)
	err = val.Validate(&blk, 2, blkhash, true)
	require.Error(err)
	require.True(
		strings.Contains(err.Error(), "wrong number of coinbase transfers in block"),
	)

	// no transfer
	blk, err = block.NewTestingBuilder().
		SetChainID(1).
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)
	err = val.Validate(&blk, 2, blkhash, true)
	require.Error(err)
	require.True(
		strings.Contains(err.Error(), "wrong number of coinbase transfers"),
	)
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
	val.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	val.AddActionValidators(account.NewProtocol(), vote.NewProtocol(bc),
		execution.NewProtocol(bc))

	invalidRecipient := "io1qyqsyqcyq5narhapakcsrhksfajfcpl24us3xp38zwvsep"
	tsf, err := action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["producer"].Bech32(), invalidRecipient, []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf).SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).
		SetNonce(1).SetDestinationAddress(invalidRecipient).Build()
	selp, err := action.Sign(elp, ta.Addrinfo["producer"].Bech32(), ta.Keyinfo["producer"].PriKey)
	require.NoError(t, err)
	blk1, err := block.NewTestingBuilder().
		SetChainID(1).
		SetHeight(3).
		SetPrevBlockHash(hash.ZeroHash32B).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(t, err)
	err = val.ValidateActionsOnly(
		blk1.Actions,
		false,
		blk1.SecretWitness,
		blk1.SecretProposals,
		blk1.PublicKey(),
		blk1.ChainID(),
		blk1.Height(),
	)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "error when validating recipient's address"))

	invalidVotee := "ioaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	vote, err := action.NewVote(1, ta.Addrinfo["producer"].Bech32(), invalidVotee, uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(vote).SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).
		SetNonce(1).SetDestinationAddress(invalidVotee).Build()
	selp, err = action.Sign(elp, ta.Addrinfo["producer"].Bech32(), ta.Keyinfo["producer"].PriKey)
	require.NoError(t, err)
	blk2, err := block.NewTestingBuilder().
		SetChainID(1).
		SetHeight(3).
		SetPrevBlockHash(hash.ZeroHash32B).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(t, err)

	err = val.ValidateActionsOnly(
		blk2.Actions,
		false,
		blk2.SecretWitness,
		blk2.SecretProposals,
		blk2.PublicKey(),
		blk2.ChainID(),
		blk2.Height(),
	)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "error when validating votee's address"))

	invalidContract := "123"
	execution, err := action.NewExecution(ta.Addrinfo["producer"].Bech32(), invalidContract, 1, big.NewInt(1), uint64(100000), big.NewInt(10), []byte{})
	require.NoError(t, err)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(execution).SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).
		SetNonce(1).SetDestinationAddress(invalidContract).Build()
	selp, err = action.Sign(elp, ta.Addrinfo["producer"].Bech32(), ta.Keyinfo["producer"].PriKey)
	require.NoError(t, err)
	blk3, err := block.NewTestingBuilder().
		SetChainID(1).
		SetHeight(3).
		SetPrevBlockHash(hash.ZeroHash32B).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(selp).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(t, err)
	err = val.ValidateActionsOnly(
		blk3.Actions,
		false,
		blk3.SecretWitness,
		blk3.SecretProposals,
		blk3.PublicKey(),
		blk3.ChainID(),
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

	pk, err := keypair.DecodePublicKey(
		"1d1727028b1e9dac0cafa693edd8496297f5c3281924ec578c0526e7340f7180bfa5af059084c8b90954bf2802a0060e145bece9580f9021352eb112340186e68dc9bea4f7711707")
	require.NoError(t, err)
	sk, err := keypair.DecodePrivateKey(
		"29cf385adfc5b1a84bd7e778ea2c056b85c977771005d545e54100266e224fc276ed7101")
	require.NoError(t, err)
	pkHash := keypair.HashPubKey(pk)
	addr := address.New(cfg.Chain.ID, pkHash[:])
	blk, err := chain.MintNewBlock(nil, pk, sk, addr.Bech32(),
		nil, nil, "")
	require.NoError(t, err)
	validator := validator{}
	require.NoError(t, validator.ValidateActionsOnly(
		blk.Actions,
		true,
		blk.SecretWitness,
		blk.SecretProposals,
		blk.PublicKey(),
		blk.ChainID(),
		blk.Height(),
	))
}

func TestValidateSecretBlock(t *testing.T) {
	cfg := config.Default
	testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.TrieDBPath)
	testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	defer testutil.CleanupPath(t, cfg.Chain.ChainDBPath)
	require := require.New(t)
	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(context.Background()))
	require.Nil(addCreatorToFactory(sf))

	idList := make([][]uint8, 0)
	delegates := []string{ta.Addrinfo["producer"].Bech32()}
	for i := 0; i < 20; i++ {
		pk, _, err := crypto.EC283.NewKeyPair()
		require.NoError(err)
		pkHash := keypair.HashPubKey(pk)
		addr := address.New(cfg.Chain.ID, pkHash[:])
		delegates = append(delegates, addr.Bech32())
	}

	for _, delegate := range delegates {
		idList = append(idList, address.Bech32ToID(delegate))
	}
	producerSK := crypto.DKG.SkGeneration()
	_, shares, witness, err := crypto.DKG.Init(producerSK, idList)
	require.NoError(err)

	secretProposals := make([]*action.SecretProposal, 0)
	for i, share := range shares {
		secretProposal, err := action.NewSecretProposal(uint64(i+1), delegates[0], delegates[i], share)
		require.NoError(err)
		secretProposals = append(secretProposals, secretProposal)
	}
	secretWitness, err := action.NewSecretWitness(uint64(22), delegates[0], witness)
	require.NoError(err)
	blkhash := secretProposals[0].Hash()
	blk, err := block.NewTestingBuilder().
		SetChainID(1).
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		SetSecretProposals(secretProposals).
		SetSecretWitness(secretWitness).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)

	val := &validator{sf: sf, validatorAddr: delegates[1]}
	require.NoError(val.Validate(&blk, 2, blkhash, false))

	// Falsify secret proposal
	dummySecretProposal, err := action.NewSecretProposal(2, delegates[0], delegates[1], []uint32{1, 2, 3, 4, 5})
	require.NoError(err)
	secretProposals[1] = dummySecretProposal
	blk, err = block.NewTestingBuilder().
		SetChainID(1).
		SetHeight(3).
		SetPrevBlockHash(blkhash).
		SetTimeStamp(testutil.TimestampNow()).
		SetSecretProposals(secretProposals).
		SetSecretWitness(secretWitness).
		SignAndBuild(ta.Keyinfo["producer"].PubKey, ta.Keyinfo["producer"].PriKey)
	require.NoError(err)
	err = val.Validate(&blk, 2, blkhash, false)
	require.Error(err)
	require.Equal(ErrDKGSecretProposal, errors.Cause(err))
}
