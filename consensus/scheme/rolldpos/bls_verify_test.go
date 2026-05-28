// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"math"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/v2/endorsement"
)

// blsTestKeys derives a deterministic (ECDSA, BLS) key pair from a seed byte.
func blsTestKeys(t *testing.T, seed byte) (crypto.PrivateKey, *crypto.BLS12381PrivateKey) {
	t.Helper()
	ikm := make([]byte, 32)
	for i := range ikm {
		ikm[i] = seed
	}
	ecdsa, err := crypto.BytesToPrivateKey(ikm)
	require.NoError(t, err)
	bls, err := crypto.GenerateBLS12381PrivateKey(ikm)
	require.NoError(t, err)
	return ecdsa, bls
}

// roundCtxWithBLS hand-builds a minimal roundCtx for unit tests of the BLS
// verify dispatch — just enough state for AddVoteEndorsement / verifyEndorsement
// to find the delegate set and the BLS pubkey index.
func roundCtxWithBLS(delegateAddr string, blsPubKey *crypto.BLS12381PublicKey) *roundCtx {
	r := &roundCtx{
		delegates:      []string{delegateAddr},
		numOfDelegates: 1,
	}
	if blsPubKey != nil {
		r.blsPubKeys = map[string]*crypto.BLS12381PublicKey{delegateAddr: blsPubKey}
	}
	return r
}

func TestRoundCtx_VerifyEndorsement_ECDSA(t *testing.T) {
	require := require.New(t)
	ecdsa, _ := blsTestKeys(t, 0x01)
	addr := ecdsa.PublicKey().Address().String()
	ctx := roundCtxWithBLS(addr, nil)

	blkHash := hash.Hash256b([]byte("block-A"))
	vote := NewConsensusVote(blkHash[:], COMMIT)
	ens, err := endorsement.Endorse(vote, time.Unix(1700000000, 0), ecdsa)
	require.NoError(err)
	require.Equal(1, len(ens))
	require.Equal(crypto.Secp256k1SigSizeWithRecID, len(ens[0].Signature()),
		"secp256k1 signature should be 65 bytes")

	require.NoError(ctx.verifyEndorsement(vote, ens[0]),
		"a valid secp256k1 endorsement on the matching vote must verify")
}

func TestRoundCtx_VerifyEndorsement_BLS(t *testing.T) {
	require := require.New(t)
	ecdsa, bls := blsTestKeys(t, 0x02)
	addr := ecdsa.PublicKey().Address().String()
	ctx := roundCtxWithBLS(addr, bls.PublicKey())

	blkHash := hash.Hash256b([]byte("block-B"))
	vote := NewConsensusVote(blkHash[:], COMMIT)
	en, err := endorsement.EndorseBLS(vote, time.Unix(1700000000, 0), ecdsa.PublicKey(), bls)
	require.NoError(err)
	require.Equal(crypto.BLSAggregateSignatureLength, len(en.Signature()),
		"BLS signature should be 96 bytes")

	require.NoError(ctx.verifyEndorsement(vote, en),
		"a valid BLS endorsement with the delegate's registered pubkey must verify")
}

func TestRoundCtx_VerifyEndorsement_BLS_MissingPubKey(t *testing.T) {
	require := require.New(t)
	ecdsa, bls := blsTestKeys(t, 0x03)
	addr := ecdsa.PublicKey().Address().String()
	// the round has NO BLS pubkey for this delegate
	ctx := roundCtxWithBLS(addr, nil)

	blkHash := hash.Hash256b([]byte("block-C"))
	vote := NewConsensusVote(blkHash[:], COMMIT)
	en, err := endorsement.EndorseBLS(vote, time.Unix(1700000000, 0), ecdsa.PublicKey(), bls)
	require.NoError(err)

	err = ctx.verifyEndorsement(vote, en)
	require.Error(err)
	require.Contains(err.Error(), "no registered BLS pubkey")
}

