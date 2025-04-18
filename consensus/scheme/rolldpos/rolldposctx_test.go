// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/endorsement"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

var dummyCandidatesByHeightFunc = func(uint64, []byte) ([]string, error) { return nil, nil }

func TestRollDPoSCtx(t *testing.T) {
	require := require.New(t)
	cfg := DefaultConfig
	g := genesis.TestDefault()
	dbConfig := db.DefaultConfig
	dbConfig.DbPath = DefaultConfig.ConsensusDBPath
	b, sf, _, _, _ := makeChain(t)

	t.Run("case 1:panic because of chain is nil", func(t *testing.T) {
		_, err := NewRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg.FSM, consensusfsm.DefaultDardanellesUpgradeConfig, consensusfsm.DefaultWakeUpgradeConfig, g, cfg.Delay), dbConfig, true, time.Second, true, nil, block.NewDeserializer(0), nil, nil, dummyCandidatesByHeightFunc, dummyCandidatesByHeightFunc, nil, nil, 0)
		require.Error(err)
	})

	t.Run("case 2:panic because of rp is nil", func(t *testing.T) {
		_, err := NewRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg.FSM, consensusfsm.DefaultDardanellesUpgradeConfig, consensusfsm.DefaultWakeUpgradeConfig, g, cfg.Delay), dbConfig, true, time.Second, true, NewChainManager(b, sf, &dummyBlockBuildFactory{}), block.NewDeserializer(0), nil, nil, dummyCandidatesByHeightFunc, dummyCandidatesByHeightFunc, nil, nil, 0)
		require.Error(err)
	})

	rp := rolldpos.NewProtocol(
		g.NumCandidateDelegates,
		g.NumDelegates,
		g.NumSubEpochs,
	)
	t.Run("case 3:panic because of clock is nil", func(t *testing.T) {
		_, err := NewRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg.FSM, consensusfsm.DefaultDardanellesUpgradeConfig, consensusfsm.DefaultWakeUpgradeConfig, g, cfg.Delay), dbConfig, true, time.Second, true, NewChainManager(b, sf, &dummyBlockBuildFactory{}), block.NewDeserializer(0), rp, nil, dummyCandidatesByHeightFunc, dummyCandidatesByHeightFunc, nil, nil, 0)
		require.Error(err)
	})

	c := clock.New()
	cfg.FSM.AcceptBlockTTL = time.Second * 10
	cfg.FSM.AcceptProposalEndorsementTTL = time.Second
	cfg.FSM.AcceptLockEndorsementTTL = time.Second
	cfg.FSM.CommitTTL = time.Second
	t.Run("case 4:panic because of fsm time bigger than block interval", func(t *testing.T) {
		_, err := NewRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg.FSM, consensusfsm.DefaultDardanellesUpgradeConfig, consensusfsm.DefaultWakeUpgradeConfig, g, cfg.Delay), dbConfig, true, time.Second, true, NewChainManager(b, sf, &dummyBlockBuildFactory{}), block.NewDeserializer(0), rp, nil, dummyCandidatesByHeightFunc, dummyCandidatesByHeightFunc, nil, c, 0)
		require.Error(err)
	})

	g.Blockchain.BlockInterval = time.Second * 20
	t.Run("case 5:panic because of nil CandidatesByHeight function", func(t *testing.T) {
		_, err := NewRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg.FSM, consensusfsm.DefaultDardanellesUpgradeConfig, consensusfsm.DefaultWakeUpgradeConfig, g, cfg.Delay), dbConfig, true, time.Second, true, NewChainManager(b, sf, &dummyBlockBuildFactory{}), block.NewDeserializer(0), rp, nil, nil, nil, nil, c, 0)
		require.Error(err)
	})

	t.Run("case 6:normal", func(t *testing.T) {
		bh := g.BeringBlockHeight
		rctx, err := NewRollDPoSCtx(consensusfsm.NewConsensusConfig(cfg.FSM, consensusfsm.DefaultDardanellesUpgradeConfig, consensusfsm.DefaultWakeUpgradeConfig, g, cfg.Delay), dbConfig, true, time.Second, true, NewChainManager(b, sf, &dummyBlockBuildFactory{}), block.NewDeserializer(0), rp, nil, dummyCandidatesByHeightFunc, dummyCandidatesByHeightFunc, nil, c, bh)
		require.NoError(err)
		require.Equal(bh, rctx.RoundCalculator().beringHeight)
		require.NotNil(rctx)
	})
}

