// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// guardFork is the height at which the bucket guard activates in these tests.
const guardFork = 5

var (
	guardAlg         = pqcrypto.AlgSLHDSA192s
	guardEventTopic  = crypto.Keccak256([]byte("BucketGuard(uint64,uint64,bytes32,uint64,uint64,address,address)"))
	guardRegAmount   = unit.ConvertIotxToRau(1_200_000)
	guardStakeAmount = unit.ConvertIotxToRau(10_000)
)

// guardPayload is the subset of the action payload contract the builder needs.
type guardPayload interface {
	IntrinsicGas() (uint64, error)
	SanityCheck() error
	FillAction(*iotextypes.ActionCore)
}

// guardFixture is a chain with the bucket guard fork at guardFork, plus
// helpers to build signed actions and expectations.
type guardFixture struct {
	test      *e2etest
	chainID   uint32
	domain    bucketguard.Domain // native chain ID and genesis hash, as the transcripts require
	gasLimit  uint64
	gasPrice  *big.Int
	start     time.Time
	blockTime time.Time
	// pending are actions signed when their test case runs, so they can
	// anchor to the chain's tip; see signedLater.
	pending map[*actionWithTime]pendingAct
}

// pendingAct is an action whose nonce and block time are reserved when the
// test cases are built, and which is signed when its case runs.
type pendingAct struct {
	id    int
	nonce uint64
	build func() guardPayload
}

func newGuardFixture(t *testing.T, opts ...func(*config.Config)) *guardFixture {
	cfg := initCfg(require.New(t))
	cfg.Genesis.ToBeEnabledBlockHeight = guardFork
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	for _, opt := range opts {
		opt(&cfg)
	}
	test := newE2ETest(t, cfg)
	now := time.Now()
	return &guardFixture{
		test:      test,
		chainID:   test.cfg.Chain.ID,
		domain:    bucketguard.NewDomain(test.cfg.Chain.ID, test.cfg.Genesis.Hash()),
		gasLimit:  4_000_000,
		gasPrice:  big.NewInt(unit.Qev*2 + 1),
		start:     now,
		blockTime: now,
		pending:   map[*actionWithTime]pendingAct{},
	}
}

// run runs the cases, signing each pending action just before its case.
func (f *guardFixture) run(cases []*testcase) {
	for _, c := range cases {
		p, ok := f.pending[c.act]
		if !ok {
			continue
		}
		a, pre := c.act, c.preFunc
		c.preFunc = func(e *e2etest) {
			if pre != nil {
				pre(e)
			}
			a.act = f.sign(p.id, p.nonce, p.build())
		}
	}
	f.test.run(cases)
}

func guardKey(fill byte) *signer.PrivateKey {
	return mustNoErr(signer.NewKeyFromSeeds(guardAlg, bytes.Repeat([]byte{fill}, signer.SeedSize(guardAlg))))
}

// tick gives every block its own, increasing timestamp: an unstake is only
// seen as such when it is strictly later than the stake start.
func (f *guardFixture) tick() time.Time {
	f.blockTime = f.blockTime.Add(time.Second)
	return f.blockTime
}

func (f *guardFixture) nonce(id int) uint64 {
	return f.test.nonceMgr.pop(identityset.Address(id).String())
}

// at wraps an action signed by one of the action.Signed* helpers.
func (f *guardFixture) at(se *action.SealedEnvelope, err error) *actionWithTime {
	return &actionWithTime{mustNoErr(se, err), f.tick()}
}

func (f *guardFixture) sign(id int, nonce uint64, p guardPayload) *action.SealedEnvelope {
	elp := (&action.EnvelopeBuilder{}).SetNonce(nonce).
		SetGasLimit(f.gasLimit).SetGasPrice(f.gasPrice).SetChainID(f.chainID).SetAction(p).Build()
	return mustNoErr(action.Sign(elp, identityset.PrivateKey(id)))
}

func (f *guardFixture) signed(id int, p guardPayload) *actionWithTime {
	return &actionWithTime{f.sign(id, f.nonce(id), p), f.tick()}
}

// signedLater is signed for id when its case runs (f.run), from the payload
// build returns then. Guarded ops use it to anchor to the tip at that time.
func (f *guardFixture) signedLater(id int, build func() guardPayload) *actionWithTime {
	a := &actionWithTime{nil, f.tick()}
	f.pending[a] = pendingAct{id: id, nonce: f.nonce(id), build: build}
	return a
}