func TestRoundCtx_VerifyEndorsement_BLS_WrongPubKey(t *testing.T) {
	require := require.New(t)
	ecdsa, signerBLS := blsTestKeys(t, 0x04)
	_, otherBLS := blsTestKeys(t, 0x05)
	addr := ecdsa.PublicKey().Address().String()
	// the round records a *different* BLS pubkey for this delegate
	ctx := roundCtxWithBLS(addr, otherBLS.PublicKey())

	blkHash := hash.Hash256b([]byte("block-D"))
	vote := NewConsensusVote(blkHash[:], COMMIT)
	en, err := endorsement.EndorseBLS(vote, time.Unix(1700000000, 0), ecdsa.PublicKey(), signerBLS)
	require.NoError(err)

	err = ctx.verifyEndorsement(vote, en)
	require.Error(err)
	require.Contains(err.Error(), "invalid BLS endorsement signature")
}

// rollDPoSCtxWithBLSGate hand-builds a minimal rollDPoSCtx for unit tests
// of the public VerifyEndorsement entry point. blsAggHeight controls the
// feature gate: 0 means BLS aggregation is on for all heights, MaxUint64
// means it is off.
func rollDPoSCtxWithBLSGate(t *testing.T, round *roundCtx, blsAggHeight uint64) *rollDPoSCtx {
	t.Helper()
	g := genesis.TestDefault()
	g.ToBeEnabledBlockHeight = blsAggHeight
	cfg := consensusfsm.NewConsensusConfig(
		consensusfsm.ConsensusTiming{},
		consensusfsm.DefaultDardanellesUpgradeConfig,
		consensusfsm.DefaultWakeUpgradeConfig,
		g,
		0,
	)
	return &rollDPoSCtx{
		ConsensusConfig: cfg,
		round:           round,
	}
}

func TestRollDPoSCtx_VerifyEndorsement_PreForkRejectsBLS(t *testing.T) {
	require := require.New(t)
	ecdsa, bls := blsTestKeys(t, 0x10)
	addr := ecdsa.PublicKey().Address().String()
	round := roundCtxWithBLS(addr, bls.PublicKey())
	ctx := rollDPoSCtxWithBLSGate(t, round, math.MaxUint64) // BLS off

	blkHash := hash.Hash256b([]byte("blk"))
	vote := NewConsensusVote(blkHash[:], COMMIT)
	en, err := endorsement.EndorseBLS(vote, time.Unix(1700000000, 0), ecdsa.PublicKey(), bls)
	require.NoError(err)

	err = ctx.VerifyEndorsement(1, vote, en)
	require.Error(err)
	require.Contains(err.Error(), "expected secp256k1")
}

func TestRollDPoSCtx_VerifyEndorsement_PostForkRejectsECDSA(t *testing.T) {
	require := require.New(t)
	ecdsa, bls := blsTestKeys(t, 0x11)
	addr := ecdsa.PublicKey().Address().String()
	round := roundCtxWithBLS(addr, bls.PublicKey())
	ctx := rollDPoSCtxWithBLSGate(t, round, 0) // BLS on

	blkHash := hash.Hash256b([]byte("blk"))
	vote := NewConsensusVote(blkHash[:], COMMIT)
	ens, err := endorsement.Endorse(vote, time.Unix(1700000000, 0), ecdsa)
	require.NoError(err)

	err = ctx.VerifyEndorsement(100, vote, ens[0])
	require.Error(err)
	require.Contains(err.Error(), "expected BLS")
}

func TestRollDPoSCtx_VerifyEndorsement_PostForkAcceptsBLS(t *testing.T) {
	require := require.New(t)
	ecdsa, bls := blsTestKeys(t, 0x12)
	addr := ecdsa.PublicKey().Address().String()
	round := roundCtxWithBLS(addr, bls.PublicKey())
	ctx := rollDPoSCtxWithBLSGate(t, round, 0) // BLS on

	blkHash := hash.Hash256b([]byte("blk"))
	vote := NewConsensusVote(blkHash[:], COMMIT)
	en, err := endorsement.EndorseBLS(vote, time.Unix(1700000000, 0), ecdsa.PublicKey(), bls)
	require.NoError(err)

	require.NoError(ctx.VerifyEndorsement(100, vote, en))
}

