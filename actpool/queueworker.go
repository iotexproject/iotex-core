package actpool

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/iotexproject/go-pkgs/cache/ttl"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/tracer"
)

type (
	workerJob struct {
		ctx context.Context
		act action.SealedEnvelope
		err chan error
	}

	queueWorker struct {
		queue chan workerJob
		ap    *actPool

		mu            sync.RWMutex
		accountActs   map[string]ActQueue
		emptyAccounts *ttl.Cache
	}

	pendingActions struct {
		sender string
		acts   []action.SealedEnvelope
	}
)

func newQueueWorker(ap *actPool, jobQueue chan workerJob) *queueWorker {
	acc, _ := ttl.NewCache()
	return &queueWorker{
		queue:         jobQueue,
		ap:            ap,
		accountActs:   make(map[string]ActQueue),
		emptyAccounts: acc,
	}
}

func (worker *queueWorker) Start() {
	if worker.queue == nil || worker.ap == nil {
		panic("worker is invalid")
	}
	for {
		job, more := <-worker.queue
		if !more { // worker chan is closed
			return
		}
		worker.Handle(job)
	}
}

func (worker *queueWorker) Handle(job workerJob) {
	// ctx is canceled or timeout
	if job.ctx.Err() != nil {
		job.err <- job.ctx.Err()
		return
	}

	ctx := job.ctx
	span := tracer.SpanFromContext(ctx)
	defer span.End()
	var (
		act             = job.act
		sender          = act.SrcPubkey().Address().String()
		actHash, _      = act.Hash()
		intrinsicGas, _ = act.IntrinsicGas()
	)

	confirmedNonce, confirmedBalance, err := worker.getConfirmedState(act.SrcPubkey().Address())
	if err != nil {
		job.err <- err
		return
	}

	if err := worker.checkSelpWithState(&act, confirmedNonce, confirmedBalance); err != nil {
		job.err <- err
		return
	}

	if err := worker.putAction(sender, act, confirmedNonce, confirmedBalance); err != nil {
		job.err <- err
		return
	}

	worker.ap.allActions.Set(actHash, act)

	//add actions to destination map
	if desAddress, ok := act.Destination(); ok && !strings.EqualFold(sender, desAddress) {
		worker.ap.accountDesActs.mu.Lock()
		if desQueue := worker.ap.accountDesActs.acts[desAddress]; desQueue == nil {
			worker.ap.accountDesActs.acts[desAddress] = make(map[hash.Hash256]action.SealedEnvelope)
		}
		worker.ap.accountDesActs.acts[desAddress][actHash] = act
		worker.ap.accountDesActs.mu.Unlock()
	}

	atomic.AddUint64(&worker.ap.gasInPool, intrinsicGas)

	job.err <- nil

	worker.removeEmptyAccounts()
}

func (worker *queueWorker) getConfirmedState(sender address.Address) (uint64, *big.Int, error) {
	queue := worker.accountActs[sender.String()]
	// account state isn't cached in the actpool
	if queue == nil {
		confirmedState, err := accountutil.AccountState(worker.ap.sf, sender)
		if err != nil {
			return 0, nil, err
		}
		return confirmedState.Nonce, confirmedState.Balance, nil
	}
	return queue.PendingNonce(), queue.PendingBalance(), nil
}

func (worker *queueWorker) checkSelpWithState(act *action.SealedEnvelope, confirmedNonce uint64, confirmedBalance *big.Int) error {
	if act.Nonce() <= confirmedNonce {
		_actpoolMtc.WithLabelValues("nonceTooSmall").Inc()
		return action.ErrNonceTooLow
	}

	// Nonce exceeds current range
	if act.Nonce()-confirmedNonce >= worker.ap.cfg.MaxNumActsPerAcct+1 {
		hash, _ := act.Hash()
		log.L().Debug("Rejecting action because nonce is too large.",
			log.Hex("hash", hash[:]),
			zap.Uint64("startNonce", confirmedNonce+1),
			zap.Uint64("actNonce", act.Nonce()))
		_actpoolMtc.WithLabelValues("nonceTooLarge").Inc()
		return action.ErrNonceTooHigh
	}

	if cost, _ := act.Cost(); confirmedBalance.Cmp(cost) < 0 {
		_actpoolMtc.WithLabelValues("insufficientBalance").Inc()
		sender := act.SrcPubkey().Address().String()
		actHash, _ := act.Hash()
		log.L().Info("insufficient balance for action",
			zap.String("actionHash", hex.EncodeToString(actHash[:])),
			zap.String("cost", cost.String()),
			zap.String("pendingBalance", confirmedBalance.String()),
			zap.String("sender", sender),
		)
		return action.ErrInsufficientFunds
	}
	return nil
}

