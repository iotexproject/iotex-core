// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Round-lock tests with enforcement disabled (legacy behaviour), plus the
// shared harness used by roundlock_test.go.
//
// These are internal tests driving the unexported roundCtx / rollDPoSCtx APIs
// with real signed endorsements under mainnet-like conditions:
//   - TimeBasedRotation = true, proposer = proposers[(h+r)%4];
//   - blocks are minted and signed by the round proposer's delegate key and
//     wrapped like real proposals (blockProposal + proposer endorsement);
//   - every node runs CheckBlockProposer and CheckVoteEndorser on receipt;
//   - proposals are delivered via proto serialize -> deserialize.

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
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/poll"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/endorsement"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// rlNumDelegates is the delegate-set size of the harness. With 4 delegates a
// quorum is 3 (3*3 > 2*4).
const rlNumDelegates = 4

// Timing. The base chain commits 50 blocks at g.Timestamp+i; roundInfo therefore
// anchors lastBlockTime at g.Timestamp+40 (49/20*20). With BlockInterval=20s the
// round-0 start is +60s and the round-1 start is +80s. Proposals and clocks are
// pinned to these round starts so that, under TimeBasedRotation, the round-0
// proposer is proposers[51%4]=proposers[3] and the round-1 proposer is
// proposers[52%4]=proposers[0].
const (
	rlGenesisTS = int64(1562382372)
	rlRound0TS  = rlGenesisTS + 60
	rlRound1TS  = rlGenesisTS + 80
)

// makeChain4 builds a real blockchain with exactly rlNumDelegates lifelong
// delegates (identityset keys 0..3) and mines 50 committed blocks, mirroring the
// makeChain helper in roundcalculator_test.go but with a 4-delegate genesis and
// TimeBasedRotation enabled (mainnet default). producerKey fixes the base block
// producer so two separately-built chains are byte-identical (same block hashes
// at every height); a nil key means "generate a fresh one".
func makeChain4(t *testing.T, producerKey crypto.PrivateKey) (blockchain.Blockchain, factory.Factory, actpool.ActPool, *rolldpos.Protocol, genesis.Genesis) {
	require := require.New(t)
	cfg := blockchain.DefaultConfig

	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)
	cfg.TrieDBPath = testTriePath
	cfg.ChainDBPath = testDBPath
	cfg.IndexDBPath = testIndexPath
	t.Cleanup(func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
	})

	g := genesis.TestDefault()
	g.Timestamp = rlGenesisTS
	g.Blockchain.BlockInterval = 20 * time.Second
	g.TimeBasedRotation = true // mainnet default
	// shrink the delegate set to rlNumDelegates
	g.NumDelegates = rlNumDelegates
	g.NumCandidateDelegates = rlNumDelegates
	g.Delegates = nil

	if producerKey == nil {
		producerKey, err = crypto.GenerateKey()
		require.NoError(err)
	}
	cfg.ProducerPrivKey = producerKey.HexString()

	for i := 0; i < identityset.Size(); i++ {
		addr := identityset.Address(i).String()
		value := unit.ConvertIotxToRau(100000000).String()
		g.InitBalanceMap[addr] = value
		if uint64(i) < g.NumDelegates {
			g.Delegates = append(g.Delegates, genesis.Delegate{
				OperatorAddrStr: addr,
				RewardAddrStr:   addr,
				VotesStr:        value,
			})
		}
	}

	registry := protocol.NewRegistry()
	factoryCfg := factory.GenerateConfig(cfg, g)
	db1, err := db.CreateKVStore(db.DefaultConfig, cfg.TrieDBPath)
	require.NoError(err)
	sf, err := factory.NewStateDB(factoryCfg, db1, factory.RegistryStateDBOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(g, sf, actpool.DefaultConfig)
	require.NoError(err)
	dbcfg := db.DefaultConfig
	dbcfg.DbPath = cfg.ChainDBPath
	store, err := filedao.NewFileDAO(dbcfg, block.NewDeserializer(cfg.EVMNetworkID))
	require.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, dbcfg.MaxCacheSize)
	chain := blockchain.NewBlockchain(
		cfg,
		g,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	rp := rolldpos.NewProtocol(g.NumCandidateDelegates, g.NumDelegates, g.NumSubEpochs)
	require.NoError(rp.Register(registry))
	require.NoError(rewarding.NewProtocol(g.Rewarding).Register(registry))
	require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
	require.NoError(poll.NewLifeLongDelegatesProtocol(g.Delegates).Register(registry))
	require.NoError(chain.Start(context.Background()))
	t.Cleanup(func() { chain.Stop(context.Background()) })
	for i := 0; i < 50; i++ {
		blk, err := chain.MintNewBlock(time.Unix(g.Timestamp+int64(i), 0))
		require.NoError(err)
		require.NoError(blk.Finalize(nil, time.Unix(g.Timestamp+int64(i), 0)))
		require.NoError(chain.CommitBlock(blk))
	}
	require.Equal(uint64(50), chain.TipHeight())
	return chain, sf, ap, rp, g
}

