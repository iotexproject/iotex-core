package rolldpos

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

type mockChain struct {
	draft         uint64
	finalized     uint64
	finalizedHash map[string]bool
	blocks        map[uint64]*block.Block
	draftBlocks   map[uint64]*block.Header
	mutex         sync.RWMutex
}

func (m *mockChain) OngoingBlockHeight() uint64 {
	log.L().Debug("OngoingBlockHeight enter")
	m.mutex.RLock()
	log.L().Debug("OngoingBlockHeight locked")
	defer m.mutex.RUnlock()
	return m.draft
}

func (m *mockChain) NewDraft() uint64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.draft++
	return m.draft
}

func (m *mockChain) NewDraftBlock(blk *block.Block) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.draftBlocks[blk.Height()] = &blk.Header
}

func (m *mockChain) Finalized(blkHash []byte) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if _, ok := m.finalizedHash[string(blkHash)]; ok {
		return true
	}
	return false
}

func (m *mockChain) FinalizeBlock(blk *block.Block) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	height := blk.Height()
	hash := blk.HashBlock()
	if height != m.finalized+1 {
		return errors.Errorf("cannot finalize block %d, current finalized block is %d", height, m.finalized)
	}
	m.finalized = height
	m.finalizedHash[string(hash[:])] = true
	m.blocks[height] = blk
	delete(m.draftBlocks, height)
	return nil
}

func (m *mockChain) Block(height uint64) (*block.Block, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	blk, ok := m.blocks[height]
	if !ok {
		return nil, errors.Errorf("block %d not found", height)
	}
	return blk, nil
}

func (m *mockChain) PendingBlockHeader(height uint64) (*block.Header, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	var header *block.Header
	blk, ok := m.blocks[height]
	if !ok {
		header, ok = m.draftBlocks[height]
		if !ok {
			return nil, errors.Errorf("block %d not found", height)
		}
	} else {
		header = &blk.Header
	}
	return header, nil
}

func (m *mockChain) TipHeight() uint64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.finalized
}

func (m *mockChain) CancelBlock(height uint64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	log.L().Debug("cancel block", zap.Uint64("height", height), zap.Uint64("draft", m.draft))
	for i := height; i <= m.draft; i++ {
		delete(m.draftBlocks, i)
	}
	if height > 0 {
		m.draft = height - 1
	} else {
		m.draft = 0
	}
}

