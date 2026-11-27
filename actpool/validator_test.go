// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/holiman/uint256"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	. "github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

// TestActPool_blobCapEnforcedByWorker submits more blob txs than the
// per-account cap while the sender's worker is not draining its queue. Each tx
// takes a slot of the quota before it is queued, so the txs over the cap are
// rejected before they reach the worker; the worker still enforces the cap on
// a tx handed to it directly.
func TestActPool_blobCapEnforcedByWorker(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := require.New(t)

	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(acc interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := acc.(*state.Account)
		r.True(ok)
		r.NoError(acct.AddBalance(big.NewInt(1_000_000_000_000)))
		return 0, nil
	}).AnyTimes()

	cfg := getActPoolCfg()
	Ap, err := NewActPool(genesis.TestDefault(), sf, cfg)
	r.NoError(err)
	ap := Ap.(*actPool)
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())

	sk := identityset.PrivateKey(1)
	sender := sk.PublicKey().Address()
	recipient := identityset.Address(2).String()
	idx := ap.allocatedWorker(sender)
	worker := ap.worker[idx]
	// detach the sender's job queue from its worker so jobs pile up
	queue := make(chan workerJob, 2*_maxNumBlobTxPerAcct)
	ap.jobQueue[idx] = queue

	blobTx := func(nonce uint64) *action.SealedEnvelope {
		tx := action.NewBlobTx(0, nonce, 100000, big.NewInt(1), big.NewInt(1), nil,
			action.NewBlobTxData(uint256.NewInt(1), []common.Hash{{byte(nonce)}}, nil))
		selp, err := action.Sign(action.NewEnvelope(tx, action.NewTransfer(big.NewInt(1), recipient, nil)), sk)
		r.NoError(err)
		return selp
	}
	total := _maxNumBlobTxPerAcct + 2
	errs := make([]chan error, total)
	for i := 0; i < total; i++ {
		selp := blobTx(uint64(i + 1))
		errs[i] = make(chan error, 1)
		go func(c chan error) { c <- ap.Add(ctx, selp) }(errs[i])
		if i < _maxNumBlobTxPerAcct {
			r.Eventually(func() bool { return len(queue) == i+1 }, 5*time.Second, time.Millisecond)
		} else {
			r.ErrorIs(<-errs[i], action.ErrNonceTooHigh)
			r.Len(queue, _maxNumBlobTxPerAcct)
		}
	}
	for i := 0; i < _maxNumBlobTxPerAcct; i++ {
		job := <-queue
		job.err <- worker.Handle(job)
	}
	for i := 0; i < _maxNumBlobTxPerAcct; i++ {
		r.NoError(<-errs[i])
	}
	r.Len(ap.GetUnconfirmedActs(sender.String()), _maxNumBlobTxPerAcct)

	// the worker re-checks the cap on a tx that did not go through Add
	errc := make(chan error, 1)
	r.ErrorIs(worker.Handle(workerJob{ctx: ctx, act: blobTx(uint64(_maxNumBlobTxPerAcct + 1)), err: errc}), action.ErrNonceTooHigh)
	r.Len(ap.GetUnconfirmedActs(sender.String()), _maxNumBlobTxPerAcct)
	// no slot is left pending
	ap.blobValidator.mutex.RLock()
	r.Empty(ap.blobValidator.pendingCntPerAcc)
	ap.blobValidator.mutex.RUnlock()
}

func testBlobSelp(r *require.Assertions, signer, nonce int, validProof bool) *action.SealedEnvelope {
	var blob, other kzg4844.Blob
	blob[0], other[0] = 1, 2
	commitment := MustNoErrorV(kzg4844.BlobToCommitment(&blob))
	proof := MustNoErrorV(kzg4844.ComputeBlobProof(&blob, commitment))
	if !validProof {
		// a well-formed proof for a different blob
		proof = MustNoErrorV(kzg4844.ComputeBlobProof(&other, MustNoErrorV(kzg4844.BlobToCommitment(&other))))
	}
	tx := types.MustSignNewTx(identityset.PrivateKey(signer).EcdsaPrivateKey().(*ecdsa.PrivateKey),
		types.NewCancunSigner(big.NewInt(4689)), &types.BlobTx{
			Nonce:      uint64(nonce),
			GasTipCap:  uint256.NewInt(10),
			GasFeeCap:  uint256.NewInt(2000000000000),
			Gas:        100000,
			To:         common.BytesToAddress(identityset.Address(1).Bytes()),
			Value:      uint256.NewInt(1),
			BlobFeeCap: uint256.NewInt(10),
			BlobHashes: []common.Hash{kzg4844.CalcBlobHashV1(sha256.New(), &commitment)},
			Sidecar: &types.BlobTxSidecar{
				Blobs:       []kzg4844.Blob{blob},
				Commitments: []kzg4844.Commitment{commitment},
				Proofs:      []kzg4844.Proof{proof},
			},
		})
	_, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
	r.NoError(err)
	return MustNoErrorV((&action.Deserializer{}).SetEvmNetworkID(4689).ActionToSealedEnvelope(&iotextypes.Action{
		Core:         MustNoErrorV(action.EthRawToContainer(1, hex.EncodeToString(MustNoErrorV(tx.MarshalBinary())))),
		SenderPubKey: pubkey.Bytes(),
		Signature:    sig,
		Encoding:     iotextypes.Encoding_TX_CONTAINER,
	}))
}

