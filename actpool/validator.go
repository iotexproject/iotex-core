package actpool

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type blobValidator struct {
	blobCntLimitPerAcc uint64
	blobCntPerAcc      map[string]uint64
	mutex              sync.RWMutex
}

func newBlobValidator(blobCntLimitPerAcc uint64) *blobValidator {
	return &blobValidator{
		blobCntLimitPerAcc: blobCntLimitPerAcc,
		blobCntPerAcc:      make(map[string]uint64),
	}
}

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

func (v *blobValidator) OnAdded(act *action.SealedEnvelope) {
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
