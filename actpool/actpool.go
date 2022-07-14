// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"context"
	"encoding/hex"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/cache/ttl"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/tracer"
)

const (
	// move to config
	_numWorker = 64
)

var (
	_actpoolMtc = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "iotex_actpool_rejection_metrics",
		Help: "actpool metrics.",
	}, []string{"type"})
)

func init() {
	prometheus.MustRegister(_actpoolMtc)
}

// ActPool is the interface of actpool
type ActPool interface {
	action.SealedEnvelopeValidator
	// Reset resets actpool state
	Reset()
	// PendingActionMap returns an action map with all accepted actions
	PendingActionMap() map[string][]action.SealedEnvelope
	// Add adds an action into the pool after passing validation
	Add(ctx context.Context, act action.SealedEnvelope) error
	// GetPendingNonce returns pending nonce in pool given an account address
	GetPendingNonce(addr string) (uint64, error)
	// GetUnconfirmedActs returns unconfirmed actions in pool given an account address
	GetUnconfirmedActs(addr string) []action.SealedEnvelope
	// GetActionByHash returns the pending action in pool given action's hash
	GetActionByHash(hash hash.Hash256) (action.SealedEnvelope, error)
	// GetSize returns the act pool size
	GetSize() uint64
	// GetCapacity returns the act pool capacity
	GetCapacity() uint64
	// GetGasSize returns the act pool gas size
	GetGasSize() uint64
	// GetGasCapacity returns the act pool gas capacity
	GetGasCapacity() uint64
	// DeleteAction deletes an invalid action from pool
	DeleteAction(address.Address)
	// ReceiveBlock will be called when a new block is committed
	ReceiveBlock(*block.Block) error

	AddActionEnvelopeValidators(...action.SealedEnvelopeValidator)
}

// SortedActions is a slice of actions that implements sort.Interface to sort by Value.
type SortedActions []action.SealedEnvelope

func (p SortedActions) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p SortedActions) Len() int           { return len(p) }
func (p SortedActions) Less(i, j int) bool { return p[i].Nonce() < p[j].Nonce() }

// Option sets action pool construction parameter
type Option func(pool *actPool) error

// EnableExperimentalActions enables the action pool to take experimental actions
func EnableExperimentalActions() Option {
	return func(pool *actPool) error {
		pool.enableExperimentalActions = true
		return nil
	}
}

// actPool implements ActPool interface
type actPool struct {
	cfg                       Config
	g                         genesis.Genesis
	sf                        protocol.StateReader
	accountDesActs            *desActs
	allActions                *ttl.Cache
	gasInPool                 uint64
	actionEnvelopeValidators  []action.SealedEnvelopeValidator
	timerFactory              *prometheustimer.TimerFactory
	enableExperimentalActions bool
	senderBlackList           map[string]bool
	jobQueue                  []chan workerJob
	worker                    []*queueWorker
}

type desActs struct {
	mu   sync.Mutex
	acts map[string]map[hash.Hash256]action.SealedEnvelope
}

// NewActPool constructs a new actpool
func NewActPool(g genesis.Genesis, sf protocol.StateReader, cfg Config, opts ...Option) (ActPool, error) {
	if sf == nil {
		return nil, errors.New("Try to attach a nil state reader")
	}

	senderBlackList := make(map[string]bool)
	for _, bannedSender := range cfg.BlackList {
		senderBlackList[bannedSender] = true
	}

	actsMap, _ := ttl.NewCache()
	ap := &actPool{
		cfg:             cfg,
		g:               g,
		sf:              sf,
		senderBlackList: senderBlackList,
		accountDesActs:  &desActs{acts: make(map[string]map[hash.Hash256]action.SealedEnvelope)},
		allActions:      actsMap,
		jobQueue:        make([]chan workerJob, _numWorker),
		worker:          make([]*queueWorker, _numWorker),
	}
	for _, opt := range opts {
		if err := opt(ap); err != nil {
			return nil, err
		}
	}
	timerFactory, err := prometheustimer.New(
		"iotex_action_pool_perf",
		"Performance of action pool",
		[]string{"type"},
		[]string{"default"},
	)
	if err != nil {
		return nil, err
	}
	ap.timerFactory = timerFactory

	for i := 0; i < _numWorker; i++ {
		ap.jobQueue[i] = make(chan workerJob, ap.cfg.MaxNumActsPerAcct)
		ap.worker[i] = newQueueWorker(ap, ap.jobQueue[i])
		if err := ap.worker[i].Start(); err != nil {
			return nil, err
		}
	}
	return ap, nil
}

