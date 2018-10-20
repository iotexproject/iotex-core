// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/network/node"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_network"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestRollDPoSCtx(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := make([]string, 4)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = testAddrs[i].RawAddress
	}

	clock := clock.NewMock()
	var prevHash hash.Hash32B
	blk := blockchain.NewBlock(
		1,
		8,
		prevHash,
		testutil.TimestampNowFromClock(clock),
		make([]*action.Transfer, 0),
		make([]*action.Vote, 0),
		make([]*action.Execution, 0),
		make([]action.Action, 0),
	)
	ctx := makeTestRollDPoSCtx(
		testAddrs[0],
		ctrl,
		config.RollDPoS{
			NumSubEpochs: 1,
			NumDelegates: 4,
			EnableDKG:    true,
		},
		func(blockchain *mock_blockchain.MockBlockchain) {
			blockchain.EXPECT().TipHeight().Return(uint64(8)).Times(4)
			blockchain.EXPECT().GetBlockByHeight(uint64(8)).Return(blk, nil).Times(1)
			blockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
				{Address: candidates[0]},
				{Address: candidates[1]},
				{Address: candidates[2]},
				{Address: candidates[3]},
			}, nil).Times(1)
		},
		func(_ *mock_actpool.MockActPool) {},
		func(_ *mock_network.MockOverlay) {},
		clock,
	)

	epoch, height, err := ctx.calcEpochNumAndHeight()
	require.NoError(t, err)
	assert.Equal(t, uint64(2), epoch)
	assert.Equal(t, uint64(9), height)

	ctx.epoch.height = height

	subEpoch, err := ctx.calcSubEpochNum()
	require.NoError(t, err)
	assert.Equal(t, uint64(0), subEpoch)

	ctx.epoch.seed = crypto.CryptoSeed
	delegates, err := ctx.rollingDelegates(epoch)
	require.NoError(t, err)
	crypto.SortCandidates(candidates, epoch, crypto.CryptoSeed)
	assert.Equal(t, candidates, delegates)

	ctx.epoch.num = epoch
	ctx.epoch.height = height
	ctx.epoch.numSubEpochs = 2
	ctx.epoch.delegates = delegates

	proposer, height, err := ctx.rotatedProposer()
	require.NoError(t, err)
	assert.Equal(t, candidates[1], proposer)
	assert.Equal(t, uint64(9), height)

	clock.Add(time.Second)
	duration, err := ctx.calcDurationSinceLastBlock()
	require.NoError(t, err)
	assert.Equal(t, time.Second, duration)

	yes, no := ctx.calcQuorum(map[string]bool{
		candidates[0]: true,
		candidates[1]: true,
		candidates[2]: true,
	})
	assert.True(t, yes)
	assert.False(t, no)

	yes, no = ctx.calcQuorum(map[string]bool{
		candidates[0]: false,
		candidates[1]: false,
		candidates[2]: false,
	})
	assert.False(t, yes)
	assert.True(t, no)

	yes, no = ctx.calcQuorum(map[string]bool{
		candidates[0]: true,
		candidates[1]: true,
		candidates[2]: false,
		candidates[3]: false,
	})
	assert.False(t, yes)
	assert.True(t, no)
}

func TestIsEpochFinished(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := make([]string, 4)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = testAddrs[i].RawAddress
	}

	t.Run("not-finished", func(t *testing.T) {
		ctx := makeTestRollDPoSCtx(
			testAddrs[0],
			ctrl,
			config.RollDPoS{
				NumSubEpochs: 1,
				EnableDKG:    true,
			},
			func(blockchain *mock_blockchain.MockBlockchain) {
				blockchain.EXPECT().TipHeight().Return(uint64(7)).Times(1)
			},
			func(_ *mock_actpool.MockActPool) {},
			func(_ *mock_network.MockOverlay) {},
			clock.NewMock(),
		)
		ctx.epoch.delegates = candidates
		ctx.epoch.height = 1
		ctx.epoch.numSubEpochs = 2

		finished, err := ctx.isEpochFinished()
		require.NoError(t, err)
		assert.False(t, finished)
	})
	t.Run("finished", func(t *testing.T) {
		ctx := makeTestRollDPoSCtx(
			testAddrs[0],
			ctrl,
			config.RollDPoS{
				NumSubEpochs: 1,
				EnableDKG:    true,
			},
			func(blockchain *mock_blockchain.MockBlockchain) {
				blockchain.EXPECT().TipHeight().Return(uint64(8)).Times(1)
			},
			func(_ *mock_actpool.MockActPool) {},
			func(_ *mock_network.MockOverlay) {},
			clock.NewMock(),
		)
		ctx.epoch.delegates = candidates
		ctx.epoch.height = 1
		ctx.epoch.numSubEpochs = 2

		finished, err := ctx.isEpochFinished()
		require.NoError(t, err)
		assert.True(t, finished)
	})
}

