// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
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
	guardCtx         = []byte(bucketguard.Context)
	guardEventTopic  = crypto.Keccak256([]byte("BucketGuard(uint64,uint64,uint64,uint64,address,address)"))
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
	chainID   uint32 // native chain ID, as the transcripts require
	gasLimit  uint64
	gasPrice  *big.Int
	start     time.Time
	blockTime time.Time
}

func newGuardFixture(t *testing.T) *guardFixture {
	cfg := initCfg(require.New(t))
	cfg.Genesis.ToBeEnabledBlockHeight = guardFork
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	test := newE2ETest(t, cfg)
	now := time.Now()
	return &guardFixture{
		test:      test,
		chainID:   test.cfg.Chain.ID,
		gasLimit:  2_000_000,
		gasPrice:  big.NewInt(unit.Qev*2 + 1),
		start:     now,
		blockTime: now,
	}
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

func (f *guardFixture) signed(id int, p guardPayload) *actionWithTime {
	elp := (&action.EnvelopeBuilder{}).SetNonce(f.nonce(id)).
		SetGasLimit(f.gasLimit).SetGasPrice(f.gasPrice).SetChainID(f.chainID).SetAction(p).Build()
	return f.at(action.Sign(elp, identityset.PrivateKey(id)))
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
	acc := mustNoErr(k.Sign(guardCtx, bucketguard.EncodeAccept(f.chainID, index, newGen, owner, guardAlg, pub, 0)))
	return action.NewBindBucketGuard(&bucketguard.BindBucketGuard{BucketIndex: index, AlgID: guardAlg, PublicKey: pub, AcceptSig: acc})
}

func guardOp(index uint64, o bucketguard.Op, gen, nonce uint64) *bucketguard.GuardedBucketOp {
	return &bucketguard.GuardedBucketOp{BucketIndex: index, Generation: gen, Nonce: nonce, Deadline: 1000, Op: o}
}

// newGuard attaches next as the incoming key, accepted for resultingOwner.
func (f *guardFixture) newGuard(op *bucketguard.GuardedBucketOp, next *signer.PrivateKey, resultingOwner address.Address) {
	pub := next.PublicKey().Bytes()
	acc := mustNoErr(next.Sign(guardCtx, bucketguard.EncodeAccept(f.chainID, op.BucketIndex, op.Generation+1, resultingOwner, guardAlg, pub, op.Deadline)))
	op.NewGuard = &bucketguard.NewGuard{AlgID: guardAlg, PublicKey: pub, AcceptSig: acc}
}

// guarded signs op's CALL with k for a bucket owned by owner at execution time.
func (f *guardFixture) guarded(k *signer.PrivateKey, owner address.Address, op *bucketguard.GuardedBucketOp) *action.GuardedBucketOp {
	op.Sig = mustNoErr(k.Sign(guardCtx, bucketguard.EncodeCall(f.chainID, owner, op)))
	return action.NewGuardedBucketOp(op)
}

func (f *guardFixture) balance(id int) *big.Int {
	ctx := genesis.WithGenesisContext(context.Background(), f.test.cfg.Genesis)
	acc, err := accountutil.AccountState(ctx, f.test.cs.StateFactory(), identityset.Address(id))
	require.NoError(f.test.t, err)
	return acc.Balance
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

// guardEventIs checks the single IIP §11 event of a successful bind or op.
func guardEventIs(index uint64, op uint32, gen, nonce uint64, sponsor, counterparty address.Address) actionExpect {
	return expectFn(func(test *e2etest, receipt *action.Receipt) {
		require.Len(test.t, receipt.Logs(), 1, "exactly one event, no legacy logs")
		l := receipt.Logs()[0]
		require.Equal(test.t, guardEventTopic, l.Topics[0][:])
		require.Equal(test.t, index, binary.BigEndian.Uint64(l.Topics[1][24:]))
		require.Equal(test.t, op, binary.BigEndian.Uint32(l.Topics[2][28:]))
		require.Len(test.t, l.Data, 56)
		require.Equal(test.t, gen, binary.BigEndian.Uint64(l.Data[0:8]))
		require.Equal(test.t, nonce, binary.BigEndian.Uint64(l.Data[8:16]))
		require.Equal(test.t, sponsor.Bytes(), l.Data[16:36], "sponsor")
		want := make([]byte, 20)
		if counterparty != nil {
			want = counterparty.Bytes()
		}
		require.Equal(test.t, want, l.Data[36:56], "counterparty")
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
// submitted by a sponsor that is not the owner, replay and wrong-key failures
// with state rollback, key rotation, unstake and withdrawal to a signed
// recipient.
func TestBucketGuard(t *testing.T) {
	f := newGuardFixture(t)
	defer f.test.teardown()
	var (
		ownerID, sponsorID, recipientID = 3, 4, 5
		owner                           = identityset.Address(ownerID)
		sponsor                         = identityset.Address(sponsorID)
		cand2ID                         = identityset.Address(2) // candidates are identified by owner
		keyA, keyB                      = guardKey(0xa), guardKey(0xb)
		recipientStart                  *big.Int
	)
	const (
		selfStakeBucket = 0 // cand1's self-stake
		bucket          = 2
		plainBucket     = 3
	)
	f.test.run([]*testcase{
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
			expect: []actionExpect{successExpect, guardIs(bucket, 1, 0, keyA), guardEventIs(bucket, 0, 1, 0, owner, nil)},
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
			name: "sponsor submits a guarded CHANGE_CANDIDATE signed by key A",
			act: func() *actionWithTime {
				m := guardOp(bucket, bucketguard.OpChangeCandidate, 1, 0)
				m.CandidateName, m.ExpectedCandidateID = "cand2", cand2ID.Bytes()
				return f.signed(sponsorID, f.guarded(keyA, owner, m))
			}(),
			expect: []actionExpect{successExpect, guardIs(bucket, 1, 1, keyA), guardEventIs(bucket, uint32(bucketguard.OpChangeCandidate), 1, 1, sponsor, nil),
				bucketIs(bucket, func(t *testing.T, b *iotextypes.VoteBucket) {
					require.Equal(t, cand2ID.String(), b.CandidateAddress)
				})},
		},
		{
			name: "replaying the same signed op fails on the counters",
			act: func() *actionWithTime {
				m := guardOp(bucket, bucketguard.OpChangeCandidate, 1, 0)
				m.CandidateName, m.ExpectedCandidateID = "cand2", cand2ID.Bytes()
				return f.signed(sponsorID, f.guarded(keyA, owner, m))
			}(),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrGuardMismatch), guardIs(bucket, 1, 1, keyA)},
		},
		{
			name: "a key that is not the guard is refused and the restake is rolled back",
			act: func() *actionWithTime {
				m := guardOp(bucket, bucketguard.OpRestake, 1, 1)
				m.Duration = 30
				return f.signed(sponsorID, f.guarded(keyB, owner, m))
			}(),
			expect: []actionExpect{receiptStatus(iotextypes.ReceiptStatus_ErrGuardSignature), guardIs(bucket, 1, 1, keyA),
				bucketIs(bucket, func(t *testing.T, b *iotextypes.VoteBucket) {
					require.EqualValues(t, 0, b.StakedDuration, "restake effect must be reverted")
				})},
		},
		{
			name: "ROTATE from key A to key B",
			act: func() *actionWithTime {
				m := guardOp(bucket, bucketguard.OpRotate, 1, 1)
				f.newGuard(m, keyB, owner)
				return f.signed(sponsorID, f.guarded(keyA, owner, m))
			}(),
			expect: []actionExpect{successExpect, guardIs(bucket, 2, 2, keyB), guardEventIs(bucket, uint32(bucketguard.OpRotate), 2, 2, sponsor, nil)},
		},
		{
			name:   "UNSTAKE with key B",
			act:    f.signed(sponsorID, f.guarded(keyB, owner, guardOp(bucket, bucketguard.OpUnstake, 2, 2))),
			expect: []actionExpect{successExpect, guardIs(bucket, 2, 3, keyB)},
		},
		{
			name:    "WITHDRAW pays the signed recipient and deletes the guard",
			preFunc: func(*e2etest) { recipientStart = f.balance(recipientID) },
			act: func() *actionWithTime {
				m := guardOp(bucket, bucketguard.OpWithdraw, 2, 3)
				m.Recipient = identityset.Address(recipientID).Bytes()
				a := f.signed(sponsorID, f.guarded(keyB, owner, m))
				a.t = f.start.Add(f.test.cfg.Genesis.WithdrawWaitingPeriod + 24*time.Hour)
				return a
			}(),
			expect: []actionExpect{successExpect, guardAbsent(bucket),
				guardEventIs(bucket, uint32(bucketguard.OpWithdraw), 2, 4, sponsor, identityset.Address(recipientID)),
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
// a transfer that clears the guard, and the legacy paths working again on
// the tombstone until a withdrawal deletes it.
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
	f.test.run([]*testcase{
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
			name: "TRANSFER to the new owner installs the new owner's key C",
			act: func() *actionWithTime {
				m := guardOp(bucket, bucketguard.OpTransfer, 1, 0)
				m.NewOwner = newOwner.Bytes()
				f.newGuard(m, keyC, newOwner)
				return f.signed(newOwnerID, f.guarded(keyA, owner, m))
			}(),
			expect: []actionExpect{successExpect, guardIs(bucket, 2, 1, keyC),
				guardEventIs(bucket, uint32(bucketguard.OpTransfer), 2, 1, newOwner, newOwner),
				bucketIs(bucket, func(t *testing.T, b *iotextypes.VoteBucket) {
					require.Equal(t, newOwner.String(), b.Owner)
				})},
		},
		{
			name: "TRANSFER back with clearGuard leaves a tombstone",
			act: func() *actionWithTime {
				m := guardOp(bucket, bucketguard.OpTransfer, 2, 1)
				m.NewOwner, m.ClearGuard = owner.Bytes(), true
				return f.signed(newOwnerID, f.guarded(keyC, newOwner, m))
			}(),
			expect: []actionExpect{successExpect, guardIs(bucket, 2, 2, nil),
				bucketIs(bucket, func(t *testing.T, b *iotextypes.VoteBucket) {
					require.Equal(t, owner.String(), b.Owner)
				})},
		},
		{
			name:   "legacy Unstake works on the cleared bucket",
			act:    f.at(action.SignedReclaimStake(false, f.nonce(ownerID), bucket, nil, f.gasLimit, f.gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(f.chainID))),
			expect: []actionExpect{successExpect, guardIs(bucket, 2, 2, nil)},
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