// anchor returns the tip height and hash, the anchor a wallet reads.
func (f *guardFixture) anchor() (uint64, []byte) {
	h := f.test.cs.Blockchain().TipHeight()
	return h, f.blockHash(h)
}

func (f *guardFixture) blockHash(height uint64) []byte {
	hash := mustNoErr(f.test.cs.Blockchain().BlockHeaderByHeight(height)).HashBlock()
	return hash[:]
}

func (f *guardFixture) register(id int, name string) *actionWithTime {
	a := identityset.Address(id).String()
	return f.at(action.SignedCandidateRegister(f.nonce(id), name, a, a, a, guardRegAmount.String(), 1, true, nil,
		f.gasLimit, f.gasPrice, identityset.PrivateKey(id), action.WithChainID(f.chainID)))
}

func (f *guardFixture) createStake(id int, cand string, amount *big.Int) *actionWithTime {
	return f.at(action.SignedCreateStake(f.nonce(id), cand, amount.String(), 0, false, nil,
		f.gasLimit, f.gasPrice, identityset.PrivateKey(id), action.WithChainID(f.chainID)))
}

func (f *guardFixture) bind(index uint64, k *signer.PrivateKey, newGen uint64, owner address.Address) *action.BindBucketGuard {
	pub := k.PublicKey().Bytes()
	acc := mustNoErr(k.Sign([]byte(bucketguard.AcceptContext), bucketguard.EncodeAccept(f.domain, index, newGen, owner, guardAlg, pub, [bucketguard.CallHashLen]byte{})))
	return action.NewBindBucketGuard(&bucketguard.BindBucketGuard{BucketIndex: index, AlgID: guardAlg, PublicKey: pub, AcceptSig: acc})
}

// op returns an op anchored to the current tip, valid for 1000 blocks.
func (f *guardFixture) op(index uint64, o bucketguard.Op, gen, nonce uint64) *bucketguard.GuardedBucketOp {
	h, hash := f.anchor()
	return &bucketguard.GuardedBucketOp{BucketIndex: index, Generation: gen, Nonce: nonce,
		Deadline: h + 1000, AnchorHeight: h, AnchorHash: hash, Op: o}
}

// newGuard attaches next as the incoming key, accepting op (otherwise
// complete, on a bucket owned by owner) for resultingOwner.
func (f *guardFixture) newGuard(op *bucketguard.GuardedBucketOp, owner address.Address, next *signer.PrivateKey, resultingOwner address.Address) {
	pub := next.PublicKey().Bytes()
	op.NewGuard = &bucketguard.NewGuard{AlgID: guardAlg, PublicKey: pub}
	h := bucketguard.CallHash(f.domain, owner, op)
	op.NewGuard.AcceptSig = mustNoErr(next.Sign([]byte(bucketguard.AcceptContext),
		bucketguard.EncodeAccept(f.domain, op.BucketIndex, op.Generation+1, resultingOwner, guardAlg, pub, h)))
}

// guarded signs op's CALL with k for a bucket owned by owner at execution time.
func (f *guardFixture) guarded(k *signer.PrivateKey, owner address.Address, op *bucketguard.GuardedBucketOp) *action.GuardedBucketOp {
	op.Sig = mustNoErr(k.Sign([]byte(bucketguard.CallContext), bucketguard.EncodeCall(f.domain, owner, op)))
	return action.NewGuardedBucketOp(op)
}

func (f *guardFixture) balance(id int) *big.Int {
	ctx := genesis.WithGenesisContext(context.Background(), f.test.cfg.Genesis)
	acc, err := accountutil.AccountState(ctx, f.test.cs.StateFactory(), identityset.Address(id))
	require.NoError(f.test.t, err)
	return acc.Balance
}

// readGuard reads a bucket's guard through the ReadStakingData API; height ""
// is the tip.
func (f *guardFixture) readGuard(index uint64, height string) *iotextypes.BucketGuard {
	method := mustNoErr(proto.Marshal(&iotexapi.ReadStakingDataMethod{Method: iotexapi.ReadStakingDataMethod_BUCKET_GUARD}))
	arg := mustNoErr(proto.Marshal(&iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketGuard_{BucketGuard: &iotexapi.ReadStakingDataRequest_BucketGuard{Index: index}},
	}))
	resp, err := f.test.api.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("staking"), MethodName: method, Arguments: [][]byte{arg}, Height: height,
	})
	require.NoError(f.test.t, err)
	g := &iotextypes.BucketGuard{}
	require.NoError(f.test.t, proto.Unmarshal(resp.GetData(), g))
	return g
}