// TODO: add start() and stop() in actpool
// func (ap *actPool) Start() {
// }

// func (ap *actPool) Stop() {
// }

func (ap *actPool) AddActionEnvelopeValidators(fs ...action.SealedEnvelopeValidator) {
	ap.actionEnvelopeValidators = append(ap.actionEnvelopeValidators, fs...)
}

// Reset resets actpool state
// Step I: remove all the actions in actpool that have already been committed to block
// Step II: update pending balance of each account if it still exists in pool
// Step III: update queue's status in each account and remove invalid actions following queue's update
// Specifically, first reset the pending nonce based on confirmed nonce in order to prevent omitting reevaluation of
// unconfirmed but pending actions in pool after update of pending balance
// Then starting from the current confirmed nonce, iteratively update pending nonce if nonces are consecutive and pending
// balance is sufficient, and remove all the subsequent actions once the pending balance becomes insufficient
func (ap *actPool) Reset() {
	ap.reset()
}

func (ap *actPool) reset() {
	var (
		wg  sync.WaitGroup
		ctx = ap.context(context.Background())
	)
	for i := range ap.worker {
		wg.Add(1)
		go func(worker *queueWorker) {
			defer wg.Done()
			worker.Reset(ctx)
		}(ap.worker[i])
	}
	wg.Wait()
}

func (ap *actPool) ReceiveBlock(*block.Block) error {
	ap.reset()
	return nil
}

// PendingActionMap returns an action interator with all accepted actions
func (ap *actPool) PendingActionMap() map[string][]action.SealedEnvelope {
	var (
		wg             sync.WaitGroup
		actsFromWorker = make([][]*pendingActions, _numWorker)
		ctx            = ap.context(context.Background())
		totalAccounts  = uint64(0)
	)
	for i := range ap.worker {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			actsFromWorker[i] = ap.worker[i].PendingActions(ctx)
			atomic.AddUint64(&totalAccounts, uint64(len(actsFromWorker[i])))
		}(i)
	}
	wg.Wait()

	var pendingActs uint64 = 0
	ret := make(map[string][]action.SealedEnvelope, totalAccounts)
	for _, v := range actsFromWorker {
		for _, w := range v {
			ret[w.sender] = w.acts
			pendingActs += uint64(len(w.acts))
		}
	}

	log.L().Info("DebugBenchmark", zap.Uint64("pendingActs", pendingActs))
	return ret
}

func (ap *actPool) Add(ctx context.Context, act action.SealedEnvelope) error {
	ctx, span := tracer.NewSpan(ap.context(ctx), "actPool.Add")
	defer span.End()
	ctx = ap.context(ctx)

	if err := checkSelpData(&act); err != nil {
		return err
	}

	if err := ap.checkSelpWithoutState(ctx, &act); err != nil {
		return err
	}

	// Reject action if pool space is full

	if uint64(ap.allActions.Count()) >= ap.cfg.MaxNumActsPerPool {
		_actpoolMtc.WithLabelValues("overMaxNumActsPerPool").Inc()
		return action.ErrTxPoolOverflow
	}

	if intrinsicGas, _ := act.IntrinsicGas(); atomic.LoadUint64(&ap.gasInPool)+intrinsicGas > ap.cfg.MaxGasLimitPerPool {
		_actpoolMtc.WithLabelValues("overMaxGasLimitPerPool").Inc()
		return action.ErrGasLimit
	}

	return ap.enqueue(ctx, act)
}

