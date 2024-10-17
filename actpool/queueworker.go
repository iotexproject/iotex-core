package actpool

import (
	"context"
	"encoding/hex"
	"math/big"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/iotexproject/go-pkgs/cache/ttl"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/tracer"
)

type (
	queueWorker struct {
		queue         chan workerJob
		ap            *actPool
		mu            sync.RWMutex
		accountActs   *accountPool
		emptyAccounts *ttl.Cache
	}

	workerJob struct {
		ctx context.Context
		act *action.SealedEnvelope
		rep bool
		err chan error
	}

	pendingActions struct {
		sender string
		acts   []*action.SealedEnvelope
	}
)

func newQueueWorker(ap *actPool, jobQueue chan workerJob) *queueWorker {
	acc, _ := ttl.NewCache()
	return &queueWorker{
		queue:         jobQueue,
		ap:            ap,
		accountActs:   newAccountPool(),
		emptyAccounts: acc,
	}
}

func (worker *queueWorker) Start() error {
	if worker.queue == nil || worker.ap == nil {
		return errors.New("worker is invalid")
	}
	go func() {
		for {
			job, more := <-worker.queue
			if !more { // worker chan is closed
				return
			}
			job.err <- worker.Handle(job)
		}
	}()
	return nil
}

func (worker *queueWorker) Stop() error {
	close(worker.queue)
	return nil
}

// Handle is called sequentially by worker
func (worker *queueWorker) Handle(job workerJob) error {
	ctx := job.ctx
	// ctx is canceled or timeout
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var (
		span            = tracer.SpanFromContext(ctx)
		act             = job.act
		sender          = act.SenderAddress().String()
		actHash, _      = act.Hash()
		intrinsicGas, _ = act.IntrinsicGas()
		replace         = job.rep
	)
	defer span.End()

	nonce, balance, err := worker.getConfirmedState(ctx, act.SenderAddress())
	if err != nil {
		return err
	}

	if err := worker.checkSelpWithState(act, nonce, balance); err != nil {
		return err
	}
	if err := worker.putAction(sender, act, nonce, balance); err != nil {
		return err
	}

	worker.ap.allActions.Set(actHash, act)
	worker.ap.onAdded(act)
	isBlobTx := len(act.BlobHashes()) > 0 // only store blob tx
	if worker.ap.store != nil && isBlobTx {
		if err := worker.ap.store.Put(act); err != nil {
			log.L().Warn("failed to store action", zap.Error(err), log.Hex("hash", actHash[:]))
		}
	}

	if desAddress, ok := act.Destination(); ok && !strings.EqualFold(sender, desAddress) {
		if err := worker.ap.accountDesActs.addAction(act); err != nil {
			log.L().Debug("fail to add destination map", zap.Error(err))
		}
	}

	atomic.AddUint64(&worker.ap.gasInPool, intrinsicGas)

	worker.mu.Lock()
	defer worker.mu.Unlock()
	if replace {
		// TODO: early return if sender is the account to pop and nonce is larger than largest in the queue
		actToReplace := worker.accountActs.PopPeek()
		if actToReplace == nil {
			log.L().Warn("UNEXPECTED ERROR: action pool is full, but no action to drop")
			return nil
		}
		worker.ap.removeInvalidActs([]*action.SealedEnvelope{actToReplace})
		if actToReplace.SenderAddress().String() == sender && actToReplace.Nonce() == nonce {
			err = action.ErrTxPoolOverflow
			_actpoolMtc.WithLabelValues("overMaxNumActsPerPool").Inc()
		}
	}

	worker.removeEmptyAccounts()

	return err
}

func (worker *queueWorker) getConfirmedState(ctx context.Context, sender address.Address) (uint64, *big.Int, error) {
	worker.mu.RLock()
	queue := worker.accountActs.Account(sender.String())
	worker.mu.RUnlock()
	// account state isn't cached in the actpool
	if queue == nil {
		confirmedState, err := accountutil.AccountState(ctx, worker.ap.sf, sender)
		if err != nil {
			return 0, nil, err
		}
		var nonce uint64
		if protocol.MustGetFeatureCtx(ctx).UseZeroNonceForFreshAccount {
			nonce = confirmedState.PendingNonceConsideringFreshAccount()
		} else {
			nonce = confirmedState.PendingNonce()
		}
		return nonce, confirmedState.Balance, nil
	}
	nonce, balance := queue.AccountState()
	return nonce, balance, nil
}