// callGuard reads a bucket's guard through eth_call bucketGuard(uint64) on the
// staking address at block height (nil: latest).
func (f *guardFixture) callGuard(index uint64, height *big.Int) []interface{} {
	m := mustNoErr(abi.JSON(strings.NewReader(`[{"inputs":[{"name":"index","type":"uint64"}],"name":"bucketGuard","outputs":[{"name":"guarded","type":"bool"},{"name":"algID","type":"uint32"},{"name":"publicKey","type":"bytes"},{"name":"generation","type":"uint64"},{"name":"nonce","type":"uint64"},{"name":"boundHeight","type":"uint64"}],"stateMutability":"view","type":"function"}]`))).Methods["bucketGuard"]
	stakingAddr := common.BytesToAddress(address.StakingProtocolAddrHash[:])
	data := append(append([]byte{}, m.ID...), mustNoErr(m.Inputs.Pack(index))...)
	out, err := f.test.ethcli.CallContract(context.Background(), ethereum.CallMsg{To: &stakingAddr, Data: data}, height)
	require.NoError(f.test.t, err)
	return mustNoErr(m.Outputs.Unpack(out))
}

func receiptStatus(s iotextypes.ReceiptStatus) actionExpect {
	return &basicActionExpect{nil, uint64(s), ""}
}

func expectFn(fn func(test *e2etest, receipt *action.Receipt)) actionExpect {
	return &functionExpect{func(test *e2etest, _ *action.SealedEnvelope, receipt *action.Receipt, _ error) {
		fn(test, receipt)
	}}
}

// guardIs checks the stored record; key == nil means a tombstone.
func guardIs(index, gen, nonce uint64, key *signer.PrivateKey) actionExpect {
	return expectFn(func(test *e2etest, _ *action.Receipt) {
		g, err := staking.ReadBucketGuard(test.cs.StateFactory(), index)
		require.NoError(test.t, err)
		require.NotNil(test.t, g, "guard record")
		require.Equal(test.t, gen, g.Generation, "generation")
		require.Equal(test.t, nonce, g.Nonce, "nonce")
		if key == nil {
			require.False(test.t, g.IsGuarded(), "tombstone")
			return
		}
		require.Equal(test.t, key.PublicKey().Bytes(), g.PublicKey, "guard key")
	})
}

func guardAbsent(index uint64) actionExpect {
	return expectFn(func(test *e2etest, _ *action.Receipt) {
		g, err := staking.ReadBucketGuard(test.cs.StateFactory(), index)
		require.NoError(test.t, err)
		require.Nil(test.t, g, "guard record")
	})
}

// guardEventIs checks the single IIP §11 event of a successful bind or op;
// key is the bucket's guard key after it, nil for none.
func guardEventIs(index uint64, op uint32, key *signer.PrivateKey, gen, nonce uint64, sponsor, counterparty address.Address) actionExpect {
	return expectFn(func(test *e2etest, receipt *action.Receipt) {
		require.Len(test.t, receipt.Logs(), 1, "exactly one event, no legacy logs")
		l := receipt.Logs()[0]
		require.Len(test.t, l.Topics, 4)
		require.Equal(test.t, guardEventTopic, l.Topics[0][:])
		require.Equal(test.t, index, binary.BigEndian.Uint64(l.Topics[1][24:]))
		require.Equal(test.t, uint64(op), binary.BigEndian.Uint64(l.Topics[2][24:]))
		// guardKeyID = SHA-256(uint32_be(algID) || publicKey), zero without a key
		var keyID [32]byte
		if key != nil {
			keyID = sha256.Sum256(append(binary.BigEndian.AppendUint32(nil, uint32(guardAlg)), key.PublicKey().Bytes()...))
		}
		require.Equal(test.t, keyID[:], l.Topics[3][:], "guardKeyID")
		// abi.encode(uint64 generation, uint64 nonce, address sponsor, address counterparty)
		var want []byte
		word := func(b []byte) { want = append(append(want, make([]byte, 32-len(b))...), b...) }
		word(binary.BigEndian.AppendUint64(nil, gen))
		word(binary.BigEndian.AppendUint64(nil, nonce))
		word(sponsor.Bytes())
		if counterparty != nil {
			word(counterparty.Bytes())
		} else {
			word(nil)
		}
		require.Equal(test.t, want, l.Data, "event data")
	})
}