// rlNode bundles one node's consensus context with its own chain, a chain
// manager wired to the real minter (so it can mint delegate-signed blocks) and a
// controllable mock clock.
type rlNode struct {
	t     *testing.T
	ctx   *rollDPoSCtx
	chain blockchain.Blockchain
	cm    ChainManager
	clk   *clock.Mock
	deser *block.Deserializer
}

func (n *rlNode) setClock(ts time.Time) { n.clk.Add(ts.Sub(n.clk.Now())) }

func (n *rlNode) prepare(expectHeight uint64, expectRound uint32) {
	require.NoError(n.t, n.ctx.Prepare())
	require.Equal(n.t, expectHeight, n.ctx.round.Height())
	require.Equal(n.t, expectRound, n.ctx.round.Number())
}

// deliverProposal deserializes a proposal that arrived "over the wire", verifies
// its signature and (the realism point) runs the real CheckBlockProposer before
// returning the message for NewProposalEndorsement.
func (n *rlNode) deliverProposal(cmsg *iotextypes.ConsensusMessage) *EndorsedConsensusMessage {
	require := require.New(n.t)
	ecm := &EndorsedConsensusMessage{}
	require.NoError(ecm.LoadProto(cmsg, n.deser))
	require.True(endorsement.VerifyEndorsedDocument(ecm), "proposal signature must verify")
	require.NoError(
		n.ctx.CheckBlockProposer(ecm.Height(), ecm.Document().(*blockProposal), ecm.Endorsement()),
		"CheckBlockProposer must accept a proposal from the legitimate round proposer",
	)
	return ecm
}

// feedVote runs CheckVoteEndorser (as rolldpos.go does on receipt) and then adds
// the endorsement straight into the round. Used for the proposal-endorsement
// (locking) phase.
func (n *rlNode) feedVote(keys []crypto.PrivateKey, idx int, blkHash []byte, topic ConsensusVoteTopic, ts time.Time) {
	ecm := rlEndorse(n.t, keys, idx, blkHash, topic, ts)
	vote := ecm.Document().(*ConsensusVote)
	require.NoError(n.t, n.ctx.CheckVoteEndorser(ecm.Height(), vote, ecm.Endorsement()))
	require.NoError(n.t, n.ctx.round.AddVoteEndorsement(vote, ecm.Endorsement()))
}

// newRLNode builds a node whose key is keys[nodeIdx]. All nodes share the same
// fixed producer key so their base chains (and therefore block hashes) are
// identical. TimeBasedRotation is passed through to match genesis.
func newRLNode(t *testing.T, producerKey crypto.PrivateKey, delegates []string, keys []crypto.PrivateKey, nodeIdx int, opts ...Option) *rlNode {
	return newRLNodeWithDB(t, producerKey, delegates, keys, nodeIdx, "", opts...)
}

