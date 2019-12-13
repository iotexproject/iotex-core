// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
)

var dummyCandidatesByHeightFunc = func(context.Context, uint64) (state.CandidateList, error) { return nil, nil }

func TestRollDPoSCtx(t *testing.T) {
	require := require.New(t)
	cfg := config.Default
	dbConfig := config.Default.DB
	dbConfig.DbPath = config.Default.Consensus.RollDPoS.ConsensusDBPath
	b, _, _ := makeChain(t)

	t.Run("case 1:panic because of chain is nil", func(t *testing.T) {
		_, err := newRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg), dbConfig, true, time.Second, true, nil, nil, nil, nil, dummyCandidatesByHeightFunc, "", nil, nil, 0)
		require.Error(err)
	})

	t.Run("case 2:panic because of rp is nil", func(t *testing.T) {
		_, err := newRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg), dbConfig, true, time.Second, true, b, nil, nil, nil, dummyCandidatesByHeightFunc, "", nil, nil, 0)
		require.Error(err)
	})

	rp := rolldpos.NewProtocol(
		config.Default.Genesis.NumCandidateDelegates,
		config.Default.Genesis.NumDelegates,
		config.Default.Genesis.NumSubEpochs,
	)
	t.Run("case 3:panic because of clock is nil", func(t *testing.T) {
		_, err := newRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg), dbConfig, true, time.Second, true, b, nil, rp, nil, dummyCandidatesByHeightFunc, "", nil, nil, 0)
		require.Error(err)
	})

	c := clock.New()
	cfg.Consensus.RollDPoS.FSM.AcceptBlockTTL = time.Second * 10
	cfg.Consensus.RollDPoS.FSM.AcceptProposalEndorsementTTL = time.Second
	cfg.Consensus.RollDPoS.FSM.AcceptLockEndorsementTTL = time.Second
	cfg.Consensus.RollDPoS.FSM.CommitTTL = time.Second
	t.Run("case 4:panic because of fsm time bigger than block interval", func(t *testing.T) {
		_, err := newRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg), dbConfig, true, time.Second, true, b, nil, rp, nil, dummyCandidatesByHeightFunc, "", nil, c, 0)
		require.Error(err)
	})

	cfg.Genesis.Blockchain.BlockInterval = time.Second * 20
	t.Run("case 5:panic because of nil CandidatesByHeight function", func(t *testing.T) {
		_, err := newRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg), dbConfig, true, time.Second, true, b, nil, rp, nil, nil, "", nil, c, 0)
		require.Error(err)
	})

	t.Run("case 6:normal", func(t *testing.T) {
		bh := config.Default.Genesis.BeringBlockHeight
		rctx, err := newRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg), dbConfig, true, time.Second, true, b, nil, rp, nil, dummyCandidatesByHeightFunc, "", nil, c, bh)
		require.NoError(err)
		require.Equal(bh, rctx.roundCalc.beringHeight)
		require.NotNil(rctx)
	})
}

func TestCheckVoteEndorser(t *testing.T) {
	require := require.New(t)
	cfg := config.Default
	b, sf, _ := makeChain(t)
	rp := rolldpos.NewProtocol(
		config.Default.Genesis.NumCandidateDelegates,
		config.Default.Genesis.NumDelegates,
		config.Default.Genesis.NumSubEpochs,
	)
	c := clock.New()
	cfg.Genesis.BlockInterval = time.Second * 20
	rctx, err := newRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg), config.Default.DB, true, time.Second, true, b, nil, rp, nil, func(ctx context.Context, height uint64) (state.CandidateList, error) {
		return candidatesutil.CandidatesByHeight(sf, rp.GetEpochHeight(rp.GetEpochNum(height)))
	}, "", nil, c, config.Default.Genesis.BeringBlockHeight)
	require.NoError(err)
	require.NotNil(rctx)

	// case 1:endorser nil caused panic
	require.Panics(func() { rctx.CheckVoteEndorser(0, nil, nil) }, "")

	// case 2:endorser address error
	en := endorsement.NewEndorsement(time.Now(), identityset.PrivateKey(0).PublicKey(), nil)
	require.Error(rctx.CheckVoteEndorser(0, nil, en))

	// case 3:normal
	en = endorsement.NewEndorsement(time.Now(), identityset.PrivateKey(10).PublicKey(), nil)
	require.NoError(rctx.CheckVoteEndorser(1, nil, en))
}