func TestIsDKGFinished(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := make([]string, 4)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = testAddrs[i].RawAddress
	}

	t.Run("not-finished", func(t *testing.T) {
		ctx := makeTestRollDPoSCtx(
			testAddrs[0],
			ctrl,
			config.RollDPoS{
				NumSubEpochs: 1,
				EnableDKG:    true,
			},
			func(blockchain *mock_blockchain.MockBlockchain) {
				blockchain.EXPECT().TipHeight().Return(uint64(3)).Times(1)
			},
			func(_ *mock_actpool.MockActPool) {},
			func(_ *mock_network.MockOverlay) {},
			clock.NewMock(),
		)
		ctx.epoch.delegates = candidates
		ctx.epoch.height = 1
		ctx.epoch.numSubEpochs = 2

		assert.False(t, ctx.isDKGFinished())
	})
	t.Run("finished", func(t *testing.T) {
		ctx := makeTestRollDPoSCtx(
			testAddrs[0],
			ctrl,
			config.RollDPoS{
				NumSubEpochs: 1,
				EnableDKG:    true,
			},
			func(blockchain *mock_blockchain.MockBlockchain) {
				blockchain.EXPECT().TipHeight().Return(uint64(4)).Times(1)
			},
			func(_ *mock_actpool.MockActPool) {},
			func(_ *mock_network.MockOverlay) {},
			clock.NewMock(),
		)
		ctx.epoch.delegates = candidates
		ctx.epoch.height = 1
		ctx.epoch.numSubEpochs = 2

		assert.True(t, ctx.isDKGFinished())
	})
}

func TestGenerateDKGSecrets(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := make([]string, 21)
	testAddrs := test21Addrs()
	for i := 0; i < len(candidates); i++ {
		candidates[i] = testAddrs[i].RawAddress
	}

	ctx := makeTestRollDPoSCtx(
		testAddrs[0],
		ctrl,
		config.RollDPoS{
			NumSubEpochs: 1,
			EnableDKG:    true,
		},
		func(blockchain *mock_blockchain.MockBlockchain) {},
		func(_ *mock_actpool.MockActPool) {},
		func(_ *mock_network.MockOverlay) {},
		clock.NewMock(),
	)

	ctx.epoch.delegates = candidates

	secrets, witness, err := ctx.generateDKGSecrets()
	assert.NoError(t, err)
	assert.Equal(t, iotxaddress.CreateID(ctx.addr.RawAddress), ctx.epoch.dkgAddress.ID)
	assert.NotNil(t, secrets)
	assert.NotNil(t, witness)
}

func TestGenerateDKGKeyPair(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := make([]string, 21)
	testAddrs := test21Addrs()
	for i := 0; i < len(candidates); i++ {
		candidates[i] = testAddrs[i].RawAddress
	}

	ctx := makeTestRollDPoSCtx(
		testAddrs[0],
		ctrl,
		config.RollDPoS{
			NumDelegates: 21,
			NumSubEpochs: 1,
			EnableDKG:    true,
		},
		func(blockchain *mock_blockchain.MockBlockchain) {},
		func(_ *mock_actpool.MockActPool) {},
		func(_ *mock_network.MockOverlay) {},
		clock.NewMock(),
	)

	ctx.epoch.delegates = candidates
	ctx.epoch.committedSecrets = make(map[string][]uint32)

	idList := make([][]uint8, 0)
	for _, addr := range ctx.epoch.delegates {
		dkgID := iotxaddress.CreateID(addr)
		idList = append(idList, dkgID)
	}
	for _, delegate := range ctx.epoch.delegates {
		_, secrets, _, err := crypto.DKG.Init(crypto.DKG.SkGeneration(), idList)
		assert.NoError(t, err)
		assert.NotNil(t, secrets)
		//if i % 2 == 0 {
		//	ctx.epoch.committedSecrets[delegate] = secrets[0]
		//}
		ctx.epoch.committedSecrets[delegate] = secrets[0]
	}
	dkgPubKey, dkgPriKey, err := ctx.generateDKGKeyPair()
	assert.NoError(t, err)
	assert.NotNil(t, dkgPubKey)
	assert.NotNil(t, dkgPriKey)
}