// newRLNodeWithDB is newRLNode with the consensus (endorsement manager) DB
// persisted at dbPath; an empty path keeps it in memory.
func newRLNodeWithDB(t *testing.T, producerKey crypto.PrivateKey, delegates []string, keys []crypto.PrivateKey, nodeIdx int, dbPath string, opts ...Option) *rlNode {
	require := require.New(t)
	dbCfg := db.DefaultConfig
	dbCfg.DbPath = dbPath
	b, sf, ap, rp, g := makeChain4(t, producerKey)
	delegatesByEpoch := func(uint64, []byte) ([]string, error) { return delegates, nil }
	clk := clock.NewMock()
	cm := NewChainManager(b, sf, factory.NewMinter(sf, ap))
	rdctx, err := NewRollDPoSCtx(
		consensusfsm.NewConsensusConfig(DefaultConfig.FSM, consensusfsm.DefaultDardanellesUpgradeConfig, consensusfsm.DefaultWakeUpgradeConfig, g, DefaultConfig.Delay),
		dbCfg,
		true,
		time.Second,
		g.TimeBasedRotation, // true
		cm,
		block.NewDeserializer(0),
		rp,
		func(_ proto.Message) error { return nil },
		delegatesByEpoch,
		delegatesByEpoch,
		[]crypto.PrivateKey{keys[nodeIdx]},
		clk,
		g.BeringBlockHeight,
		append([]Option{WithPremintDisabled()}, opts...)...,
	)
	require.NoError(err)
	ctx := rdctx.(*rollDPoSCtx)
	require.NoError(ctx.Start(context.Background()))
	t.Cleanup(func() { ctx.Stop(context.Background()) })
	return &rlNode{t: t, ctx: ctx, chain: b, cm: cm, clk: clk, deser: block.NewDeserializer(0)}
}

// rlEndorse returns an endorsement of (blkHash, topic) by keys[idx] at ts,
// wrapped as a consensus message the ctx methods accept.
func rlEndorse(t *testing.T, keys []crypto.PrivateKey, idx int, blkHash []byte, topic ConsensusVoteTopic, ts time.Time) *EndorsedConsensusMessage {
	t.Helper()
	vote := NewConsensusVote(blkHash, topic)
	ens, err := endorsement.Endorse(vote, ts, keys[idx])
	require.NoError(t, err)
	require.Len(t, ens, 1)
	return NewEndorsedConsensusMessage(51, vote, ens[0])
}

// mintProposal mints a height-51 block signed by proposerKey at ts (the round
// start) on cm, wraps it as a real blockProposal endorsed by the same proposer,
// and serializes it to the wire form. It returns the block hash and the proto.
func mintProposal(t *testing.T, cm ChainManager, prevHash hash.Hash256, proposerKey crypto.PrivateKey, ts time.Time) (hash.Hash256, *iotextypes.ConsensusMessage) {
	t.Helper()
	blk, err := cm.MintNewBlock(ts, proposerKey, prevHash)
	require.NoError(t, err)
	require.Equal(t, uint64(51), blk.Height())
	require.Equal(t, proposerKey.PublicKey().Address().String(), blk.ProducerAddress())
	bp := newBlockProposal(blk, nil)
	ens, err := endorsement.Endorse(bp, ts, proposerKey)
	require.NoError(t, err)
	cmsg, err := NewEndorsedConsensusMessage(51, bp, ens[0]).Proto()
	require.NoError(t, err)
	return blk.HashBlock(), cmsg
}

