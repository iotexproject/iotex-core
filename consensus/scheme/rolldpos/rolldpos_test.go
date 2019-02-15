// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"fmt"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/protogen/iotexrpc"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-ethereum/crypto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/p2p/node"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_explorer"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

type addrKeyPair struct {
	pubKey      keypair.PublicKey
	priKey      keypair.PrivateKey
	encodedAddr string
}

var testAddrs = []*addrKeyPair{
	newTestAddr(),
	newTestAddr(),
	newTestAddr(),
	newTestAddr(),
	newTestAddr(),
}

func newTestAddr() *addrKeyPair {
	sk, err := crypto.GenerateKey()
	if err != nil {
		log.L().Panic("Error when creating private key.", zap.Error(err))
	}
	pk := &sk.PublicKey
	pkHash := keypair.HashPubKey(pk)
	addr, err := address.FromBytes(pkHash[:])
	if err != nil {
		log.L().Panic("Error when creating address.", zap.Error(err))
	}
	return &addrKeyPair{pubKey: pk, priKey: sk, encodedAddr: addr.String()}
}

func test21Addrs() []*addrKeyPair {
	addrs := make([]*addrKeyPair, 0)
	for i := 0; i < 21; i++ {
		addrs = append(addrs, newTestAddr())
	}
	return addrs
}

func TestNewRollDPoS(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("normal", func(t *testing.T) {
		addr := newTestAddr()
		r, err := NewRollDPoSBuilder().
			SetConfig(config.RollDPoS{}).
			SetAddr(addr.encodedAddr).
			SetPubKey(addr.pubKey).
			SetPriKey(addr.priKey).
			SetBlockchain(mock_blockchain.NewMockBlockchain(ctrl)).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetBroadcast(func(_ proto.Message) error {
				return nil
			}).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, r)
	})
	t.Run("mock-clock", func(t *testing.T) {
		addr := newTestAddr()
		r, err := NewRollDPoSBuilder().
			SetConfig(config.RollDPoS{}).
			SetAddr(addr.encodedAddr).
			SetPubKey(addr.pubKey).
			SetPriKey(addr.priKey).
			SetBlockchain(mock_blockchain.NewMockBlockchain(ctrl)).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetBroadcast(func(_ proto.Message) error {
				return nil
			}).
			SetClock(clock.NewMock()).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, r)
		_, ok := r.ctx.clock.(*clock.Mock)
		assert.True(t, ok)
	})

	t.Run("root chain API", func(t *testing.T) {
		addr := newTestAddr()
		r, err := NewRollDPoSBuilder().
			SetConfig(config.RollDPoS{}).
			SetAddr(addr.encodedAddr).
			SetPubKey(addr.pubKey).
			SetPriKey(addr.priKey).
			SetBlockchain(mock_blockchain.NewMockBlockchain(ctrl)).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetBroadcast(func(_ proto.Message) error {
				return nil
			}).
			SetClock(clock.NewMock()).
			SetRootChainAPI(mock_explorer.NewMockExplorer(ctrl)).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, r)
		assert.NotNil(t, r.ctx.rootChainAPI)
	})
	t.Run("missing-dep", func(t *testing.T) {
		addr := newTestAddr()
		r, err := NewRollDPoSBuilder().
			SetConfig(config.RollDPoS{}).
			SetAddr(addr.encodedAddr).
			SetPubKey(addr.pubKey).
			SetPriKey(addr.priKey).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetBroadcast(func(_ proto.Message) error {
				return nil
			}).
			Build()
		assert.Error(t, err)
		assert.Nil(t, r)
	})
}

func TestValidateBlockFooter(t *testing.T) {
	// TODO: add unit test
}