func TestNewRollDPoS(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("normal", func(t *testing.T) {
		r, err := NewRollDPoSBuilder().
			SetConfig(config.RollDPoS{}).
			SetAddr(newTestAddr()).
			SetBlockchain(mock_blockchain.NewMockBlockchain(ctrl)).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetP2P(mock_network.NewMockOverlay(ctrl)).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, r)
	})
	t.Run("mock-clock", func(t *testing.T) {
		r, err := NewRollDPoSBuilder().
			SetConfig(config.RollDPoS{}).
			SetAddr(newTestAddr()).
			SetBlockchain(mock_blockchain.NewMockBlockchain(ctrl)).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetP2P(mock_network.NewMockOverlay(ctrl)).
			SetClock(clock.NewMock()).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, r)
		_, ok := r.ctx.clock.(*clock.Mock)
		assert.True(t, ok)
	})
	t.Run("missing-dep", func(t *testing.T) {
		r, err := NewRollDPoSBuilder().
			SetConfig(config.RollDPoS{}).
			SetAddr(newTestAddr()).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetP2P(mock_network.NewMockOverlay(ctrl)).
			Build()
		assert.Error(t, err)
		assert.Nil(t, r)
	})
}

func TestRollDPoS_Metrics(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := make([]string, 5)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = testAddrs[i].RawAddress
	}

	blockchain := mock_blockchain.NewMockBlockchain(ctrl)
	blockchain.EXPECT().TipHeight().Return(uint64(8)).Times(2)
	blockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
		{Address: candidates[0]},
		{Address: candidates[1]},
		{Address: candidates[2]},
		{Address: candidates[3]},
		{Address: candidates[4]},
	}, nil).AnyTimes()

	r, err := NewRollDPoSBuilder().
		SetConfig(config.RollDPoS{NumDelegates: 4}).
		SetAddr(newTestAddr()).
		SetBlockchain(blockchain).
		SetActPool(mock_actpool.NewMockActPool(ctrl)).
		SetP2P(mock_network.NewMockOverlay(ctrl)).
		Build()
	require.NoError(t, err)
	require.NotNil(t, r)

	m, err := r.Metrics()
	require.NoError(t, err)
	assert.Equal(t, uint64(3), m.LatestEpoch)
	crypto.SortCandidates(candidates, m.LatestEpoch, r.ctx.epoch.seed)
	assert.Equal(t, candidates[:4], m.LatestDelegates)
	assert.Equal(t, candidates[1], m.LatestBlockProducer)
	assert.Equal(t, candidates, m.Candidates)
}