func TestCheckVoteEndorser(t *testing.T) {
	require := require.New(t)
	b, sf, _, rp, pp := makeChain(t)
	c := clock.New()
	g := genesis.TestDefault()
	g.Blockchain.BlockInterval = time.Second * 20
	delegatesByEpochFunc := func(epochnum uint64, _ []byte) ([]string, error) {
		re := protocol.NewRegistry()
		if err := rp.Register(re); err != nil {
			return nil, err
		}
		tipHeight := b.TipHeight()
		ctx := genesis.WithGenesisContext(
			protocol.WithBlockchainCtx(
				protocol.WithRegistry(context.Background(), re),
				protocol.BlockchainCtx{
					Tip: protocol.TipInfo{
						Height: tipHeight,
					},
				},
			), g)
		tipEpochNum := rp.GetEpochNum(tipHeight)
		var candidatesList state.CandidateList
		var addrs []string
		var err error
		switch epochnum {
		case tipEpochNum:
			candidatesList, err = pp.Delegates(ctx, sf)
		case tipEpochNum + 1:
			candidatesList, err = pp.NextDelegates(ctx, sf)
		default:
			err = errors.Errorf("invalid epoch number %d compared to tip epoch number %d", epochnum, tipEpochNum)
		}
		if err != nil {
			return nil, err
		}
		for _, cand := range candidatesList {
			addrs = append(addrs, cand.Address)
		}
		return addrs, nil
	}
	rctx, err := NewRollDPoSCtx(
		consensusfsm.NewConsensusConfig(DefaultConfig.FSM, consensusfsm.DefaultDardanellesUpgradeConfig, consensusfsm.DefaultWakeUpgradeConfig, g, DefaultConfig.Delay),
		db.DefaultConfig,
		true,
		time.Second,
		true,
		NewChainManager(b, sf, &dummyBlockBuildFactory{}),
		block.NewDeserializer(0),
		rp,
		nil,
		delegatesByEpochFunc,
		delegatesByEpochFunc,
		nil,
		c,
		g.BeringBlockHeight,
	)
	require.NoError(err)
	require.NotNil(rctx)
	require.NoError(rctx.Start(context.Background()))

	// case 1:endorser nil caused panic
	require.Panics(func() { rctx.CheckVoteEndorser(0, nil, nil) }, "")

	// case 2:endorser address error
	en := endorsement.NewEndorsement(time.Now(), identityset.PrivateKey(3).PublicKey(), nil)
	require.Error(rctx.CheckVoteEndorser(51, nil, en))

	// case 3:normal
	en = endorsement.NewEndorsement(time.Now(), identityset.PrivateKey(10).PublicKey(), nil)
	require.NoError(rctx.CheckVoteEndorser(51, nil, en))
}

func TestCheckBlockProposer(t *testing.T) {
	require := require.New(t)
	g := genesis.TestDefault()
	b, sf, ap, rp, pp := makeChain(t)
	c := clock.New()
	g.Blockchain.BlockInterval = time.Second * 20
	delegatesByEpochFunc := func(epochnum uint64, _ []byte) ([]string, error) {
		re := protocol.NewRegistry()
		if err := rp.Register(re); err != nil {
			return nil, err
		}
		tipHeight := b.TipHeight()
		ctx := genesis.WithGenesisContext(
			protocol.WithBlockchainCtx(
				protocol.WithRegistry(context.Background(), re),
				protocol.BlockchainCtx{
					Tip: protocol.TipInfo{
						Height: tipHeight,
					},
				},
			), g)
		tipEpochNum := rp.GetEpochNum(tipHeight)
		var candidatesList state.CandidateList
		var addrs []string
		var err error
		switch epochnum {
		case tipEpochNum:
			candidatesList, err = pp.Delegates(ctx, sf)
		case tipEpochNum + 1:
			candidatesList, err = pp.NextDelegates(ctx, sf)
		default:
			err = errors.Errorf("invalid epoch number %d compared to tip epoch number %d", epochnum, tipEpochNum)
		}
		if err != nil {
			return nil, err
		}
		for _, cand := range candidatesList {
			addrs = append(addrs, cand.Address)
		}
		return addrs, nil
	}
	rctx, err := NewRollDPoSCtx(
		consensusfsm.NewConsensusConfig(DefaultConfig.FSM, consensusfsm.DefaultDardanellesUpgradeConfig, consensusfsm.DefaultWakeUpgradeConfig, g, DefaultConfig.Delay),
		db.DefaultConfig,
		true,
		time.Second,
		true,
		NewChainManager(b, sf, factory.NewMinter(sf, ap)),
		block.NewDeserializer(0),
		rp,
		nil,
		delegatesByEpochFunc,
		delegatesByEpochFunc,
		nil,
		c,
		g.BeringBlockHeight,
	)
	require.NoError(err)
	require.NotNil(rctx)
	prevHash := b.TipHash()
	block := getBlockforctx(t, 0, false, prevHash)
	en := endorsement.NewEndorsement(time.Unix(1596329600, 0), identityset.PrivateKey(10).PublicKey(), nil)
	bp := newBlockProposal(&block, []*endorsement.Endorsement{en})

	// case 1:panic caused by blockproposal is nil
	require.Panics(func() {
		rctx.CheckBlockProposer(51, nil, nil)
	}, "blockproposal is nil")

	// case 2:height != proposal.block.Height()
	require.Error(rctx.CheckBlockProposer(1, bp, nil))

	// case 3:panic caused by endorsement is nil
	require.Panics(func() {
		rctx.CheckBlockProposer(51, bp, nil)
	}, "endorsement is nil")

	// case 4:en's address is not proposer of the corresponding round
	require.Error(rctx.CheckBlockProposer(51, bp, en))

	// case 5:endorsor is not proposer of the corresponding round
	en = endorsement.NewEndorsement(time.Unix(1596329600, 0), identityset.PrivateKey(22).PublicKey(), nil)
	require.Error(rctx.CheckBlockProposer(51, bp, en))

	// case 6:invalid block signature
	block = getBlockforctx(t, 1, false, prevHash)
	en = endorsement.NewEndorsement(time.Unix(1596329600, 0), identityset.PrivateKey(1).PublicKey(), nil)
	bp = newBlockProposal(&block, []*endorsement.Endorsement{en})
	require.Error(rctx.CheckBlockProposer(51, bp, en))

	// case 7:invalid endorsement for the vote when call AddVoteEndorsement
	block = getBlockforctx(t, 1, true, prevHash)
	en = endorsement.NewEndorsement(time.Unix(1596329600, 0), identityset.PrivateKey(1).PublicKey(), nil)
	en2 := endorsement.NewEndorsement(time.Unix(1596329600, 0), identityset.PrivateKey(7).PublicKey(), nil)
	bp = newBlockProposal(&block, []*endorsement.Endorsement{en2, en})
	require.Error(rctx.CheckBlockProposer(51, bp, en2))

	// case 8:Insufficient endorsements
	block = getBlockforctx(t, 1, true, prevHash)
	hash := block.HashBlock()
	vote := NewConsensusVote(hash[:], COMMIT)
	ens, err := endorsement.Endorse(vote, time.Unix(1562382592, 0), identityset.PrivateKey(7))
	require.Equal(1, len(ens))
	require.NoError(err)
	bp = newBlockProposal(&block, ens)
	require.Error(rctx.CheckBlockProposer(51, bp, ens[0]))

	// case 9:normal
	block = getBlockforctx(t, 1, true, prevHash)
	bp = newBlockProposal(&block, []*endorsement.Endorsement{en})
	require.NoError(rctx.CheckBlockProposer(51, bp, en))
}