func (worker *queueWorker) putAction(sender string, act action.SealedEnvelope, confirmedNonce uint64, confirmedBalance *big.Int) error {
	queue := worker.accountActs[sender]

	if queue == nil {
		queue = NewActQueue(worker.ap, sender, WithTimeOut(worker.ap.cfg.ActionExpiry))
		queue.SetPendingNonce(confirmedNonce + 1)
		queue.SetPendingBalance(confirmedBalance)
		worker.mu.Lock()
		worker.accountActs[sender] = queue
		worker.mu.Unlock()
	}

	if err := queue.Put(act); err != nil {
		actHash, _ := act.Hash()
		_actpoolMtc.WithLabelValues("failedPutActQueue").Inc()
		log.L().Info("failed put action into ActQueue", zap.String("actionHash", hex.EncodeToString(actHash[:])))
		return err
	}

	return nil
}

func (worker *queueWorker) removeEmptyAccounts() {
	if worker.emptyAccounts.Count() == 0 {
		return
	}

	worker.mu.Lock()
	defer worker.mu.Unlock()

	worker.emptyAccounts.Range(func(key, _ interface{}) error {
		sender := key.(string)
		if worker.accountActs[sender].IsEmpty() {
			delete(worker.accountActs, sender)
		}
		return nil
	})

	worker.emptyAccounts.Reset()
}

func (worker *queueWorker) Reset() {
	worker.mu.RLock()
	defer worker.mu.RUnlock()

	for from, queue := range worker.accountActs {
		addr, _ := address.FromString(from)
		confirmedState, err := accountutil.AccountState(worker.ap.sf, addr)
		if err != nil {
			log.L().Error("Error when removing confirmed actions", zap.Error(err))
			queue.Reset()
			worker.emptyAccounts.Set(from, struct{}{})
			continue
		}
		pendingNonce := confirmedState.Nonce + 1
		queue.SetPendingNonce(pendingNonce)
		queue.SetPendingBalance(confirmedState.Balance)
		// Remove all actions that are committed to new block
		acts := queue.CleanConfrirmedAct()
		acts2 := queue.UpdateQueue(queue.PendingNonce())
		worker.ap.removeInvalidActs(append(acts, acts2...))
		// Delete the queue entry if it becomes empty
		if queue.IsEmpty() {
			worker.emptyAccounts.Set(from, struct{}{})
		}
	}
}

// PendingActionIterator returns an action interator with all accepted actions
func (worker *queueWorker) PendingAction() []*pendingActions {
	actionArr := make([]*pendingActions, 0)

	worker.mu.RLock()
	defer worker.mu.RUnlock()
	for from, queue := range worker.accountActs {
		if queue.IsEmpty() {
			continue
		}
		// Remove the actions that are already timeout
		acts := queue.UpdateQueue(queue.PendingNonce())
		worker.ap.removeInvalidActs(acts)
		actionArr = append(actionArr, &pendingActions{
			sender: from,
			acts:   queue.PendingActs(),
		})
	}
	return actionArr
}

// GetQueue returns the actQueue of sender
func (worker *queueWorker) GetQueue(sender address.Address) ActQueue {
	worker.mu.RLock()
	defer worker.mu.RUnlock()
	return worker.accountActs[sender.String()]
}

// ResetAccount resets account in the accountActs of worker
func (worker *queueWorker) ResetAccount(sender address.Address) {
	senderStr := sender.String()
	worker.mu.RLock()
	defer worker.mu.RUnlock()
	if queue := worker.accountActs[senderStr]; queue != nil {
		queue.Reset()
		worker.emptyAccounts.Set(senderStr, struct{}{})
	}
}