func TestRollDPoS_convertToConsensusEvt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	r, err := NewRollDPoSBuilder().
		SetConfig(config.RollDPoS{}).
		SetAddr(newTestAddr()).
		SetBlockchain(mock_blockchain.NewMockBlockchain(ctrl)).
		SetActPool(mock_actpool.NewMockActPool(ctrl)).
		SetP2P(mock_network.NewMockOverlay(ctrl)).
		Build()
	assert.NoError(t, err)
	assert.NotNil(t, r)

	// Test propose msg
	addr := newTestAddr()
	transfer, err := action.NewTransfer(1, big.NewInt(100), "src", "dst", []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	selfPubKey := testaddress.Addrinfo["producer"].PublicKey
	selfPubKeyHash := keypair.HashPubKey(selfPubKey)
	address := address.New(config.Default.Chain.ID, selfPubKeyHash[:])
	vote, err := action.NewVote(2, address.IotxAddress(), address.IotxAddress(), uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	var prevHash hash.Hash32B
	blk := blockchain.NewBlock(
		1,
		1,
		prevHash,
		testutil.TimestampNow(),
		[]*action.Transfer{transfer},
		[]*action.Vote{vote},
		nil,
		nil,
	)
	roundNum := uint32(0)
	pMsg := iproto.ProposePb{
		Block:    blk.ConvertToBlockPb(),
		Proposer: addr.RawAddress,
		Round:    roundNum,
	}
	pEvt, err := r.cfsm.newProposeBlkEvtFromProposePb(&pMsg)
	assert.NoError(t, err)
	assert.NotNil(t, pEvt)
	assert.NotNil(t, pEvt.block)

	// Test proposal endorse msg
	blkHash := blk.HashBlock()
	en, err := endorsement.NewEndorsement(
		endorsement.NewConsensusVote(
			blkHash,
			blk.Height(),
			roundNum,
			endorsement.PROPOSAL,
		),
		addr,
	)
	assert.NoError(t, err)
	msg := en.ToProtoMsg()

	eEvt, err := r.cfsm.newEndorseEvtWithEndorsePb(msg)
	assert.NoError(t, err)
	assert.NotNil(t, eEvt)

	// Test commit endorse msg
	en, err = endorsement.NewEndorsement(
		endorsement.NewConsensusVote(
			blkHash,
			blk.Height(),
			roundNum,
			endorsement.LOCK,
		),
		addr,
	)
	assert.NoError(t, err)
	msg = en.ToProtoMsg()
	eEvt, err = r.cfsm.newEndorseEvtWithEndorsePb(msg)
	assert.NoError(t, err)
	assert.NotNil(t, eEvt)
}

func TestUpdateSeed(t *testing.T) {
	require := require.New(t)
	lastSeed, _ := hex.DecodeString("9de6306b08158c423330f7a27243a1a5cbe39bfd764f07818437882d21241567")
	chain := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(chain.Start(context.Background()))
	ctx := rollDPoSCtx{cfg: config.Default.Consensus.RollDPoS, chain: chain, epoch: epochCtx{seed: lastSeed}}
	fsm := cFSM{ctx: &ctx}

	var err error
	const numNodes = 21
	addresses := make([]string, numNodes)
	skList := make([][]uint32, numNodes)
	idList := make([][]uint8, numNodes)
	coeffsList := make([][][]uint32, numNodes)
	sharesList := make([][][]uint32, numNodes)
	shares := make([][]uint32, numNodes)
	witnessesList := make([][][]byte, numNodes)
	sharestatusmatrix := make([][numNodes]bool, numNodes)
	qsList := make([][]byte, numNodes)
	pkList := make([][]byte, numNodes)
	askList := make([][]uint32, numNodes)
	ec283PKList := make([]keypair.PublicKey, numNodes)
	ec283SKList := make([]keypair.PrivateKey, numNodes)

	// Generate 21 identifiers for the delegates
	for i := 0; i < numNodes; i++ {
		var err error
		ec283PKList[i], ec283SKList[i], err = crypto.EC283.NewKeyPair()
		if err != nil {
			require.NoError(err)
		}
		pkHash := keypair.HashPubKey(ec283PKList[i])
		addresses[i] = address.New(chain.ChainID(), pkHash[:]).IotxAddress()
		idList[i] = iotxaddress.CreateID(addresses[i])
		skList[i] = crypto.DKG.SkGeneration()
	}

	// Initialize DKG and generate secret shares
	for i := 0; i < numNodes; i++ {
		coeffsList[i], sharesList[i], witnessesList[i], err = crypto.DKG.Init(skList[i], idList)
		require.NoError(err)
	}

	// Verify all the received secret shares
	for i := 0; i < numNodes; i++ {
		for j := 0; j < numNodes; j++ {
			result, err := crypto.DKG.ShareVerify(idList[i], sharesList[j][i], witnessesList[j])
			require.NoError(err)
			require.True(result)
			shares[j] = sharesList[j][i]
		}
		sharestatusmatrix[i], err = crypto.DKG.SharesCollect(idList[i], shares, witnessesList)
		require.NoError(err)
		for _, b := range sharestatusmatrix[i] {
			require.True(b)
		}
	}

	// Generate private and public key shares of a group key
	for i := 0; i < numNodes; i++ {
		for j := 0; j < numNodes; j++ {
			shares[j] = sharesList[j][i]
		}
		qsList[i], pkList[i], askList[i], err = crypto.DKG.KeyPairGeneration(shares, sharestatusmatrix)
		require.NoError(err)
	}

	// Generate dkg signature for each block
	require.NoError(err)
	dummy := chain.MintNewDummyBlock()
	require.NoError(chain.ValidateBlock(dummy, false))
	err = chain.CommitBlock(dummy)
	require.NoError(err)
	for i := 1; i < numNodes; i++ {
		iotxAddr := iotxaddress.Address{
			PublicKey:  ec283PKList[i],
			PrivateKey: ec283SKList[i],
			RawAddress: addresses[i],
		}
		blk, err := chain.MintNewBlock(nil, nil, nil, nil, &iotxAddr,
			&iotxaddress.DKGAddress{PrivateKey: askList[i], PublicKey: pkList[i], ID: idList[i]},
			lastSeed, "")
		require.NoError(err)
		require.NoError(verifyDKGSignature(blk, lastSeed))
		require.NoError(chain.ValidateBlock(blk, true))
		require.NoError(chain.CommitBlock(blk))
		require.Equal(pkList[i], blk.Header.DKGPubkey)
		require.Equal(idList[i], blk.Header.DKGID)
		require.True(len(blk.Header.DKGBlockSig) > 0)
	}
	height := chain.TipHeight()
	require.Equal(int(height), 21)

	newSeed, err := fsm.ctx.updateSeed()
	require.NoError(err)
	require.True(len(newSeed) > 0)
	require.NotEqual(fsm.ctx.epoch.seed, newSeed)
	fmt.Println(fsm.ctx.epoch.seed)
	fmt.Println(newSeed)
}

func makeTestRollDPoSCtx(
	addr *iotxaddress.Address,
	ctrl *gomock.Controller,
	cfg config.RollDPoS,
	mockChain func(*mock_blockchain.MockBlockchain),
	mockActPool func(*mock_actpool.MockActPool),
	mockP2P func(overlay *mock_network.MockOverlay),
	clock clock.Clock,
) *rollDPoSCtx {
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	mockChain(chain)
	actPool := mock_actpool.NewMockActPool(ctrl)
	mockActPool(actPool)
	p2p := mock_network.NewMockOverlay(ctrl)
	mockP2P(p2p)
	return &rollDPoSCtx{
		cfg:     cfg,
		addr:    addr,
		chain:   chain,
		actPool: actPool,
		p2p:     p2p,
		clock:   clock,
	}
}

// E2E RollDPoS tests bellow

type directOverlay struct {
	addr  net.Addr
	peers map[net.Addr]*RollDPoS
}

func (o *directOverlay) Start(_ context.Context) error { return nil }

func (o *directOverlay) Stop(_ context.Context) error { return nil }

func (o *directOverlay) Broadcast(chainID uint32, msg proto.Message) error {
	// Only broadcast consensus message
	if propose, ok := msg.(*iproto.ProposePb); ok {
		for _, r := range o.peers {
			if err := r.HandleBlockPropose(propose); err != nil {
				return errors.Wrap(err, "error when handling block propose directly")
			}
		}
	} else if endorse, ok := msg.(*iproto.EndorsePb); ok {
		for _, r := range o.peers {
			if err := r.HandleEndorse(endorse); err != nil {
				return errors.Wrap(err, "error when handling endorse directly")
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
		cfg.Consensus.RollDPoS.ProposerInterval = time.Second
		cfg.Consensus.RollDPoS.AcceptProposeTTL = 1000 * time.Millisecond
		cfg.Consensus.RollDPoS.AcceptProposalEndorseTTL = 1000 * time.Millisecond
		cfg.Consensus.RollDPoS.AcceptCommitEndorseTTL = 1000 * time.Millisecond
		cfg.Consensus.RollDPoS.NumDelegates = uint(numNodes)
		cfg.Consensus.RollDPoS.NumSubEpochs = 1
		cfg.Consensus.RollDPoS.EnableDKG = true

		chainAddrs := make([]*iotxaddress.Address, 0, numNodes)
		networkAddrs := make([]net.Addr, 0, numNodes)
		for i := 0; i < numNodes; i++ {
			chainAddrs = append(chainAddrs, newTestAddr())
			networkAddrs = append(networkAddrs, node.NewTCPNode(fmt.Sprintf("127.0.0.%d:4689", i+1)))
		}

		chainRawAddrs := make([]string, 0, numNodes)
		addressMap := make(map[string]*iotxaddress.Address, 0)
		for _, addr := range chainAddrs {
			chainRawAddrs = append(chainRawAddrs, addr.RawAddress)
			addressMap[addr.RawAddress] = addr
		}
		crypto.SortCandidates(chainRawAddrs, 1, crypto.CryptoSeed)
		for i, rawAddress := range chainRawAddrs {
			chainAddrs[i] = addressMap[rawAddress]
		}

		candidatesByHeightFunc := func(_ uint64) ([]*state.Candidate, error) {
			candidates := make([]*state.Candidate, 0, numNodes)
			for _, addr := range chainAddrs {
				candidates = append(candidates, &state.Candidate{Address: addr.RawAddress})
			}
			return candidates, nil
		}

		chains := make([]blockchain.Blockchain, 0, numNodes)
		p2ps := make([]*directOverlay, 0, numNodes)
		cs := make([]*RollDPoS, 0, numNodes)
		for i := 0; i < numNodes; i++ {
			sf, err := state.NewFactory(&cfg, state.InMemTrieOption())
			require.NoError(t, err)
			for j := 0; j < numNodes; j++ {
				ws, err := sf.NewWorkingSet()
				require.NoError(t, err)
				_, err = ws.LoadOrCreateAccountState(chainRawAddrs[j], big.NewInt(0))
				require.NoError(t, err)
				_, err = ws.RunActions(0, nil, nil, nil, nil)
				require.NoError(t, err)
				require.NoError(t, sf.Commit(ws))
			}
			chain := blockchain.NewBlockchain(&cfg, blockchain.InMemDaoOption(), blockchain.PrecreatedStateFactoryOption(sf))
			chains = append(chains, chain)

			actPool, err := actpool.NewActPool(chain, cfg.ActPool)
			require.NoError(t, err)

			p2p := &directOverlay{
				addr:  networkAddrs[i],
				peers: make(map[net.Addr]*RollDPoS),
			}
			p2ps = append(p2ps, p2p)

			consensus, err := NewRollDPoSBuilder().
				SetAddr(chainAddrs[i]).
				SetConfig(cfg.Consensus.RollDPoS).
				SetBlockchain(chain).
				SetActPool(actPool).
				SetP2P(p2p).
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
			require.NoError(t, cs[i].Start(ctx))
		}

		defer func() {
			for i := 0; i < 21; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		assert.NoError(t, testutil.WaitUntil(100*time.Millisecond, 10*time.Second, func() (bool, error) {
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
			require.NoError(t, cs[i].Start(ctx))
		}

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

	checkChains := func(chains []blockchain.Blockchain, height uint64) {
		assert.NoError(t, testutil.WaitUntil(100*time.Millisecond, 15*time.Second, func() (bool, error) {
			for _, chain := range chains {
				blk, err := chain.GetBlockByHeight(height)
				if blk == nil || err != nil {
					return false, nil
				}
				if !blk.IsDummyBlock() {
					return true, errors.New("not a dummy block")
				}
			}
			return true, nil
		}))
	}

	t.Run("proposer-network-partition-dummy-block", func(t *testing.T) {
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
			require.NoError(t, cs[i].Start(ctx))
		}

		defer func() {
			for i := 0; i < 21; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()

		checkChains(chains, 1)
	})

	t.Run("non-proposer-network-partition-dummy-block", func(t *testing.T) {
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
			require.NoError(t, cs[i].Start(ctx))
		}

		defer func() {
			for i := 0; i < 21; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()

		checkChains(chains, 1)
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
			cs[i].ctx.cfg.TimeBasedRotation = true
			cs[i].ctx.cfg.EnableDummyBlock = false
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
			require.NoError(t, cs[i].Start(ctx))
		}

		defer func() {
			for i := 0; i < 21; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()

		checkChains(chains, 4)
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
			cs[i].ctx.cfg.EnableDummyBlock = false
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
			require.NoError(t, cs[i].Start(ctx))
		}

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
			cs[i].ctx.cfg.EnableDummyBlock = false
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
			require.NoError(t, cs[i].Start(ctx))
		}

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