func TestChainedRollDPoS(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	r := require.New(t)
	g := genesis.Default
	g.NumCandidateDelegates = 4
	g.NumDelegates = 1
	builderCfg := BuilderConfig{
		Chain:              blockchain.DefaultConfig,
		Consensus:          DefaultConfig,
		DardanellesUpgrade: consensusfsm.DefaultDardanellesUpgradeConfig,
		DB:                 db.DefaultConfig,
		Genesis:            g,
		SystemActive:       true,
	}
	consensusdb, err := testutil.PathOfTempFile("consensus")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(consensusdb)
	}()

	builderCfg.Consensus.ConsensusDBPath = consensusdb
	builderCfg.Consensus.Delay = 400 * time.Millisecond
	builderCfg.Consensus.ToleratedOvertime = 400 * time.Millisecond
	builderCfg.Consensus.FSM.AcceptBlockTTL = 1 * time.Second
	builderCfg.Consensus.FSM.AcceptProposalEndorsementTTL = 1 * time.Second
	builderCfg.Consensus.FSM.AcceptLockEndorsementTTL = 1 * time.Second
	builderCfg.Consensus.FSM.CommitTTL = 1 * time.Second
	builderCfg.Consensus.FSM.UnmatchedEventTTL = 1 * time.Second
	builderCfg.Consensus.FSM.UnmatchedEventInterval = 100 * time.Millisecond
	builderCfg.Consensus.FSM.EventChanSize = 10000
	builderCfg.Genesis.BlockInterval = 5 * time.Second
	builderCfg.DardanellesUpgrade.AcceptBlockTTL = 1 * time.Second
	builderCfg.DardanellesUpgrade.AcceptLockEndorsementTTL = 1 * time.Second
	builderCfg.DardanellesUpgrade.AcceptProposalEndorsementTTL = 1 * time.Second
	builderCfg.DardanellesUpgrade.BlockInterval = 5 * time.Second
	builderCfg.DardanellesUpgrade.CommitTTL = 1 * time.Second
	builderCfg.DardanellesUpgrade.Delay = 400 * time.Millisecond
	builderCfg.DardanellesUpgrade.UnmatchedEventInterval = 100 * time.Millisecond
	builderCfg.DardanellesUpgrade.UnmatchedEventTTL = 1 * time.Second

	chain := &mockChain{
		finalizedHash: make(map[string]bool),
		blocks:        make(map[uint64]*block.Block),
		draftBlocks:   make(map[uint64]*block.Header),
	}
	bc := mock_blockchain.NewMockBlockchain(ctrl)
	bc.EXPECT().EvmNetworkID().Return(uint32(1)).AnyTimes()
	bc.EXPECT().Genesis().Return(g).AnyTimes()
	bc.EXPECT().MintNewBlock(gomock.Any()).DoAndReturn(func(timestamp time.Time) (*block.Block, error) {
		b := block.NewBuilder(block.NewRunnableActionsBuilder().Build())
		b.SetHeight(chain.NewDraft())
		b.SetTimestamp(timestamp)
		blk, err := b.SignAndBuild(identityset.PrivateKey(1))
		if err != nil {
			return nil, err
		}
		log.L().Debug("mint new block", zap.Time("ts", timestamp), zap.Uint64("height", blk.Height()))
		chain.NewDraftBlock(&blk)
		return &blk, nil
	}).AnyTimes()
	bc.EXPECT().ValidateBlock(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	bc.EXPECT().CommitBlock(gomock.Any()).DoAndReturn(func(blk *block.Block) error {
		return chain.FinalizeBlock(blk)
	}).AnyTimes()
	bc.EXPECT().ChainAddress().Return(builderCfg.Chain.Address).AnyTimes()
	bc.EXPECT().BlockFooterByHeight(gomock.Any()).DoAndReturn(func(height uint64) (*block.Footer, error) {
		blk, err := chain.Block(height)
		if err != nil {
			return nil, errors.Wrapf(err, "block %d not found", height)
		}
		return &blk.Footer, nil
	}).AnyTimes()
	bc.EXPECT().BlockHeaderByHeight(gomock.Any()).DoAndReturn(func(height uint64) (*block.Header, error) {
		blk, err := chain.PendingBlockHeader(height)
		if err != nil {
			return nil, errors.Wrapf(err, "block %d not found", height)
		}
		return blk, nil
	}).AnyTimes()
	bc.EXPECT().TipHeight().DoAndReturn(func() uint64 {
		return chain.TipHeight()
	}).AnyTimes()
	clock := clock.New()
	broadcastHandler := func(msg proto.Message) error { return nil }
	delegatesByEpochFunc := func(uint64) ([]string, error) {
		delegates := []string{}
		for i := 1; i <= int(g.NumCandidateDelegates); i++ {
			delegates = append(delegates, identityset.Address(i).String())
		}
		return delegates, nil
	}
	proposersByEpochFunc := func(uint64) ([]string, error) {
		return []string{
			identityset.Address(1).String(),
		}, nil
	}
	rp := rolldpos.NewProtocol(
		g.NumCandidateDelegates,
		g.NumDelegates,
		g.NumSubEpochs,
	)
	bd := NewRollDPoSBuilder().
		SetAddr(identityset.Address(1).String()).
		SetPriKey(identityset.PrivateKey(1)).
		SetConfig(builderCfg).
		SetChainManager(NewChainManager2(bc, chain)).
		SetBlockDeserializer(block.NewDeserializer(bc.EvmNetworkID())).
		SetClock(clock).
		SetBroadcast(broadcastHandler).
		SetDelegatesByEpochFunc(delegatesByEpochFunc).
		SetProposersByEpochFunc(proposersByEpochFunc).
		RegisterProtocol(rp)
	crdpos := NewChainedRollDPoS(bd)
	r.NoError(crdpos.Start(context.Background()))
	defer func() {
		r.NoError(crdpos.Stop(context.Background()))
	}()

	time.Sleep(60 * time.Second)
}