func checkSelpData(act *action.SealedEnvelope) error {
	_, err := act.IntrinsicGas()
	if err != nil {
		return err
	}
	_, err = act.Hash()
	if err != nil {
		return err
	}
	_, err = act.Cost()
	if err != nil {
		return err
	}
	if act.SrcPubkey() == nil {
		return action.ErrAddress
	}
	return nil
}

func (ap *actPool) checkSelpWithoutState(ctx context.Context, selp *action.SealedEnvelope) error {
	span := tracer.SpanFromContext(ctx)
	span.AddEvent("actPool.checkSelpWithoutState")
	defer span.End()

	hash, _ := selp.Hash()
	// Reject action if it already exists in pool
	if _, exist := ap.allActions.Get(hash); exist {
		_actpoolMtc.WithLabelValues("existedAction").Inc()
		return action.ErrExistedInPool
	}

	// Reject action if the gas price is lower than the threshold
	if selp.GasPrice().Cmp(ap.cfg.MinGasPrice()) < 0 {
		_actpoolMtc.WithLabelValues("gasPriceLower").Inc()
		actHash, _ := selp.Hash()
		log.L().Info("action rejected due to low gas price",
			zap.String("actionHash", hex.EncodeToString(actHash[:])),
			zap.String("gasPrice", selp.GasPrice().String()))
		return action.ErrUnderpriced
	}

	if _, ok := ap.senderBlackList[selp.SenderAddress().String()]; ok {
		_actpoolMtc.WithLabelValues("blacklisted").Inc()
		return errors.Wrap(action.ErrAddress, "action source address is blacklisted")
	}

	for _, ev := range ap.actionEnvelopeValidators {
		span.AddEvent("ev.Validate")
		if err := ev.Validate(ctx, *selp); err != nil {
			return err
		}
	}
	return nil
}

// GetPendingNonce returns pending nonce in pool or confirmed nonce given an account address
func (ap *actPool) GetPendingNonce(addrStr string) (uint64, error) {
	addr, err := address.FromString(addrStr)
	if err != nil {
		return 0, err
	}
	if queue := ap.worker[ap.allocatedWorker(addr)].GetQueue(addr); queue != nil {
		return queue.PendingNonce(), nil
	}
	ctx := ap.context(context.Background())
	confirmedState, err := accountutil.AccountState(ctx, ap.sf, addr)
	if err != nil {
		return 0, err
	}
	return confirmedState.PendingNonce(), err
}

// GetUnconfirmedActs returns unconfirmed actions in pool given an account address
func (ap *actPool) GetUnconfirmedActs(addrStr string) []action.SealedEnvelope {
	addr, err := address.FromString(addrStr)
	if err != nil {
		return []action.SealedEnvelope{}
	}

	var ret []action.SealedEnvelope
	if queue := ap.worker[ap.allocatedWorker(addr)].GetQueue(addr); queue != nil {
		ret = queue.AllActs()
	}
	ap.accountDesActs.mu.Lock()
	defer ap.accountDesActs.mu.Unlock()
	if desMap, ok := ap.accountDesActs.acts[addrStr]; ok {
		if desMap != nil {
			sortActions := make(SortedActions, 0)
			for _, v := range desMap {
				sortActions = append(sortActions, v)
			}
			sort.Stable(sortActions)
			ret = append(ret, sortActions...)
		}
	}
	return ret
}

// GetActionByHash returns the pending action in pool given action's hash
func (ap *actPool) GetActionByHash(hash hash.Hash256) (action.SealedEnvelope, error) {
	act, ok := ap.allActions.Get(hash)
	if !ok {
		return action.SealedEnvelope{}, errors.Wrapf(action.ErrNotFound, "action hash %x does not exist in pool", hash)
	}
	return act.(action.SealedEnvelope), nil
}

// GetSize returns the act pool size
func (ap *actPool) GetSize() uint64 {
	return uint64(ap.allActions.Count())
}

// GetCapacity returns the act pool capacity
func (ap *actPool) GetCapacity() uint64 {
	return ap.cfg.MaxNumActsPerPool
}