func bucketIs(index uint64, check func(*testing.T, *iotextypes.VoteBucket)) actionExpect {
	return expectFn(func(test *e2etest, _ *action.Receipt) {
		b, err := test.getBucket(index, "")
		require.NoError(test.t, err)
		check(test.t, b)
	})
}

// TestBucketGuard drives one native bucket through the lifecycle: rejection
// before the fork, bind, scope and legacy-path rejections, guarded ops
// submitted by a sponsor that is not the owner, anchor and sponsor binding,
// replay and wrong-key failures with state rollback, key rotation, unstake
// and withdrawal to a signed recipient.
func TestBucketGuard(t *testing.T) {
	f := newGuardFixture(t)
	defer f.test.teardown()
	var (
		ownerID, sponsorID, recipientID, otherID = 3, 4, 5, 6
		owner                                    = identityset.Address(ownerID)
		sponsor                                  = identityset.Address(sponsorID)
		cand2ID                                  = identityset.Address(2) // candidates are identified by owner
		keyA, keyB                               = guardKey(0xa), guardKey(0xb)
		recipientStart                           *big.Int
	)
	const (
		selfStakeBucket = 0 // cand1's self-stake
		bucket          = 2
		plainBucket     = 3
	)
	// invalid is an op refused by the pool's stateless checks; its sender's
	// nonce is reused.
	invalid := func(id int, m *bucketguard.GuardedBucketOp) *actionWithTime {
		m.Sig = make([]byte, guardAlg.SignatureSize())
		a := f.signed(id, action.NewGuardedBucketOp(m))
		f.test.nonceMgr[identityset.Address(id).String()]--
		return a
	}
	anchored := func(m *bucketguard.GuardedBucketOp) *bucketguard.GuardedBucketOp {
		m.AnchorHeight, m.AnchorHash, m.Deadline = 1, bytes.Repeat([]byte{1}, 32), 100
		return m
	}
	f.run([]*testcase{
		{
			name:    "setup: two candidates and an owner bucket",
			preActs: []*actionWithTime{f.register(1, "cand1"), f.register(2, "cand2"), f.createStake(ownerID, "cand1", guardStakeAmount)},
		},
		{
			name: "before the fork the action is invalid",
			act: func() *actionWithTime {
				a := f.signed(ownerID, f.bind(bucket, keyA, 1, owner))
				f.test.nonceMgr[owner.String()]-- // rejected by the pool, so its nonce is reused
				return a
			}(),
			expect: []actionExpect{&basicActionExpect{action.ErrInvalidAct, 0, ""}},
		},
		{
			name: "last pre-fork block",
			act:  f.createStake(ownerID, "cand1", guardStakeAmount),
			expect: []actionExpect{successExpect, expectFn(func(test *e2etest, _ *action.Receipt) {
				require.EqualValues(test.t, guardFork-1, test.cs.Blockchain().TipHeight())
			})},
		},
		{
			name:   "owner binds key A at the fork height",
			act:    f.signed(ownerID, f.bind(bucket, keyA, 1, owner)),
			expect: []actionExpect{successExpect, guardIs(bucket, 1, 0, keyA), guardEventIs(bucket, 0, keyA, 1, 0, owner, nil)},
		},
		{
			name:   "a self-stake bucket cannot be guarded",
			act:    f.signed(1, f.bind(selfStakeBucket, keyA, 1, identityset.Address(1))),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrGuardScope), guardAbsent(selfStakeBucket)},
		},
		{
			name:   "legacy ChangeCandidate is rejected",
			act:    f.at(action.SignedChangeCandidate(f.nonce(ownerID), "cand2", bucket, nil, f.gasLimit, f.gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(f.chainID))),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrBucketGuarded)},
		},
		{
			name:   "legacy Unstake is rejected",
			act:    f.at(action.SignedReclaimStake(false, f.nonce(ownerID), bucket, nil, f.gasLimit, f.gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(f.chainID))),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrBucketGuarded)},
		},
		{
			name:   "legacy TransferStake is rejected",
			act:    f.at(action.SignedTransferStake(f.nonce(ownerID), sponsor.String(), bucket, nil, f.gasLimit, f.gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(f.chainID))),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrBucketGuarded)},
		},
		{
			name:   "legacy Restake on an unguarded bucket still works",
			act:    f.at(action.SignedRestake(f.nonce(ownerID), plainBucket, 1, false, nil, f.gasLimit, f.gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(f.chainID))),
			expect: []actionExpect{successExpect},
		},
		{
			name: "a deadline beyond the anchor window is invalid",
			act: invalid(sponsorID, func() *bucketguard.GuardedBucketOp {
				m := anchored(&bucketguard.GuardedBucketOp{BucketIndex: bucket, Generation: 1, Op: bucketguard.OpUnstake})
				m.Deadline = m.AnchorHeight + bucketguard.ParamsV1().MaxDeadlineWindow + 1
				return m
			}()),
			expect: []actionExpect{&basicActionExpect{action.ErrInvalidAct, 0, ""}},
		},
		{
			name: "a zero recipient is invalid",
			act: invalid(sponsorID, anchored(&bucketguard.GuardedBucketOp{BucketIndex: bucket, Generation: 1, Op: bucketguard.OpWithdraw,
				Recipient: make([]byte, 20)})),
			expect: []actionExpect{&basicActionExpect{action.ErrInvalidAct, 0, ""}},
		},
		{
			name: "a zero sponsor is invalid",
			act: invalid(sponsorID, anchored(&bucketguard.GuardedBucketOp{BucketIndex: bucket, Generation: 1, Op: bucketguard.OpUnstake,
				Sponsor: make([]byte, 20)})),
			expect: []actionExpect{&basicActionExpect{action.ErrInvalidAct, 0, ""}},
		},
		{
			name: "an op anchored to a wrong block hash is refused",
			act: f.signedLater(sponsorID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpRestake, 1, 0)
				m.Duration = 30
				m.AnchorHash = bytes.Repeat([]byte{0xee}, 32)
				return f.guarded(keyA, owner, m)
			}),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrGuardMismatch), guardIs(bucket, 1, 0, keyA)},
		},
		{
			name: "an op anchored to an older block with the hash of the tip is refused",
			act: f.signedLater(sponsorID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpRestake, 1, 0)
				m.Duration = 30
				m.AnchorHeight--
				return f.guarded(keyA, owner, m)
			}),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrGuardMismatch), guardIs(bucket, 1, 0, keyA)},
		},
		{
			name: "an op whose short window after an old anchor has passed is refused",
			act: f.signedLater(sponsorID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpRestake, 1, 0)
				m.Duration = 30
				m.AnchorHeight = 2
				m.AnchorHash = f.blockHash(2)
				m.Deadline = 3
				return f.guarded(keyA, owner, m)
			}),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrGuardMismatch), guardIs(bucket, 1, 0, keyA)},
		},
		{
			name: "an op bound to the sponsor is refused from another sender",
			act: f.signedLater(otherID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpChangeCandidate, 1, 0)
				m.CandidateName, m.ExpectedCandidateID, m.Sponsor = "cand2", cand2ID.Bytes(), sponsor.Bytes()
				return f.guarded(keyA, owner, m)
			}),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrUnauthorizedOperator), guardIs(bucket, 1, 0, keyA)},
		},
		{
			name: "the bound sponsor submits a guarded CHANGE_CANDIDATE signed by key A",
			act: f.signedLater(sponsorID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpChangeCandidate, 1, 0)
				m.CandidateName, m.ExpectedCandidateID, m.Sponsor = "cand2", cand2ID.Bytes(), sponsor.Bytes()
				return f.guarded(keyA, owner, m)
			}),
			expect: []actionExpect{successExpect, guardIs(bucket, 1, 1, keyA), guardEventIs(bucket, uint32(bucketguard.OpChangeCandidate), keyA, 1, 1, sponsor, nil),
				bucketIs(bucket, func(t *testing.T, b *iotextypes.VoteBucket) {
					require.Equal(t, cand2ID.String(), b.CandidateAddress)
				})},
		},
		{
			name: "replaying the same signed op fails on the counters",
			act: f.signedLater(sponsorID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpChangeCandidate, 1, 0)
				m.CandidateName, m.ExpectedCandidateID = "cand2", cand2ID.Bytes()
				return f.guarded(keyA, owner, m)
			}),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrGuardMismatch), guardIs(bucket, 1, 1, keyA)},
		},
		{
			name: "an op signed for another chain instance with the same chain ID is refused",
			act: f.signedLater(sponsorID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpRestake, 1, 1)
				m.Duration = 30
				other := bucketguard.NewDomain(f.chainID, [32]byte{0xff})
				m.Sig = mustNoErr(keyA.Sign([]byte(bucketguard.CallContext), bucketguard.EncodeCall(other, owner, m)))
				return action.NewGuardedBucketOp(m)
			}),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrGuardSignature), guardIs(bucket, 1, 1, keyA)},
		},
		{
			name: "a key that is not the guard is refused and the restake is rolled back",
			act: f.signedLater(sponsorID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpRestake, 1, 1)
				m.Duration = 30
				return f.guarded(keyB, owner, m)
			}),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrGuardSignature), guardIs(bucket, 1, 1, keyA),
				bucketIs(bucket, func(t *testing.T, b *iotextypes.VoteBucket) {
					require.EqualValues(t, 0, b.StakedDuration, "restake effect must be reverted")
				})},
		},
		{
			name: "ROTATE from key A to key B",
			act: f.signedLater(sponsorID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpRotate, 1, 1)
				f.newGuard(m, owner, keyB, owner)
				return f.guarded(keyA, owner, m)
			}),
			expect: []actionExpect{successExpect, guardIs(bucket, 2, 2, keyB), guardEventIs(bucket, uint32(bucketguard.OpRotate), keyB, 2, 2, sponsor, nil)},
		},
		{
			name: "UNSTAKE with key B",
			act: f.signedLater(sponsorID, func() guardPayload {
				return f.guarded(keyB, owner, f.op(bucket, bucketguard.OpUnstake, 2, 2))
			}),
			expect: []actionExpect{successExpect, guardIs(bucket, 2, 3, keyB), guardEventIs(bucket, uint32(bucketguard.OpUnstake), keyB, 2, 3, sponsor, nil)},
		},
		{
			name:    "WITHDRAW pays the signed recipient and deletes the guard",
			preFunc: func(*e2etest) { recipientStart = f.balance(recipientID) },
			act: func() *actionWithTime {
				a := f.signedLater(sponsorID, func() guardPayload {
					m := f.op(bucket, bucketguard.OpWithdraw, 2, 3)
					m.Recipient = identityset.Address(recipientID).Bytes()
					return f.guarded(keyB, owner, m)
				})
				a.t = f.start.Add(f.test.cfg.Genesis.WithdrawWaitingPeriod + 24*time.Hour)
				return a
			}(),
			expect: []actionExpect{successExpect, guardAbsent(bucket),
				guardEventIs(bucket, uint32(bucketguard.OpWithdraw), nil, 2, 4, sponsor, identityset.Address(recipientID)),
				bucketIs(bucket, func(t *testing.T, b *iotextypes.VoteBucket) { require.Nil(t, b, "bucket withdrawn") }),
				expectFn(func(test *e2etest, _ *action.Receipt) {
					got := new(big.Int).Sub(f.balance(recipientID), recipientStart)
					require.Equal(test.t, guardStakeAmount.String(), got.String(), "recipient credited, gas paid by the sponsor")
				})},
		},
	})
}

