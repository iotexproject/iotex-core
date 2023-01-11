package actpool

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/iotexproject/go-pkgs/cache/ttl"
	"github.com/iotexproject/iotex-address/address"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/tracer"
)

type (
	queueWorker struct {
		queue         chan workerJob
		ap            *actPool
		mu            sync.RWMutex
		accountActs   map[string]ActQueue
		emptyAccounts *ttl.Cache
	}

	workerJob struct {
		ctx context.Context
		act action.SealedEnvelope
		err chan error
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

// Hanlde is called sequentially by worker
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
	)
	defer span.End()

	nonce, balance, err := worker.getConfirmedState(ctx, act.SenderAddress())
	if err != nil {
		return err
	}

	if err := worker.checkSelpWithState(&act, nonce, balance); err != nil {
		return err
	}

	if err := worker.putAction(sender, act, nonce, balance); err != nil {
		return err
	}

	worker.ap.allActions.Set(actHash, act)

	if desAddress, ok := act.Destination(); ok && !strings.EqualFold(sender, desAddress) {
		if err := worker.ap.accountDesActs.addAction(act); err != nil {
			log.L().Debug("fail to add destionation map", zap.Error(err))
		}
	}

	atomic.AddUint64(&worker.ap.gasInPool, intrinsicGas)

	worker.mu.Lock()
	defer worker.mu.Unlock()
	worker.removeEmptyAccounts()

	return nil
}

func (worker *queueWorker) getConfirmedState(ctx context.Context, sender address.Address) (uint64, *big.Int, error) {
	worker.mu.RLock()
	queue := worker.accountActs[sender.String()]
	worker.mu.RUnlock()
	// account state isn't cached in the actpool
	if queue == nil {
		confirmedState, err := accountutil.AccountState(ctx, worker.ap.sf, sender)
		if err != nil {
			return 0, nil, err
		}
		return confirmedState.PendingNonce(), confirmedState.Balance, nil
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

	if cost, _ := act.Cost(); balance.Cmp(cost) < 0 {
		_actpoolMtc.WithLabelValues("insufficientBalance").Inc()
		sender := act.SenderAddress().String()
		actHash, _ := act.Hash()
		log.L().Info("insufficient balance for action",
			zap.String("actionHash", hex.EncodeToString(actHash[:])),
			zap.String("cost", cost.String()),
			zap.String("balance", balance.String()),
			zap.String("sender", sender),
		)
		return action.ErrInsufficientFunds
	}
	return nil
}

func (worker *queueWorker) putAction(sender string, act action.SealedEnvelope, pendingNonce uint64, confirmedBalance *big.Int) error {
	worker.mu.RLock()
	queue := worker.accountActs[sender]
	worker.mu.RUnlock()

	if queue == nil {
		queue = NewActQueue(worker.ap,
			sender,
			pendingNonce,
			confirmedBalance,
			WithTimeOut(worker.ap.cfg.ActionExpiry),
		)
		worker.mu.Lock()
		worker.accountActs[sender] = queue
		worker.mu.Unlock()
	}

	if err := queue.Put(act); err != nil {
		actHash, _ := act.Hash()
		_actpoolMtc.WithLabelValues("failedPutActQueue").Inc()
		log.L().Info("failed put action into ActQueue",
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
		sender := key.(string)
		if worker.accountActs[sender].Empty() {
			delete(worker.accountActs, sender)
		}
		return nil
	})

	worker.emptyAccounts.Reset()
}

func (worker *queueWorker) Reset(ctx context.Context) {
	worker.mu.RLock()
	defer worker.mu.RUnlock()

	for from, queue := range worker.accountActs {
		addr, _ := address.FromString(from)
		confirmedState, err := accountutil.AccountState(ctx, worker.ap.sf, addr)
		if err != nil {
			log.L().Error("Error when removing confirmed actions", zap.Error(err))
			queue.Reset()
			worker.emptyAccounts.Set(from, struct{}{})
			continue
		}
		// Remove all actions that are committed to new block
		acts := queue.UpdateAccountState(confirmedState.PendingNonce(), confirmedState.Balance)
		acts2 := queue.UpdateQueue()
		worker.ap.removeInvalidActs(append(acts, acts2...))
		// Delete the queue entry if it becomes empty
		if queue.Empty() {
			worker.emptyAccounts.Set(from, struct{}{})
		}
	}
}

// PendingActions returns all accepted actions
func (worker *queueWorker) PendingActions(ctx context.Context) []*pendingActions {
	actionArr := make([]*pendingActions, 0)

	worker.mu.RLock()
	defer worker.mu.RUnlock()
	for from, queue := range worker.accountActs {
		if queue.Empty() {
			continue
		}
		// Remove the actions that are already timeout
		acts := queue.UpdateQueue()
		worker.ap.removeInvalidActs(acts)
		actionArr = append(actionArr, &pendingActions{
			sender: from,
			acts:   queue.PendingActs(ctx),
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
func (worker *queueWorker) ResetAccount(sender address.Address) []action.SealedEnvelope {
	senderStr := sender.String()
	worker.mu.RLock()
	defer worker.mu.RUnlock()
	if queue := worker.accountActs[senderStr]; queue != nil {
		pendingActs := queue.AllActs()
		queue.Reset()
		worker.emptyAccounts.Set(senderStr, struct{}{})
		return pendingActs
	}
	return nil
}