// TestRoundLockDisabledSingleNode: a node (a non-proposer delegate) locks on A
// in round 0, keeps the lock into round 1, and with enforcement disabled goes
// on to commit a different block B at the same height. A is proposed by the legitimate round-0 proposer, B by the legitimate
// round-1 proposer; the node's CheckBlockProposer accepts both.
//
// It runs with round-lock enforcement DISABLED and documents the legacy
// behaviour; TestRoundLock* in roundlock_test.go cover the enforced behaviour.
func TestRoundLockDisabledSingleNode(t *testing.T) {
	require := require.New(t)

	// delegates 0..3; round-0 proposer = delegate 3, round-1 proposer = delegate 0.
	keys := make([]crypto.PrivateKey, rlNumDelegates)
	delegates := make([]string, rlNumDelegates)
	for i := 0; i < rlNumDelegates; i++ {
		keys[i] = identityset.PrivateKey(i)
		delegates[i] = identityset.Address(i).String()
	}
	const (
		node       = 1 // node under test: an honest, non-proposer delegate
		proposerR0 = 3 // legitimate proposer of A in round 0
		proposerR1 = 0 // legitimate proposer of B in round 1
	)

	n := newRLNode(t, identityset.PrivateKey(25), delegates, keys, node, WithRoundLockEnforcement(false))
	prevHash := n.chain.TipHash()

	round0 := time.Unix(rlRound0TS, 0)
	round1 := time.Unix(rlRound1TS, 0)

	// both proposals are minted up front, while the tip is still 50
	hashA, propA := mintProposal(t, n.cm, prevHash, keys[proposerR0], round0)
	hashB, propB := mintProposal(t, n.cm, prevHash, keys[proposerR1], round1)
	require.NotEqual(hashA, hashB)

	// --- Round 0: receive A from its legitimate proposer, lock on it ----------
	n.setClock(round0)
	n.prepare(51, 0)

	ecmA := n.deliverProposal(propA) // CheckBlockProposer(A) == nil asserted inside
	_, err := n.ctx.NewProposalEndorsement(ecmA)
	require.NoError(err) // node signs PROPOSAL(A) and registers block A

	// three PROPOSAL(A) from delegates {0,2,3} reach the 2/3 quorum -> lock on A
	for _, idx := range []int{0, 2, 3} {
		n.feedVote(keys, idx, hashA[:], PROPOSAL, round0)
	}
	require.True(n.ctx.round.IsLocked(), "node must be locked after 3 PROPOSAL(A)")
	require.Equal(hashA[:], n.ctx.round.HashOfBlockInLock())

	// --- Advance to round 1 at the same height; the lock is carried over ------
	n.setClock(round1)
	n.prepare(51, 1)
	require.True(n.ctx.round.IsLocked(), "lock on A must survive into round 1")
	require.Equal(hashA[:], n.ctx.round.HashOfBlockInLock())

	// Negative control: a block for round 1 signed by a NON-proposer (delegate 2,
	// not the round-1 proposer 0) is rejected by CheckBlockProposer. This proves
	// the guard has teeth, so its acceptance of the real B below is meaningful.
	// (A distinct timestamp inside the same round-1 window avoids the minter's
	// per-(prevHash,timestamp) block cache, which would otherwise return B.)
	_, propBad := mintProposal(t, n.cm, prevHash, keys[2], round1.Add(time.Second))
	badEcm := &EndorsedConsensusMessage{}
	require.NoError(badEcm.LoadProto(propBad, n.deser))
	require.Error(
		n.ctx.CheckBlockProposer(badEcm.Height(), badEcm.Document().(*blockProposal), badEcm.Endorsement()),
		"CheckBlockProposer must reject a proposal from a non-proposer delegate",
	)

	// --- Step 3: node endorses a DIFFERENT, legitimately-proposed block B -----
	ecmB := n.deliverProposal(propB) // CheckBlockProposer(B) == nil asserted inside
	res, err := n.ctx.NewProposalEndorsement(ecmB)
	require.NoError(err, "legacy: locked on A yet NewProposalEndorsement accepts B")
	gotVote := res.([]*EndorsedConsensusMessage)[0].Document().(*ConsensusVote)
	require.Equal(PROPOSAL, gotVote.Topic())
	require.Equal(hashB[:], gotVote.BlockHash(), "legacy: node signed PROPOSAL for B while locked on A")
	require.True(n.ctx.round.IsLocked())
	require.Equal(hashA[:], n.ctx.round.HashOfBlockInLock())

	// --- Step 4: 3 PROPOSAL(B) => node signs LOCK(B) --------------------------
	var lockRes interface{}
	for _, idx := range []int{0, 2, 3} {
		ecm := rlEndorse(t, keys, idx, hashB[:], PROPOSAL, round1)
		require.NoError(n.ctx.CheckVoteEndorser(51, ecm.Document().(*ConsensusVote), ecm.Endorsement()))
		lockRes, err = n.ctx.NewLockEndorsement(ecm)
		require.NoError(err)
	}
	require.NotNil(lockRes, "node must reach LOCK(B) after 3 PROPOSAL(B)")
	require.Equal(LOCK, lockRes.([]*EndorsedConsensusMessage)[0].Document().(*ConsensusVote).Topic())
	require.Equal(hashA[:], n.ctx.round.HashOfBlockInLock(), "legacy: still 'locked' on A while signing LOCK(B)")

	// --- Step 5: 3 LOCK(B) => node signs COMMIT(B) ----------------------------
	var commitRes interface{}
	for _, idx := range []int{0, 2, 3} {
		ecm := rlEndorse(t, keys, idx, hashB[:], LOCK, round1)
		require.NoError(n.ctx.CheckVoteEndorser(51, ecm.Document().(*ConsensusVote), ecm.Endorsement()))
		commitRes, err = n.ctx.NewPreCommitEndorsement(ecm)
		require.NoError(err)
	}
	require.NotNil(commitRes, "node must reach COMMIT(B) after 3 LOCK(B)")
	require.Equal(COMMIT, commitRes.([]*EndorsedConsensusMessage)[0].Document().(*ConsensusVote).Topic())

	// --- Step 6: 3 COMMIT(B) => Commit returns true, chain receives B ---------
	var committed bool
	for _, idx := range []int{0, 2, 3} {
		ecm := rlEndorse(t, keys, idx, hashB[:], COMMIT, round1)
		require.NoError(n.ctx.CheckVoteEndorser(51, ecm.Document().(*ConsensusVote), ecm.Endorsement()))
		committed, err = n.ctx.Commit(ecm)
		require.NoError(err)
		if committed {
			break
		}
	}
	require.True(committed, "legacy: node committed B at height 51 while locked on A")
	require.Equal(uint64(51), n.chain.TipHeight())
	require.Equal(hashB, n.chain.TipHash(), "legacy: committed block is B, not the locked block A")
	require.NotEqual(hashA, n.chain.TipHash())
}

