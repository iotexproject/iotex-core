package actpool

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

// blobValidator limits the number of blob txs per account.
//
// A blob tx takes a slot from its sender's quota when it is submitted, before
// it is validated, so the limit bounds attempts rather than only the blob txs
// that end up in the pool. A slot is held in one of three states:
//   - pending: the tx is being validated or queued for its worker
//   - in pool: the tx was added to the pool, until it is removed
//   - rejected: the tx failed validation for a reason of its own (a malformed
//     tx, an invalid signature or an invalid blob sidecar), until the next block
//
// A tx rejected for a reason that depends on the pool or the chain state (such
// as its nonce or its price) gives its slot back at once.
type blobValidator struct {
	blobCntLimitPerAcc uint64
	blobCntPerAcc      map[string]uint64 // in pool
	pendingCntPerAcc   map[string]uint64
	rejectedCntPerAcc  map[string]uint64
	mutex              sync.RWMutex
}

func newBlobValidator(blobCntLimitPerAcc uint64) *blobValidator {
	return &blobValidator{
		blobCntLimitPerAcc: blobCntLimitPerAcc,
		blobCntPerAcc:      make(map[string]uint64),
		pendingCntPerAcc:   make(map[string]uint64),
		rejectedCntPerAcc:  make(map[string]uint64),
	}
}

// reserve takes a slot from the sender's quota for a blob tx about to be
// validated. The caller must give it back with release once the tx has been
// added to the pool or rejected.
func (v *blobValidator) reserve(act *action.SealedEnvelope) error {
	if v == nil || len(act.BlobHashes()) == 0 {
		return nil
	}
	v.mutex.Lock()
	defer v.mutex.Unlock()
	sender := act.SenderAddress().String()
	if v.blobCntPerAcc[sender]+v.pendingCntPerAcc[sender]+v.rejectedCntPerAcc[sender] >= v.blobCntLimitPerAcc {
		return errors.Wrap(action.ErrNonceTooHigh, "too many blob txs in the queue")
	}
	v.pendingCntPerAcc[sender]++
	return nil
}

// release gives back a slot taken by reserve. If rejected is true, the tx was
// rejected for a reason of its own, and the slot stays taken until the next
// block.
func (v *blobValidator) release(act *action.SealedEnvelope, rejected bool) {
	if v == nil || len(act.BlobHashes()) == 0 {
		return
	}
	v.mutex.Lock()
	defer v.mutex.Unlock()
	sender := act.SenderAddress().String()
	switch cnt := v.pendingCntPerAcc[sender]; cnt {
	case 0:
		log.L().Warn("pending blob count per account is already 0", zap.String("sender", sender))
		return
	case 1:
		delete(v.pendingCntPerAcc, sender)
	default:
		v.pendingCntPerAcc[sender] = cnt - 1
	}
	if rejected {
		v.rejectedCntPerAcc[sender]++
	}
}

// resetRejected gives back the slots of rejected blob txs; it is called on
// every new block.
func (v *blobValidator) resetRejected() {
	if v == nil {
		return
	}
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.rejectedCntPerAcc = make(map[string]uint64)
}

// Validate checks the number of the sender's blob txs in the pool. It runs on
// the sender's worker, where that number is updated, right before the tx is
// put into the pool.
func (v *blobValidator) Validate(ctx context.Context, act *action.SealedEnvelope) error {
	if len(act.BlobHashes()) == 0 {
		return nil
	}
	v.mutex.RLock()
	defer v.mutex.RUnlock()
	// check max number of blob txs per account
	sender := act.SenderAddress().String()
	if v.blobCntPerAcc[sender] >= v.blobCntLimitPerAcc {
		return errors.Wrap(action.ErrNonceTooHigh, "too many blob txs in the queue")
	}
	return nil
}

func (v *blobValidator) OnAdded(ctx context.Context, act *action.SealedEnvelope) {
	if len(act.BlobHashes()) == 0 {
		return
	}
	v.mutex.Lock()
	defer v.mutex.Unlock()
	sender := act.SenderAddress().String()
	v.blobCntPerAcc[sender]++
}

func (v *blobValidator) OnRemoved(act *action.SealedEnvelope) {
	if len(act.BlobHashes()) == 0 {
		return
	}
	v.mutex.Lock()
	defer v.mutex.Unlock()
	sender := act.SenderAddress().String()
	if v.blobCntPerAcc[sender] == 0 {
		log.L().Warn("blob count per account is already 0", zap.String("sender", sender))
		return
	}
	v.blobCntPerAcc[sender]--
}

// isRejectedForOwnReason reports whether a validation error is caused by the
// tx itself, rather than by the chain state or the price it offers, which an
// honest sender may fall behind on.
func isRejectedForOwnReason(err error) bool {
	switch {
	case errors.Is(err, action.ErrNonceTooLow),
		errors.Is(err, action.ErrUnderpriced),
		errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return false
	default:
		return true
	}
}