func (worker *queueWorker) checkSelpWithState(act *action.SealedEnvelope, pendingNonce uint64, balance *big.Int) error {
	if act.Nonce() < pendingNonce {
		_actpoolMtc.WithLabelValues("nonceTooSmall").Inc()
		return action.ErrNonceTooLow
	}

	// Nonce exceeds current range
	if act.Nonce()-pendingNonce >= worker.ap.cfg.MaxNumActsPerAcct {
		hash, _ := act.Hash()
		log.L().Debug("Rejecting action because nonce is too large.",
			log.Hex("hash", hash[:]),
			zap.Uint64("startNonce", pendingNonce),
			zap.Uint64("actNonce", act.Nonce()))
		_actpoolMtc.WithLabelValues("nonceTooLarge").Inc()
		return action.ErrNonceTooHigh
	}

	// Nonce must be continuous for blob tx
	if len(act.BlobHashes()) > 0 {
		pendingNonceInPool, ok := worker.PendingNonce(act.SenderAddress())
		if !ok {
			pendingNonceInPool = pendingNonce
		}
		if act.Nonce() > pendingNonceInPool {
			_actpoolMtc.WithLabelValues("nonceTooLarge").Inc()
			return errors.Wrapf(action.ErrNonceTooHigh, "nonce %d is larger than pending nonce %d", act.Nonce(), pendingNonceInPool)
		}
	}

	if cost, _ := act.Cost(); balance.Cmp(cost) < 0 {
		_actpoolMtc.WithLabelValues("insufficientBalance").Inc()
		sender := act.SenderAddress().String()
		actHash, _ := act.Hash()
		log.L().Debug("insufficient balance for action",
			zap.String("actionHash", hex.EncodeToString(actHash[:])),
			zap.String("cost", cost.String()),
			zap.String("balance", balance.String()),
			zap.String("sender", sender),
		)
		return action.ErrInsufficientFunds
	}
	return nil
}

func (worker *queueWorker) putAction(sender string, act *action.SealedEnvelope, pendingNonce uint64, confirmedBalance *big.Int) error {
	worker.mu.Lock()
	err := worker.accountActs.PutAction(
		sender,
		worker.ap,
		pendingNonce,
		confirmedBalance,
		worker.ap.cfg.ActionExpiry,
		act,
	)
	worker.mu.Unlock()
	if err != nil {
		actHash, _ := act.Hash()
		_actpoolMtc.WithLabelValues("failedPutActQueue").Inc()
		log.L().Debug("failed put action into ActQueue",
			zap.String("actionHash", hex.EncodeToString(actHash[:])),
			zap.Error(err))
		return err
	}

	return nil
}

func (worker *queueWorker) removeEmptyAccounts() {
	if worker.emptyAccounts.Count() == 0 {
		return
	}

	worker.emptyAccounts.Range(func(key, _ interface{}) error {
		worker.accountActs.DeleteIfEmpty(key.(string))
		return nil
	})

	worker.emptyAccounts.Reset()
}

func (worker *queueWorker) Reset(ctx context.Context) {
	worker.mu.RLock()
	defer worker.mu.RUnlock()

	worker.accountActs.Range(func(from string, queue ActQueue) {
		addr, _ := address.FromString(from)
		confirmedState, err := accountutil.AccountState(ctx, worker.ap.sf, addr)
		if err != nil {
			log.L().Error("Error when removing confirmed actions", zap.Error(err))
			queue.Reset()
			worker.emptyAccounts.Set(from, struct{}{})
			return
		}
		var pendingNonce uint64
		if protocol.MustGetFeatureCtx(ctx).UseZeroNonceForFreshAccount {
			pendingNonce = confirmedState.PendingNonceConsideringFreshAccount()
		} else {
			pendingNonce = confirmedState.PendingNonce()
		}
		// Remove all actions that are committed to new block
		acts := queue.UpdateAccountState(pendingNonce, confirmedState.Balance)
		acts2 := queue.UpdateQueue()
		worker.ap.removeInvalidActs(append(acts, acts2...))
		// Delete the queue entry if it becomes empty
		if queue.Empty() {
			worker.emptyAccounts.Set(from, struct{}{})
		}
	})
}

// PendingActions returns all accepted actions
func (worker *queueWorker) PendingActions(ctx context.Context) []*pendingActions {
	actionArr := make([]*pendingActions, 0)

	worker.mu.RLock()
	defer worker.mu.RUnlock()
	worker.accountActs.Range(func(from string, queue ActQueue) {
		if queue.Empty() {
			return
		}
		// Remove the actions that are already timeout
		acts := queue.UpdateQueue()
		worker.ap.removeInvalidActs(acts)
		pd := queue.PendingActs(ctx)
		if len(pd) == 0 {
			return
		}
		actionArr = append(actionArr, &pendingActions{
			sender: from,
			acts:   pd,
		})
	})
	return actionArr
}

// AllActions returns the all actions of sender
func (worker *queueWorker) AllActions(sender address.Address) ([]*action.SealedEnvelope, bool) {
	worker.mu.RLock()
	defer worker.mu.RUnlock()
	if actQueue := worker.accountActs.Account(sender.String()); actQueue != nil {
		acts := actQueue.AllActs()
		sort.Slice(acts, func(i, j int) bool {
			return acts[i].Nonce() < acts[j].Nonce()
		})
		return acts, true
	}
	return nil, false
}

// PendingNonce returns the pending nonce of sender
func (worker *queueWorker) PendingNonce(sender address.Address) (uint64, bool) {
	worker.mu.RLock()
	defer worker.mu.RUnlock()
	if actQueue := worker.accountActs.Account(sender.String()); actQueue != nil {
		return actQueue.PendingNonce(), true
	}
	return 0, false
}

// ResetAccount resets account in the accountActs of worker
func (worker *queueWorker) ResetAccount(sender address.Address) []*action.SealedEnvelope {
	senderStr := sender.String()
	worker.mu.RLock()
	actQueue := worker.accountActs.PopAccount(senderStr)
	worker.mu.RUnlock()
	if actQueue != nil {
		pendingActs := actQueue.AllActs()
		actQueue.Reset()
		// the following line is thread safe with worker.mu.RLock
		worker.emptyAccounts.Set(senderStr, struct{}{})
		return pendingActs
	}
	return nil
}