func TestRollDPoS_Metrics(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := make([]string, 5)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = testAddrs[i].encodedAddr
	}

	clock := clock.NewMock()
	blockHeight := uint64(8)
	blk := block.NewBlockDeprecated(
		1,
		blockHeight,
		hash.Hash256{},
		testutil.TimestampNowFromClock(clock),
		testAddrs[0].pubKey,
		make([]action.SealedEnvelope, 0),
	)
	blockchain := mock_blockchain.NewMockBlockchain(ctrl)
	blockchain.EXPECT().TipHeight().Return(blockHeight).Times(1)
	blockchain.EXPECT().GetBlockByHeight(blockHeight).Return(blk, nil).Times(1)
	blockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
		{Address: candidates[0]},
		{Address: candidates[1]},
		{Address: candidates[2]},
		{Address: candidates[3]},
		{Address: candidates[4]},
	}, nil).AnyTimes()

	addr := newTestAddr()
	r, err := NewRollDPoSBuilder().
		SetConfig(config.RollDPoS{
			NumDelegates:     4,
			NumSubEpochs:     1,
			FSM:              config.Default.Consensus.RollDPoS.FSM,
			DelegateInterval: 10 * time.Second,
		}).
		SetAddr(addr.encodedAddr).
		SetPubKey(addr.pubKey).
		SetPriKey(addr.priKey).
		SetBlockchain(blockchain).
		SetActPool(mock_actpool.NewMockActPool(ctrl)).
		SetBroadcast(func(_ proto.Message) error {
			return nil
		}).
		SetClock(clock).
		Build()
	require.NoError(t, err)
	require.NotNil(t, r)
	r.ctx.round = &roundCtx{height: blockHeight + 1}
	clock.Add(r.ctx.cfg.DelegateInterval)
	require.NoError(t, r.ctx.updateEpoch(blockHeight+1))
	require.NoError(t, r.ctx.updateRound(blockHeight+1))

	m, err := r.Metrics()
	require.NoError(t, err)
	assert.Equal(t, uint64(3), m.LatestEpoch)

	cp.SortCandidates(candidates, m.LatestEpoch, cp.CryptoSeed)
	assert.Equal(t, candidates[:4], m.LatestDelegates)
	assert.Equal(t, candidates[1], m.LatestBlockProducer)
}

func makeTestRollDPoSCtx(
	addr *addrKeyPair,
	ctrl *gomock.Controller,
	cfg config.RollDPoS,
	mockChain func(*mock_blockchain.MockBlockchain),
	mockActPool func(*mock_actpool.MockActPool),
	broadcastCB func(proto.Message) error,
	clock clock.Clock,
) *rollDPoSCtx {
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	mockChain(chain)
	actPool := mock_actpool.NewMockActPool(ctrl)
	mockActPool(actPool)
	if broadcastCB == nil {
		broadcastCB = func(proto.Message) error {
			return nil
		}
	}
	return &rollDPoSCtx{
		cfg:              cfg,
		encodedAddr:      addr.encodedAddr,
		pubKey:           addr.pubKey,
		priKey:           addr.priKey,
		chain:            chain,
		actPool:          actPool,
		broadcastHandler: broadcastCB,
		clock:            clock,
	}
}

// E2E RollDPoS tests bellow

type directOverlay struct {
	addr  net.Addr
	peers map[net.Addr]*RollDPoS
}

func (o *directOverlay) Start(_ context.Context) error { return nil }

func (o *directOverlay) Stop(_ context.Context) error { return nil }

func (o *directOverlay) Broadcast(msg proto.Message) error {
	// Only broadcast consensus message
	if cMsg, ok := msg.(*iotexrpc.ConsensusPb); ok {
		for _, r := range o.peers {
			if err := r.HandleConsensusMsg(cMsg); err != nil {
				return errors.Wrap(err, "error when handling consensus message directly")
			}
		}
	}
	return nil
}

func (o *directOverlay) Tell(uint32, net.Addr, proto.Message) error { return nil }

func (o *directOverlay) Self() net.Addr { return o.addr }

func (o *directOverlay) GetPeers() []net.Addr {
	addrs := make([]net.Addr, 0, len(o.peers))
	for addr := range o.peers {
		addrs = append(addrs, addr)
	}
	return addrs
}

