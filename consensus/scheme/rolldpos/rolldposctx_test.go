// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestRollDPoSCtx(t *testing.T) {
	require := require.New(t)
	cfg := config.Default.Consensus.RollDPoS

	// case 1:panic because of chain is nil
	require.Panics(func() {
		newRollDPoSCtx(cfg, true, time.Second*10, time.Second, true, nil, nil, nil, nil, nil, "", nil, nil)
	}, "chain is nil")

	// case 2:panic because of rp is nil
	b, _ := makeChain(t)
	require.Panics(func() {
		newRollDPoSCtx(cfg, true, time.Second*10, time.Second, true, b, nil, nil, nil, nil, "", nil, nil)
	}, "rp is nil")

	// case 3:panic because of clock is nil
	rp := rolldpos.NewProtocol(
		config.Default.Genesis.NumCandidateDelegates,
		config.Default.Genesis.NumDelegates,
		config.Default.Genesis.NumSubEpochs,
	)
	require.Panics(func() {
		newRollDPoSCtx(cfg, true, time.Second*10, time.Second, true, b, nil, rp, nil, nil, "", nil, nil)
	}, "clock is nil")

	// case 4:panic because of fsm time bigger than block interval
	c := clock.New()
	cfg.FSM.AcceptBlockTTL = time.Second * 10
	cfg.FSM.AcceptProposalEndorsementTTL = time.Second
	cfg.FSM.AcceptLockEndorsementTTL = time.Second
	cfg.FSM.CommitTTL = time.Second
	require.Panics(func() {
		newRollDPoSCtx(cfg, true, time.Second*10, time.Second, true, b, nil, rp, nil, nil, "", nil, c)
	}, "fsm's time is bigger than block interval")

	// case 5:normal
	rctx := newRollDPoSCtx(cfg, true, time.Second*20, time.Second, true, b, nil, rp, nil, nil, "", nil, c)
	require.NotNil(rctx)
}

func TestCheckVoteEndorser(t *testing.T) {
	require := require.New(t)
	cfg := config.Default.Consensus.RollDPoS
	b, _ := makeChain(t)
	rp := rolldpos.NewProtocol(
		config.Default.Genesis.NumCandidateDelegates,
		config.Default.Genesis.NumDelegates,
		config.Default.Genesis.NumSubEpochs,
	)
	c := clock.New()
	rctx := newRollDPoSCtx(cfg, true, time.Second*20, time.Second, true, b, nil, rp, nil, nil, "", nil, c)
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
	cfg := config.Default.Consensus.RollDPoS
	b, rp := makeChain(t)
	c := clock.New()
	rctx := newRollDPoSCtx(cfg, true, time.Second*20, time.Second, true, b, nil, rp, nil, nil, "", nil, c)
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

	// case 5:endorsor is not proposer of the correpsonding round
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
	en2, err := endorsement.Endorse(identityset.PrivateKey(7), vote, time.Unix(1562382592, 0))
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