func TestRollDPoSCtx_VerifyEndorsement_PreForkAcceptsECDSA(t *testing.T) {
	require := require.New(t)
	ecdsa, _ := blsTestKeys(t, 0x13)
	addr := ecdsa.PublicKey().Address().String()
	round := roundCtxWithBLS(addr, nil)
	ctx := rollDPoSCtxWithBLSGate(t, round, math.MaxUint64) // BLS off

	blkHash := hash.Hash256b([]byte("blk"))
	vote := NewConsensusVote(blkHash[:], COMMIT)
	ens, err := endorsement.Endorse(vote, time.Unix(1700000000, 0), ecdsa)
	require.NoError(err)

	require.NoError(ctx.VerifyEndorsement(1, vote, ens[0]))
}

func TestRollDPoSCtx_NewEndorsement_PostForkProducesBLS(t *testing.T) {
	require := require.New(t)
	ecdsa, bls := blsTestKeys(t, 0x20)
	addr := ecdsa.PublicKey().Address().String()
	round := roundCtxWithBLS(addr, bls.PublicKey())
	round.height = 100
	round.roundStartTime = time.Unix(1700000000, 0)
	ctx := rollDPoSCtxWithBLSGate(t, round, 0) // BLS on
	ctx.producerKeys = []producerKey{{address: addr, ecdsa: ecdsa, bls: bls}}

	blkHash := hash.Hash256b([]byte("blk"))
	msgs, err := ctx.newEndorsement(blkHash[:], COMMIT, ctx.round.StartTime())
	require.NoError(err)
	require.Equal(1, len(msgs))
	sig := msgs[0].Endorsement().Signature()
	require.Equal(crypto.BLSAggregateSignatureLength, len(sig),
		"post-fork newEndorsement must emit a 96-byte BLS signature")

	// the resulting endorsement must verify through the same roundCtx
	vote := NewConsensusVote(blkHash[:], COMMIT)
	require.NoError(round.verifyEndorsement(vote, msgs[0].Endorsement()))
}

func TestRollDPoSCtx_NewEndorsement_PreForkProducesECDSA(t *testing.T) {
	require := require.New(t)
	ecdsa, bls := blsTestKeys(t, 0x21)
	addr := ecdsa.PublicKey().Address().String()
	round := roundCtxWithBLS(addr, bls.PublicKey())
	round.height = 1
	round.roundStartTime = time.Unix(1700000000, 0)
	ctx := rollDPoSCtxWithBLSGate(t, round, math.MaxUint64) // BLS off
	ctx.producerKeys = []producerKey{{address: addr, ecdsa: ecdsa, bls: bls}}

	blkHash := hash.Hash256b([]byte("blk"))
	msgs, err := ctx.newEndorsement(blkHash[:], COMMIT, ctx.round.StartTime())
	require.NoError(err)
	require.Equal(1, len(msgs))
	sig := msgs[0].Endorsement().Signature()
	require.Equal(crypto.Secp256k1SigSizeWithRecID, len(sig),
		"pre-fork newEndorsement must emit a 65-byte secp256k1 signature")
}

func TestRoundCtx_VerifyEndorsement_LengthDispatch(t *testing.T) {
	require := require.New(t)
	ecdsa, bls := blsTestKeys(t, 0x06)
	addr := ecdsa.PublicKey().Address().String()
	ctx := roundCtxWithBLS(addr, bls.PublicKey())

	blkHash := hash.Hash256b([]byte("block-E"))
	vote := NewConsensusVote(blkHash[:], COMMIT)

	// secp256k1 signature uses ECDSA path even though blsPubKeys is populated
	ens, err := endorsement.Endorse(vote, time.Unix(1700000000, 0), ecdsa)
	require.NoError(err)
	require.NoError(ctx.verifyEndorsement(vote, ens[0]))

	// BLS signature uses BLS path
	blsEn, err := endorsement.EndorseBLS(vote, time.Unix(1700000000, 0), ecdsa.PublicKey(), bls)
	require.NoError(err)
	require.NoError(ctx.verifyEndorsement(vote, blsEn))
}