func TestRollDPoSConsensus(t *testing.T) {
	t.Parallel()

	newConsensusComponents := func(numNodes int) ([]*RollDPoS, []*directOverlay, []blockchain.Blockchain) {
		cfg := config.Default
		cfg.Consensus.RollDPoS.Delay = 300 * time.Millisecond
		cfg.Consensus.RollDPoS.DelegateInterval = time.Second
		cfg.Consensus.RollDPoS.FSM.AcceptBlockTTL = 400 * time.Millisecond
		cfg.Consensus.RollDPoS.FSM.AcceptProposalEndorsementTTL = 200 * time.Millisecond
		cfg.Consensus.RollDPoS.FSM.AcceptLockEndorsementTTL = 200 * time.Millisecond
		cfg.Consensus.RollDPoS.FSM.UnmatchedEventTTL = 400 * time.Millisecond
		cfg.Consensus.RollDPoS.FSM.UnmatchedEventInterval = 10 * time.Millisecond
		cfg.Consensus.RollDPoS.ToleratedOvertime = 200 * time.Millisecond
		cfg.Consensus.RollDPoS.NumDelegates = uint(numNodes)
		cfg.Consensus.RollDPoS.NumSubEpochs = 1

		genesisCfg := genesis.Default

		chainAddrs := make([]*addrKeyPair, 0, numNodes)
		networkAddrs := make([]net.Addr, 0, numNodes)
		for i := 0; i < numNodes; i++ {
			chainAddrs = append(chainAddrs, newTestAddr())
			networkAddrs = append(networkAddrs, node.NewTCPNode(fmt.Sprintf("127.0.0.%d:4689", i+1)))
		}

		chainRawAddrs := make([]string, 0, numNodes)
		addressMap := make(map[string]*addrKeyPair)
		for _, addr := range chainAddrs {
			chainRawAddrs = append(chainRawAddrs, addr.encodedAddr)
			addressMap[addr.encodedAddr] = addr
		}
		cp.SortCandidates(chainRawAddrs, 1, cp.CryptoSeed)
		for i, rawAddress := range chainRawAddrs {
			chainAddrs[i] = addressMap[rawAddress]
		}

		candidatesByHeightFunc := func(_ uint64) ([]*state.Candidate, error) {
			candidates := make([]*state.Candidate, 0, numNodes)
			for _, addr := range chainAddrs {
				candidates = append(candidates, &state.Candidate{Address: addr.encodedAddr})
			}
			return candidates, nil
		}

		chains := make([]blockchain.Blockchain, 0, numNodes)
		p2ps := make([]*directOverlay, 0, numNodes)
		cs := make([]*RollDPoS, 0, numNodes)
		for i := 0; i < numNodes; i++ {
			ctx := context.Background()
			sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
			require.NoError(t, err)
			require.NoError(t, sf.Start(ctx))
			for j := 0; j < numNodes; j++ {
				ws, err := sf.NewWorkingSet()
				require.NoError(t, err)
				_, err = account.LoadOrCreateAccount(ws, chainRawAddrs[j], big.NewInt(0))
				require.NoError(t, err)
				gasLimit := testutil.TestGasLimit
				wsctx := protocol.WithRunActionsCtx(ctx,
					protocol.RunActionsCtx{
						Producer:        testaddress.Addrinfo["producer"],
						GasLimit:        &gasLimit,
						EnableGasCharge: testutil.EnableGasCharge,
					})
				_, _, err = ws.RunActions(wsctx, 0, nil)
				require.NoError(t, err)
				require.NoError(t, sf.Commit(ws))
			}
			chain := blockchain.NewBlockchain(
				cfg,
				blockchain.InMemDaoOption(),
				blockchain.PrecreatedStateFactoryOption(sf),
				blockchain.GenesisOption(genesisCfg),
			)
			chain.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain, 0))
			chain.Validator().AddActionValidators(account.NewProtocol())
			chains = append(chains, chain)

			actPool, err := actpool.NewActPool(chain, cfg.ActPool)
			require.NoError(t, err)

			p2p := &directOverlay{
				addr:  networkAddrs[i],
				peers: make(map[net.Addr]*RollDPoS),
			}
			p2ps = append(p2ps, p2p)

			consensus, err := NewRollDPoSBuilder().
				SetAddr(chainAddrs[i].encodedAddr).
				SetPubKey(&chainAddrs[i].priKey.PublicKey).
				SetPriKey(chainAddrs[i].priKey).
				SetConfig(cfg.Consensus.RollDPoS).
				SetBlockchain(chain).
				SetActPool(actPool).
				SetBroadcast(p2p.Broadcast).
				SetCandidatesByHeightFunc(candidatesByHeightFunc).
				Build()
			require.NoError(t, err)

			cs = append(cs, consensus)
		}
		for i := 0; i < numNodes; i++ {
			for j := 0; j < numNodes; j++ {
				if i != j {
					p2ps[i].peers[p2ps[j].addr] = cs[j]
				}
			}
		}
		return cs, p2ps, chains
	}

	t.Run("1-block", func(t *testing.T) {
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(21)

		for i := 0; i < 21; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(21)
		for i := 0; i < 21; i++ {
			go func(idx int) {
				defer wg.Done()
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 21; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		assert.NoError(t, testutil.WaitUntil(100*time.Millisecond, 2*time.Second, func() (bool, error) {
			for _, chain := range chains {
				if blk, err := chain.GetBlockByHeight(1); blk == nil || err != nil {
					return false, nil
				}
			}
			return true, nil
		}))
	})

	t.Run("1-epoch", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skip the 1-epoch test in short mode.")
		}
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(21)

		for i := 0; i < 21; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(21)
		for i := 0; i < 21; i++ {
			go func(idx int) {
				defer wg.Done()
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 21; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		assert.NoError(t, testutil.WaitUntil(100*time.Millisecond, 45*time.Second, func() (bool, error) {
			for _, chain := range chains {
				if blk, err := chain.GetBlockByHeight(42); blk == nil || err != nil {
					return false, nil
				}
			}
			return true, nil
		}))
	})

	t.Run("network-partition-time-rotation", func(t *testing.T) {
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(21)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 1 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[1].addr)
			}
		}

		for i := 0; i < 21; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(21)
		for i := 0; i < 21; i++ {
			go func(idx int) {
				defer wg.Done()
				cs[idx].ctx.cfg.TimeBasedRotation = true
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 21; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()

		assert.NoError(t, testutil.WaitUntil(100*time.Millisecond, 30*time.Second, func() (bool, error) {
			for i, chain := range chains {
				if i == 1 {
					continue
				}
				blk, err := chain.GetBlockByHeight(4)
				if blk == nil || err != nil {
					return false, nil
				}
			}
			return true, nil
		}))
	})

	t.Run("proposer-network-partition-blocking", func(t *testing.T) {
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(21)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 1 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[1].addr)
			}
		}

		for i := 0; i < 21; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(21)
		for i := 0; i < 21; i++ {
			go func(idx int) {
				defer wg.Done()
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 21; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		time.Sleep(2 * time.Second)
		for _, chain := range chains {
			blk, err := chain.GetBlockByHeight(1)
			assert.Nil(t, blk)
			assert.Error(t, err)
		}
	})

	t.Run("non-proposer-network-partition-blocking", func(t *testing.T) {
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(21)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 0 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[0].addr)
			}
		}

		for i := 0; i < 21; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(21)
		for i := 0; i < 21; i++ {
			go func(idx int) {
				defer wg.Done()
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 21; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		time.Sleep(5 * time.Second)
		for i, chain := range chains {
			blk, err := chain.GetBlockByHeight(1)
			if i == 0 {
				assert.Nil(t, blk)
				assert.Error(t, err)
			} else {
				assert.NotNil(t, blk)
				assert.NoError(t, err)
			}
		}
	})
}