func TestCheckBlockProposer(t *testing.T) {
	require := require.New(t)
	cfg := config.Default
	b, sf, rp := makeChain(t)
	c := clock.New()
	cfg.Genesis.BlockInterval = time.Second * 20
	rctx, err := newRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg), config.Default.DB, true, time.Second, true, b, nil, rp, nil, func(ctx context.Context, height uint64) (state.CandidateList, error) {
		return candidatesutil.CandidatesByHeight(sf, rp.GetEpochHeight(rp.GetEpochNum(height)))
	}, "", nil, c, config.Default.Genesis.BeringBlockHeight)
	require.NoError(err)
	require.NotNil(rctx)
	block := getBlockforctx(t, 0, false)
	en := endorsement.NewEndorsement(time.Unix(1562382392, 0), identityset.PrivateKey(10).PublicKey(), nil)
	bp := newBlockProposal(&block, []*endorsement.Endorsement{en})

	// case 1:panic caused by blockproposal is nil
	require.Panics(func() {
		rctx.CheckBlockProposer(1, nil, nil)
	}, "blockproposal is nil")

	// case 2:height != proposal.block.Height()
	require.Error(rctx.CheckBlockProposer(1, bp, nil))

	// case 3:panic caused by endorsement is nil
	require.Panics(func() {
		rctx.CheckBlockProposer(21, bp, nil)
	}, "endorsement is nil")

	// case 4:en's address is not proposer of the corresponding round
	require.Error(rctx.CheckBlockProposer(21, bp, en))

	// case 5:endorsor is not proposer of the corresponding round
	en = endorsement.NewEndorsement(time.Unix(1562382492, 0), identityset.PrivateKey(22).PublicKey(), nil)
	require.Error(rctx.CheckBlockProposer(21, bp, en))

	// case 6:invalid block signature
	block = getBlockforctx(t, 5, false)
	en = endorsement.NewEndorsement(time.Unix(1562382392, 0), identityset.PrivateKey(5).PublicKey(), nil)
	bp = newBlockProposal(&block, []*endorsement.Endorsement{en})
	require.Error(rctx.CheckBlockProposer(21, bp, en))

	// case 7:invalid endorsement for the vote when call AddVoteEndorsement
	block = getBlockforctx(t, 5, true)
	en = endorsement.NewEndorsement(time.Unix(1562382392, 0), identityset.PrivateKey(5).PublicKey(), nil)
	en2 := endorsement.NewEndorsement(time.Unix(1562382592, 0), identityset.PrivateKey(7).PublicKey(), nil)
	bp = newBlockProposal(&block, []*endorsement.Endorsement{en2, en})
	require.Error(rctx.CheckBlockProposer(21, bp, en2))

	// case 8:Insufficient endorsements
	block = getBlockforctx(t, 5, true)
	hash := block.HashBlock()
	vote := NewConsensusVote(hash[:], COMMIT)
	en2, err = endorsement.Endorse(identityset.PrivateKey(7), vote, time.Unix(1562382592, 0))
	require.NoError(err)
	bp = newBlockProposal(&block, []*endorsement.Endorsement{en2})
	require.Error(rctx.CheckBlockProposer(21, bp, en2))

	// case 9:normal
	block = getBlockforctx(t, 5, true)
	bp = newBlockProposal(&block, []*endorsement.Endorsement{en})
	require.NoError(rctx.CheckBlockProposer(21, bp, en))
}

func getBlockforctx(t *testing.T, i int, sign bool) block.Block {
	require := require.New(t)
	ts := &timestamp.Timestamp{Seconds: 1562382392, Nanos: 10}
	hcore := &iotextypes.BlockHeaderCore{
		Version:          1,
		Height:           21,
		Timestamp:        ts,
		PrevBlockHash:    []byte(""),
		TxRoot:           []byte(""),
		DeltaStateDigest: []byte(""),
		ReceiptRoot:      []byte(""),
	}
	header := block.Header{}
	protoHeader := &iotextypes.BlockHeader{Core: hcore, ProducerPubkey: identityset.PrivateKey(i).PublicKey().Bytes()}
	require.NoError(header.LoadFromBlockHeaderProto(protoHeader))

	if sign {
		hash := header.HashHeaderCore()
		sig, err := identityset.PrivateKey(i).Sign(hash[:])
		require.NoError(err)
		protoHeader = &iotextypes.BlockHeader{Core: hcore, ProducerPubkey: identityset.PrivateKey(i).PublicKey().Bytes(), Signature: sig}
		require.NoError(header.LoadFromBlockHeaderProto(protoHeader))
	}

	b := block.Block{Header: header}
	return b
}