func TestBlobValidator_QuotaCountsAttempts(t *testing.T) {
	r := require.New(t)
	act := testBlobSelp(r, 1, 0, true)
	sender := act.SenderAddress().String()

	t.Run("a rejected attempt keeps its slot until the next block", func(t *testing.T) {
		v := newBlobValidator(1)
		r.NoError(v.reserve(act))
		v.release(act, true)
		for i := 0; i < 100; i++ {
			r.ErrorIs(v.reserve(act), action.ErrNonceTooHigh)
		}
		v.resetRejected()
		r.NoError(v.reserve(act))
	})
	t.Run("an attempt rejected for the state gives its slot back", func(t *testing.T) {
		v := newBlobValidator(1)
		for i := 0; i < 100; i++ {
			r.NoError(v.reserve(act))
			v.release(act, false)
		}
		r.Empty(v.pendingCntPerAcc)
		r.Empty(v.rejectedCntPerAcc)
	})
	t.Run("an added tx holds its slot until it is removed", func(t *testing.T) {
		v := newBlobValidator(2)
		for i := 0; i < 2; i++ {
			r.NoError(v.reserve(act))
			r.NoError(v.Validate(context.Background(), act))
			v.OnAdded(context.Background(), act)
			v.release(act, false)
		}
		r.Equal(uint64(2), v.blobCntPerAcc[sender])
		r.Empty(v.pendingCntPerAcc)
		r.ErrorIs(v.reserve(act), action.ErrNonceTooHigh)
		r.ErrorIs(v.Validate(context.Background(), act), action.ErrNonceTooHigh)
		v.resetRejected() // a new block does not free a slot held in the pool
		r.ErrorIs(v.reserve(act), action.ErrNonceTooHigh)
		v.OnRemoved(act)
		r.NoError(v.reserve(act))
	})
	t.Run("concurrent attempts", func(t *testing.T) {
		const limit = 4
		v := newBlobValidator(limit)
		var (
			wg sync.WaitGroup
			ok atomic.Int32
		)
		for i := 0; i < 64; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if v.reserve(act) == nil {
					ok.Add(1)
				}
			}()
		}
		wg.Wait()
		r.EqualValues(limit, ok.Load())
	})
	t.Run("non-blob tx", func(t *testing.T) {
		v := newBlobValidator(0)
		tsf, err := action.SignedTransfer(identityset.Address(2).String(), identityset.PrivateKey(1), 1, big.NewInt(1), nil, 100000, big.NewInt(1))
		r.NoError(err)
		r.NoError(v.reserve(tsf))
		v.release(tsf, true)
		r.Empty(v.rejectedCntPerAcc)
	})
}

// TestActPool_blobQuotaBoundsInvalidSidecar submits a blob tx whose sidecar
// fails verification again and again. Each attempt takes a slot of the
// sender's blob quota, so once the quota is used up the tx is rejected before
// its sidecar is verified, until the next block.
func TestActPool_blobQuotaBoundsInvalidSidecar(t *testing.T) {
	ctrl := gomock.NewController(t)
	r := require.New(t)

	sf := mock_chainmanager.NewMockStateReader(ctrl)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	sf.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(func(acc interface{}, opts ...protocol.StateOption) (uint64, error) {
		acct, ok := acc.(*state.Account)
		r.True(ok)
		r.NoError(acct.AddBalance(unit.ConvertIotxToRau(100)))
		return 0, nil
	}).AnyTimes()

	g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
	g.VanuatuBlockHeight = 0
	Ap, err := NewActPool(g, sf, getActPoolCfg())
	r.NoError(err)
	ap := Ap.(*actPool)
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())
	r.NoError(ap.Start(ctx))
	defer ap.Stop(ctx)
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	bad, good := testBlobSelp(r, 1, 1, false), testBlobSelp(r, 1, 1, true)
	// an attempt rejected for its nonce does not use up the quota
	for i := 0; i < 2*_maxNumBlobTxPerAcct; i++ {
		err := ap.Add(ctx, testBlobSelp(r, 1, 2, true))
		r.ErrorContains(err, "is larger than pending nonce")
	}
	for i := 0; i < _maxNumBlobTxPerAcct; i++ {
		err := ap.Add(ctx, bad)
		r.ErrorContains(err, "failed to validate blob sidecar")
	}
	for i := 0; i < 10; i++ {
		err := ap.Add(ctx, bad)
		r.ErrorIs(err, action.ErrNonceTooHigh)
		r.ErrorContains(err, "too many blob txs")
	}
	// the quota is per account
	r.NoError(ap.Add(ctx, testBlobSelp(r, 2, 1, true)))
	// until the next block, the sender cannot add a valid blob tx either
	r.ErrorIs(ap.Add(ctx, good), action.ErrNonceTooHigh)
	ap.Reset()
	r.NoError(ap.Add(ctx, good))
	h, err := good.Hash()
	r.NoError(err)
	_, err = ap.GetActionByHash(h)
	r.NoError(err)
}