// GetGasSize returns the act pool gas size
func (ap *actPool) GetGasSize() uint64 {
	return atomic.LoadUint64(&ap.gasInPool)
}

// GetGasCapacity returns the act pool gas capacity
func (ap *actPool) GetGasCapacity() uint64 {
	return ap.cfg.MaxGasLimitPerPool
}

func (ap *actPool) Validate(ctx context.Context, selp action.SealedEnvelope) error {
	return ap.validate(ctx, selp)
}

func (ap *actPool) DeleteAction(caller address.Address) {
	worker := ap.worker[ap.allocatedWorker(caller)]
	if queue := worker.GetQueue(caller); queue != nil {
		pendingActs := queue.AllActs()
		ap.removeInvalidActs(pendingActs)
		worker.ResetAccount(caller)
	}
}

func (ap *actPool) validate(ctx context.Context, selp action.SealedEnvelope) error {
	span := tracer.SpanFromContext(ctx)
	span.AddEvent("actPool.validate")
	defer span.End()

	caller := selp.SenderAddress()
	if caller == nil {
		return errors.New("failed to get address")
	}
	if _, ok := ap.senderBlackList[caller.String()]; ok {
		_actpoolMtc.WithLabelValues("blacklisted").Inc()
		return errors.Wrap(action.ErrAddress, "action source address is blacklisted")
	}
	// if already validated
	selpHash, err := selp.Hash()
	if err != nil {
		return err
	}
	if _, ok := ap.allActions.Get(selpHash); ok {
		return nil
	}
	for _, ev := range ap.actionEnvelopeValidators {
		span.AddEvent("ev.Validate")
		if err := ev.Validate(ctx, selp); err != nil {
			return err
		}
	}

	return nil
}

func (ap *actPool) removeInvalidActs(acts []action.SealedEnvelope) {
	for _, act := range acts {
		hash, err := act.Hash()
		if err != nil {
			log.L().Debug("Skipping action due to hash error", zap.Error(err))
			continue
		}
		log.L().Debug("Removed invalidated action.", log.Hex("hash", hash[:]))
		ap.allActions.Delete(hash)
		intrinsicGas, _ := act.IntrinsicGas()
		atomic.AddUint64(&ap.gasInPool, ^uint64(intrinsicGas-1))
		//del actions in destination map
		ap.deleteAccountDestinationActions(act)
	}
}

// deleteAccountDestinationActions just for destination map
func (ap *actPool) deleteAccountDestinationActions(acts ...action.SealedEnvelope) {
	ap.accountDesActs.mu.Lock()
	defer ap.accountDesActs.mu.Unlock()
	for _, act := range acts {
		hash, err := act.Hash()
		if err != nil {
			log.L().Debug("Skipping action due to hash error", zap.Error(err))
			continue
		}
		if desAddress, ok := act.Destination(); ok {
			dst := ap.accountDesActs.acts[desAddress]
			if dst != nil {
				delete(dst, hash)
			}
			if len(dst) == 0 {
				delete(ap.accountDesActs.acts, desAddress)
			}
		}
	}
}

func (ap *actPool) context(ctx context.Context) context.Context {
	return genesis.WithGenesisContext(ctx, ap.g)
}

func (ap *actPool) enqueue(ctx context.Context, act action.SealedEnvelope) error {
	var errChan = make(chan error) // unused errChan will be garbage-collected
	ap.jobQueue[ap.allocatedWorker(act.SenderAddress())] <- workerJob{ctx, act, errChan}

	for {
		select {
		case <-ctx.Done():
			log.L().Error("enqueue actpool fails", zap.Error(ctx.Err()))
			return ctx.Err()
		case ret := <-errChan:
			return ret
		}
	}
}

func (ap *actPool) allocatedWorker(senderAddr address.Address) int {
	senderBytes := senderAddr.Bytes()
	var lastByte uint8 = senderBytes[len(senderBytes)-1]
	return int(lastByte) % _numWorker
}