func TestNotProducingMultipleBlocks(t *testing.T) {
	require := require.New(t)
	b, sf, _, rp, pp := makeChain(t)
	c := clock.New()
	g := genesis.TestDefault()
	g.Blockchain.BlockInterval = time.Second * 20
	delegatesByEpoch := func(epochnum uint64, _ []byte) ([]string, error) {
		re := protocol.NewRegistry()
		if err := rp.Register(re); err != nil {
			return nil, err
		}
		tipHeight := b.TipHeight()
		ctx := genesis.WithGenesisContext(
			protocol.WithBlockchainCtx(
				protocol.WithRegistry(context.Background(), re),
				protocol.BlockchainCtx{
					Tip: protocol.TipInfo{
						Height: tipHeight,
					},
				},
			), g)
		tipEpochNum := rp.GetEpochNum(tipHeight)
		var candidatesList state.CandidateList
		var addrs []string
		var err error
		switch epochnum {
		case tipEpochNum:
			candidatesList, err = pp.Delegates(ctx, sf)
		case tipEpochNum + 1:
			candidatesList, err = pp.NextDelegates(ctx, sf)
		default:
			err = errors.Errorf("invalid epoch number %d compared to tip epoch number %d", epochnum, tipEpochNum)
		}
		if err != nil {
			return nil, err
		}
		for _, cand := range candidatesList {
			addrs = append(addrs, cand.Address)
		}
		return addrs, nil
	}
	rctx, err := NewRollDPoSCtx(
		consensusfsm.NewConsensusConfig(DefaultConfig.FSM, consensusfsm.DefaultDardanellesUpgradeConfig, consensusfsm.DefaultWakeUpgradeConfig, g, DefaultConfig.Delay),
		db.DefaultConfig,
		true,
		time.Second,
		true,
		NewChainManager(b, sf, &dummyBlockBuildFactory{}),
		block.NewDeserializer(0),
		rp,
		nil,
		delegatesByEpoch,
		delegatesByEpoch,
		[]crypto.PrivateKey{identityset.PrivateKey(10)},
		c,
		g.BeringBlockHeight,
	)
	require.NoError(err)
	require.NotNil(rctx)
	require.NoError(rctx.Start(context.Background()))
	defer rctx.Stop(context.Background())

	res, err := rctx.Proposal()
	require.NoError(err)
	ecm, ok := res.(*EndorsedConsensusMessage)
	require.True(ok)
	hash1, err := ecm.Document().Hash()
	require.NoError(err)
	height1 := ecm.Height()

	res2, err := rctx.Proposal()
	require.NoError(err)
	ecm2, ok := res2.(*EndorsedConsensusMessage)
	require.True(ok)
	hash2, err := ecm2.Document().Hash()
	require.NoError(err)
	require.Equal(hash1, hash2)
	height2 := ecm2.Height()
	require.Equal(height1, height2)
}

func getBlockforctx(t *testing.T, i int, sign bool, prevHash hash.Hash256) block.Block {
	require := require.New(t)
	ts := &timestamppb.Timestamp{Seconds: 1596329600, Nanos: 10}
	hcore := &iotextypes.BlockHeaderCore{
		Version:          1,
		Height:           51,
		Timestamp:        ts,
		PrevBlockHash:    prevHash[:],
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