// TestBucketGuardTransferAndClear covers the scope rules against endorsement
// and self-stake activation, a transfer that installs the new owner's key,
// the TRANSFER newOwner rules, a transfer that clears the guard, and the
// legacy paths working again on the tombstone until a withdrawal deletes it.
func TestBucketGuardTransferAndClear(t *testing.T) {
	f := newGuardFixture(t)
	defer f.test.teardown()
	var (
		ownerID, newOwnerID = 3, 6
		owner               = identityset.Address(ownerID)
		newOwner            = identityset.Address(newOwnerID)
		keyA, keyC          = guardKey(0xa), guardKey(0xc)
	)
	const (
		bucket         = 1 // owner's bucket; cand1's self-stake is 0
		endorsedBucket = 2
	)
	stakingAddr := mustNoErr(address.FromString(address.StakingProtocolAddr))
	f.run([]*testcase{
		{
			name: "setup: candidate, a bucket, and an endorsed bucket",
			preActs: []*actionWithTime{
				f.register(1, "cand1"),
				f.createStake(ownerID, "cand1", guardStakeAmount),
				f.createStake(ownerID, "cand1", guardRegAmount),
			},
			act: f.at(action.SignedCandidateEndorsement(f.nonce(ownerID), endorsedBucket, action.CandidateEndorsementOpEndorse, f.gasLimit, f.gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(f.chainID))),
			expect: []actionExpect{successExpect, expectFn(func(test *e2etest, _ *action.Receipt) {
				require.EqualValues(test.t, guardFork-1, test.cs.Blockchain().TipHeight())
			})},
		},
		{
			name:   "bind key A",
			act:    f.signed(ownerID, f.bind(bucket, keyA, 1, owner)),
			expect: []actionExpect{successExpect, guardIs(bucket, 1, 0, keyA)},
		},
		{
			name:   "an endorsed bucket cannot be guarded",
			act:    f.signed(ownerID, f.bind(endorsedBucket, keyA, 1, owner)),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrGuardScope), guardAbsent(endorsedBucket)},
		},
		{
			name:   "a guarded bucket cannot be endorsed",
			act:    f.at(action.SignedCandidateEndorsement(f.nonce(ownerID), bucket, action.CandidateEndorsementOpEndorse, f.gasLimit, f.gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(f.chainID))),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrBucketGuarded)},
		},
		{
			name:   "a guarded bucket cannot become a self-stake bucket",
			act:    f.at(action.SignedCandidateActivate(f.nonce(1), bucket, f.gasLimit, f.gasPrice, identityset.PrivateKey(1), action.WithChainID(f.chainID))),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrBucketGuarded)},
		},
		{
			name: "TRANSFER to the current owner is refused",
			act: f.signedLater(newOwnerID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpTransfer, 1, 0)
				m.NewOwner, m.ClearGuard = owner.Bytes(), true
				return f.guarded(keyA, owner, m)
			}),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrGuardScope), guardIs(bucket, 1, 0, keyA)},
		},
		{
			name: "TRANSFER with clearGuard to the staking protocol is refused",
			act: f.signedLater(newOwnerID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpTransfer, 1, 0)
				m.NewOwner, m.ClearGuard = stakingAddr.Bytes(), true
				return f.guarded(keyA, owner, m)
			}),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrGuardScope), guardIs(bucket, 1, 0, keyA)},
		},
		{
			name: "TRANSFER to the new owner installs the new owner's key C",
			act: f.signedLater(newOwnerID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpTransfer, 1, 0)
				m.NewOwner = newOwner.Bytes()
				f.newGuard(m, owner, keyC, newOwner)
				return f.guarded(keyA, owner, m)
			}),
			expect: []actionExpect{successExpect, guardIs(bucket, 2, 1, keyC),
				guardEventIs(bucket, uint32(bucketguard.OpTransfer), keyC, 2, 1, newOwner, newOwner),
				bucketIs(bucket, func(t *testing.T, b *iotextypes.VoteBucket) {
					require.Equal(t, newOwner.String(), b.Owner)
				})},
		},
		{
			name: "TRANSFER back with clearGuard leaves a tombstone",
			act: f.signedLater(newOwnerID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpTransfer, 2, 1)
				m.NewOwner, m.ClearGuard = owner.Bytes(), true
				return f.guarded(keyC, newOwner, m)
			}),
			expect: []actionExpect{successExpect, guardIs(bucket, 3, 2, nil),
				guardEventIs(bucket, uint32(bucketguard.OpTransfer), nil, 3, 2, newOwner, owner),
				bucketIs(bucket, func(t *testing.T, b *iotextypes.VoteBucket) {
					require.Equal(t, owner.String(), b.Owner)
				})},
		},
		{
			name:   "legacy Unstake works on the cleared bucket",
			act:    f.at(action.SignedReclaimStake(false, f.nonce(ownerID), bucket, nil, f.gasLimit, f.gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(f.chainID))),
			expect: []actionExpect{successExpect, guardIs(bucket, 3, 2, nil)},
		},
		{
			name: "legacy WithdrawStake deletes the tombstone with the bucket",
			act: func() *actionWithTime {
				a := f.at(action.SignedReclaimStake(true, f.nonce(ownerID), bucket, nil, f.gasLimit, f.gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(f.chainID)))
				a.t = f.start.Add(f.test.cfg.Genesis.WithdrawWaitingPeriod + 24*time.Hour)
				return a
			}(),
			expect: []actionExpect{successExpect, guardAbsent(bucket)},
		},
	})
}