// TestRoundLockDisabledN4: with enforcement disabled and selective message
// delivery by one faulty delegate Z, H1 commits block A at height 51 while H2
// and H3 commit a different block B at the same height.
//
// Role<->key assignment is dictated by TimeBasedRotation: round-0 proposer is
// delegate 3, round-1 proposer is delegate 0. We put a correct node on delegate
// 3 (so A has an honest proposer) and Z on delegate 0 (so Z legitimately
// proposes B in round 1):
//
//	H1 = key3  (round-0 proposer; proposes + commits A)
//	H2 = key1
//	H3 = key2
//	Z  = key0  (round-1 proposer; the only faulty node)
//
// It runs with round-lock enforcement DISABLED and documents the legacy
// behaviour; see TestRoundLockEnforcedN4 for the same scenario enforced.
func TestRoundLockDisabledN4(t *testing.T) {
	require := require.New(t)

	keys := make([]crypto.PrivateKey, rlNumDelegates)
	delegates := make([]string, rlNumDelegates)
	for i := 0; i < rlNumDelegates; i++ {
		keys[i] = identityset.PrivateKey(i)
		delegates[i] = identityset.Address(i).String()
	}
	const (
		h1 = 3 // round-0 proposer, honest
		h2 = 1
		h3 = 2
		z  = 0 // round-1 proposer, faulty
	)
	producer := identityset.PrivateKey(25) // fixed, non-delegate => identical chains

	legacy := WithRoundLockEnforcement(false)
	H1 := newRLNode(t, producer, delegates, keys, h1, legacy)
	H2 := newRLNode(t, producer, delegates, keys, h2, legacy)
	H3 := newRLNode(t, producer, delegates, keys, h3, legacy)
	prevHash := H1.chain.TipHash()

	round0 := time.Unix(rlRound0TS, 0)
	round1 := time.Unix(rlRound1TS, 0)

	// A proposed by the honest round-0 proposer (H1=key3); B by Z (round-1
	// proposer, key0). Both minted up front on H1's chain manager (tip 50) and
	// delivered over the wire; identical chains make the hashes valid everywhere.
	hashA, propA := mintProposal(t, H1.cm, prevHash, keys[h1], round0)
	hashB, propB := mintProposal(t, H1.cm, prevHash, keys[z], round1)
	require.NotEqual(hashA, hashB)

	// ---- Round 0 -------------------------------------------------------------
	for _, n := range []*rlNode{H1, H2, H3} {
		n.setClock(round0)
		n.prepare(51, 0)
		// every node receives A from its legitimate proposer and endorses it
		ecmA := n.deliverProposal(propA) // CheckBlockProposer(A) == nil asserted inside
		_, err := n.ctx.NewProposalEndorsement(ecmA)
		require.NoError(err)
	}

	// H1, H2 observe 3 PROPOSAL(A) from {H1,H2,Z} -> locked on A.
	// H3 is NOT delivered those endorsements -> never locks.
	for _, n := range []*rlNode{H1, H2} {
		for _, idx := range []int{h1, h2, z} {
			n.feedVote(keys, idx, hashA[:], PROPOSAL, round0)
		}
		require.True(n.ctx.round.IsLocked())
		require.Equal(hashA[:], n.ctx.round.HashOfBlockInLock())
	}
	require.False(H3.ctx.round.IsLocked())

	// LOCK(A) from {H1,H2,Z}:
	//  - H1 (locked)   -> signs COMMIT(A)
	//  - H3 (unlocked) -> NewPreCommitEndorsement directly models the post-timeout
	//                     AcceptLockEndorsement state; signs COMMIT(A) with no lock
	var h1CommitA, h3CommitA *EndorsedConsensusMessage
	for _, idx := range []int{h1, h2, z} {
		m1 := rlEndorse(t, keys, idx, hashA[:], LOCK, round0)
		require.NoError(H1.ctx.CheckVoteEndorser(51, m1.Document().(*ConsensusVote), m1.Endorsement()))
		if r, err := H1.ctx.NewPreCommitEndorsement(m1); err == nil && r != nil {
			h1CommitA = r.([]*EndorsedConsensusMessage)[0]
		}
		m3 := rlEndorse(t, keys, idx, hashA[:], LOCK, round0)
		require.NoError(H3.ctx.CheckVoteEndorser(51, m3.Document().(*ConsensusVote), m3.Endorsement()))
		if r, err := H3.ctx.NewPreCommitEndorsement(m3); err == nil && r != nil {
			h3CommitA = r.([]*EndorsedConsensusMessage)[0]
		}
	}
	require.NotNil(h1CommitA, "H1 must reach COMMIT(A)")
	require.NotNil(h3CommitA, "H3 must reach COMMIT(A) despite not being locked")
	require.False(H3.ctx.round.IsLocked(), "H3 signed COMMIT(A) without ever locking")

	// H2 gets LOCK(A) only from {H1,H2} -> never reaches COMMIT(A)
	for _, idx := range []int{h1, h2} {
		m := rlEndorse(t, keys, idx, hashA[:], LOCK, round0)
		require.NoError(H2.ctx.CheckVoteEndorser(51, m.Document().(*ConsensusVote), m.Endorsement()))
		r, err := H2.ctx.NewPreCommitEndorsement(m)
		require.NoError(err)
		require.Nil(r)
	}

	// Z delivers COMMIT(A) only to H1; H1 has COMMIT(A) from {H1,H3,Z} -> commits A
	zCommitA := rlEndorse(t, keys, z, hashA[:], COMMIT, round0)
	var h1Done bool
	for _, m := range []*EndorsedConsensusMessage{h1CommitA, h3CommitA, zCommitA} {
		require.NoError(H1.ctx.CheckVoteEndorser(51, m.Document().(*ConsensusVote), m.Endorsement()))
		var err error
		h1Done, err = H1.ctx.Commit(m)
		require.NoError(err)
	}
	require.True(h1Done, "H1 commits A")
	require.Equal(uint64(51), H1.chain.TipHeight())
	require.Equal(hashA, H1.chain.TipHash())

	// ---- Round 1: Z proposes a different block B -----------------------------
	for _, n := range []*rlNode{H2, H3} {
		n.setClock(round1)
		n.prepare(51, 1)
	}

	// Mitigation check: neither H2 nor H3 has a COMMIT for its blockInLock, so the
	// prepare-phase guard (PreCommitEndorsement -> ReadyToCommit) does NOT fire and
	// both take the proposal path.
	require.Nil(H2.ctx.PreCommitEndorsement(), "H2 has no own COMMIT(A); guard must not fire")
	require.Nil(H3.ctx.PreCommitEndorsement(), "H3 is unlocked; guard must not fire")
	require.True(H2.ctx.round.IsLocked(), "H2 is still (nominally) locked on A in round 1")
	require.Equal(hashA[:], H2.ctx.round.HashOfBlockInLock())

	// Both nodes receive B from its legitimate round-1 proposer (Z). Delivery is
	// per-node (own deserialized block object), so each can Finalize its own copy.
	for _, n := range []*rlNode{H2, H3} {
		ecmB := n.deliverProposal(propB) // CheckBlockProposer(B) == nil asserted inside
		_, err := n.ctx.NewProposalEndorsement(ecmB)
		require.NoError(err)
	}

	// PROPOSAL(B) from {H2,H3,Z} -> LOCK(B); LOCK(B) -> COMMIT(B).
	commitB := map[*rlNode]*EndorsedConsensusMessage{}
	for _, n := range []*rlNode{H2, H3} {
		var lastLock, lastCommit interface{}
		var err error
		for _, idx := range []int{h2, h3, z} {
			m := rlEndorse(t, keys, idx, hashB[:], PROPOSAL, round1)
			require.NoError(n.ctx.CheckVoteEndorser(51, m.Document().(*ConsensusVote), m.Endorsement()))
			lastLock, err = n.ctx.NewLockEndorsement(m)
			require.NoError(err)
		}
		require.NotNil(lastLock, "node must sign LOCK(B)")
		for _, idx := range []int{h2, h3, z} {
			m := rlEndorse(t, keys, idx, hashB[:], LOCK, round1)
			require.NoError(n.ctx.CheckVoteEndorser(51, m.Document().(*ConsensusVote), m.Endorsement()))
			lastCommit, err = n.ctx.NewPreCommitEndorsement(m)
			require.NoError(err)
		}
		require.NotNil(lastCommit, "node must sign COMMIT(B)")
		commitB[n] = lastCommit.([]*EndorsedConsensusMessage)[0]
	}
	// H2 is locked on A yet signed LOCK(B)/COMMIT(B): the lock did not bind it.
	require.Equal(hashA[:], H2.ctx.round.HashOfBlockInLock(), "legacy: H2 'locked' on A but endorsed B")

	// feed COMMIT(B) from {H2,H3,Z} to both H2 and H3 -> they commit B
	zCommitB := rlEndorse(t, keys, z, hashB[:], COMMIT, round1)
	for _, n := range []*rlNode{H2, H3} {
		var done bool
		for _, m := range []*EndorsedConsensusMessage{commitB[H2], commitB[H3], zCommitB} {
			require.NoError(n.ctx.CheckVoteEndorser(51, m.Document().(*ConsensusVote), m.Endorsement()))
			var err error
			done, err = n.ctx.Commit(m)
			require.NoError(err)
		}
		require.True(done)
		require.Equal(uint64(51), n.chain.TipHeight())
		require.Equal(hashB, n.chain.TipHash())
	}

	// ---- Diverging tips --------------------------------------------------------
	require.Equal(hashA, H1.chain.TipHash(), "H1 committed A at height 51")
	require.Equal(hashB, H2.chain.TipHash(), "H2 committed B at height 51")
	require.Equal(hashB, H3.chain.TipHash(), "H3 committed B at height 51")
	require.NotEqual(H1.chain.TipHash(), H2.chain.TipHash(), "legacy: tips diverge at height 51")
}

var _ = errors.New