// TestBucketGuardErigon runs a guarded bucket through bind, restake, rotate,
// unstake and withdraw on the Erigon dual-storage backend, then reads the
// guard record back at every height through both the ReadStakingData API and
// eth_call, which serve historical heights from the Erigon archive.
func TestBucketGuardErigon(t *testing.T) {
	historyIndexPath, err := os.MkdirTemp("", "historyindex")
	require.NoError(t, err)
	defer os.RemoveAll(historyIndexPath)
	f := newGuardFixture(t, func(cfg *config.Config) {
		cfg.Chain.HistoryIndexPath = historyIndexPath
		cfg.API.GRPCPort = testutil.RandomPort()
		cfg.API.HTTPPort = testutil.RandomPort()
		cfg.API.WebSocketPort = 0
		cfg.Plugins[config.GatewayPlugin] = true
	})
	defer f.test.teardown()
	var (
		ownerID, sponsorID = 3, 4
		owner              = identityset.Address(ownerID)
		keyA, keyB         = guardKey(0xa), guardKey(0xb)
	)
	const bucket = 1
	type want struct {
		key               *signer.PrivateKey
		gen, nonce, bound uint64
	}
	// state after each block, by height; the zero value means no record
	history := map[uint64]want{
		guardFork:     {keyA, 1, 0, guardFork},
		guardFork + 1: {keyA, 1, 1, guardFork},
		guardFork + 2: {keyB, 2, 2, guardFork + 2},
		guardFork + 3: {keyB, 2, 3, guardFork + 2},
	}
	f.run([]*testcase{
		{
			name:    "setup",
			preActs: []*actionWithTime{f.register(1, "cand1"), f.createStake(ownerID, "cand1", guardStakeAmount), f.createStake(ownerID, "cand1", guardStakeAmount)},
			act:     f.createStake(ownerID, "cand1", guardStakeAmount),
			expect:  []actionExpect{successExpect},
		},
		{name: "bind", act: f.signed(ownerID, f.bind(bucket, keyA, 1, owner)), expect: []actionExpect{successExpect}},
		{
			name: "restake",
			act: f.signedLater(sponsorID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpRestake, 1, 0)
				m.Duration = 7
				return f.guarded(keyA, owner, m)
			}),
			expect: []actionExpect{successExpect},
		},
		{
			name: "rotate",
			act: f.signedLater(sponsorID, func() guardPayload {
				m := f.op(bucket, bucketguard.OpRotate, 1, 1)
				f.newGuard(m, owner, keyB, owner)
				return f.guarded(keyA, owner, m)
			}),
			expect: []actionExpect{successExpect},
		},
		{
			name: "unstake",
			act: func() *actionWithTime {
				a := f.signedLater(sponsorID, func() guardPayload {
					return f.guarded(keyB, owner, f.op(bucket, bucketguard.OpUnstake, 2, 2))
				})
				// the restake set a 7-day duration starting at its block
				a.t = f.start.Add(8 * 24 * time.Hour)
				return a
			}(),
			expect: []actionExpect{successExpect},
		},
		{
			name: "withdraw, then read every height back",
			act: func() *actionWithTime {
				a := f.signedLater(sponsorID, func() guardPayload {
					m := f.op(bucket, bucketguard.OpWithdraw, 2, 3)
					m.Recipient = identityset.Address(5).Bytes()
					return f.guarded(keyB, owner, m)
				})
				a.t = f.start.Add(8*24*time.Hour + f.test.cfg.Genesis.WithdrawWaitingPeriod + time.Hour)
				return a
			}(),
			expect: []actionExpect{successExpect, expectFn(func(test *e2etest, _ *action.Receipt) {
				tip := test.cs.Blockchain().TipHeight()
				require.EqualValues(test.t, guardFork+4, tip)
				for h := uint64(guardFork - 1); h <= tip; h++ {
					w := history[h]
					g := f.readGuard(bucket, strconv.FormatUint(h, 10))
					vals := f.callGuard(bucket, new(big.Int).SetUint64(h))
					if w.key == nil {
						require.Empty(test.t, g.GetPublicKey(), "height %d: API", h)
						require.Zero(test.t, g.GetGeneration(), "height %d: API", h)
						require.Equal(test.t, false, vals[0], "height %d: eth_call guarded", h)
						continue
					}
					pk := w.key.PublicKey().Bytes()
					require.Equal(test.t, pk, g.GetPublicKey(), "height %d: API key", h)
					require.Equal(test.t, []uint64{w.gen, w.nonce, w.bound}, []uint64{g.GetGeneration(), g.GetNonce(), g.GetBoundHeight()}, "height %d: API counters", h)
					require.Equal(test.t, []interface{}{true, uint32(guardAlg), pk, w.gen, w.nonce, w.bound}, vals, "height %d: eth_call", h)
				}
				require.Empty(test.t, f.readGuard(bucket, "").GetPublicKey(), "tip: record deleted")
				require.Equal(test.t, false, f.callGuard(bucket, nil)[0], "tip: eth_call")
			})},
		},
	})
}
