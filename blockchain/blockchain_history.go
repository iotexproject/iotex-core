// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"math/big"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/facebookgo/clock"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

// blockchainHistory implements the Blockchain interface
type blockchainHistory struct {
	*blockchain
	sfHistory factory.Factory // full-state history
}

// NewBlockchainHistory creates a new blockchain and DB instance
func NewBlockchainHistory(cfg config.Config, dao blockdao.BlockDAO, opts ...Option) Blockchain {
	// create the Blockchain
	chain := &blockchainHistory{
		blockchain: &blockchain{
			config: cfg,
			dao:    dao,
			clk:    clock.New(),
		},
	}
	for _, opt := range opts {
		if err := opt(chain.blockchain, cfg); err != nil {
			log.S().Panicf("Failed to execute blockchain creation option %p: %v", opt, err)
		}
	}
	// create full-state history DB
	var err error
	if cfg.Chain.EnableTrielessStateDB {
		chain.sfHistory, err = factory.NewStateDB(cfg, factory.DefaultHistoryDBOption())
	} else {
		chain.sfHistory, err = factory.NewFactory(cfg, factory.DefaultHistoryTrieOption())
	}
	if err != nil {
		log.L().Panic("Failed to create state factory.", zap.Error(err))
	}
	timerFactory, err := prometheustimer.New(
		"iotex_blockchain_perf",
		"Performance of blockchain module",
		[]string{"topic", "chainID"},
		[]string{"default", strconv.FormatUint(uint64(cfg.Chain.ID), 10)},
	)
	if err != nil {
		log.L().Panic("Failed to generate prometheus timer factory.", zap.Error(err))
	}
	chain.timerFactory = timerFactory
	senderBlackList := make(map[string]bool)
	for _, bannedSender := range cfg.ActPool.BlackList {
		senderBlackList[bannedSender] = true
	}
	chain.validator = &validator{
		sf:              chain.sf,
		validatorAddr:   cfg.ProducerAddress().String(),
		senderBlackList: senderBlackList,
	}

	if chain.dao != nil {
		chain.lifecycle.Add(chain.dao)
	}
	if chain.sf != nil {
		chain.lifecycle.Add(chain.sf)
	}
	if chain.sfHistory != nil {
		chain.lifecycle.Add(chain.sfHistory)
	}
	return chain
}

// CreateState adds a new account with initial balance to the factory
func (bc *blockchainHistory) CreateState(addr string, init *big.Int) (*state.Account, error) {
	account, err := bc.blockchain.CreateState(addr, init)
	if err != nil {
		return nil, err
	}
	ws2, err := bc.sfHistory.NewWorkingSet(true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create clean working set")
	}
	_, err = accountutil.LoadOrCreateAccount(ws2, addr, init)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new account %s", addr)
	}
	gasLimit := bc.config.Genesis.BlockGasLimit
	callerAddr, err := address.FromString(addr)
	if err != nil {
		return nil, err
	}
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			GasLimit:   gasLimit,
			Caller:     callerAddr,
			ActionHash: hash.ZeroHash256,
			Nonce:      0,
			Registry:   bc.registry,
			History:    true,
		})
	if _, err = ws2.RunActions(ctx, 0, nil); err != nil {
		return nil, errors.Wrap(err, "failed to run the account creation")
	}
	if err = bc.sfHistory.Commit(ws2); err != nil {
		return nil, errors.Wrap(err, "failed to commit the account creation")
	}
	return account, nil
}

// Start starts the blockchain
func (bc *blockchainHistory) Start(ctx context.Context) (err error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if err = bc.lifecycle.OnStart(ctx); err != nil {
		return err
	}
	// sfHistory only deal with account and contract
	if bc.sfHistory != nil {
		p, ok := bc.registry.Find(account.ProtocolID)
		if !ok {
			return errors.New("can not find account protocol")
		}
		bc.sfHistory.AddActionHandlers(p)
		p, ok = bc.registry.Find(execution.ProtocolID)
		if !ok {
			return errors.New("can not find execution protocol")
		}
		bc.sfHistory.AddActionHandlers(p)
	}

	// get blockchain tip height
	if bc.tipHeight, err = bc.dao.GetTipHeight(); err != nil {
		return err
	}
	if bc.tipHeight == 0 {
		return bc.startEmptyBlockchain()
	}
	// get blockchain tip hash
	if bc.tipHash, err = bc.dao.GetTipHash(); err != nil {
		return err
	}
	return bc.startExistingBlockchain()
}

//  CommitBlock validates and appends a block to the chain
func (bc *blockchainHistory) CommitBlock(blk *block.Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	timer := bc.timerFactory.NewTimer("CommitBlock")
	defer timer.End()

	return bc.commitBlock(blk)
}

func (bc *blockchainHistory) startEmptyBlockchain() error {
	if err := bc.blockchain.startEmptyBlockchain(); err != nil {
		return err
	}
	var ws factory.WorkingSet
	var err error
	if ws, err = bc.sfHistory.NewWorkingSet(true); err != nil {
		return errors.Wrap(err, "failed to obtain working set from state factory")
	}
	if !bc.config.Chain.EmptyGenesis {
		// Initialize the states before any actions happen on the blockchain
		if err := bc.createGenesisStates(ws); err != nil {
			return err
		}
		_ = ws.UpdateBlockLevelInfo(0)
	}
	// add Genesis states
	if err := bc.sfHistory.Commit(ws); err != nil {
		return errors.Wrap(err, "failed to commit Genesis states")
	}
	return nil
}

func (bc *blockchainHistory) startExistingBlockchain() error {
	if err := bc.blockchain.startExistingBlockchain(); err != nil {
		return err
	}
	if bc.sfHistory == nil {
		return errors.New("statefactory cannot be nil")
	}

	stateHeight, err := bc.sfHistory.Height()
	if err != nil {
		return err
	}
	if stateHeight > bc.tipHeight {
		return errors.New("factory is higher than blockchain")
	}

	for i := stateHeight + 1; i <= bc.tipHeight; i++ {
		blk, err := bc.getBlockByHeight(i)
		if err != nil {
			return err
		}

		ws, err := bc.sfHistory.NewWorkingSet(true)
		if err != nil {
			return errors.Wrap(err, "failed to obtain working set from state factory")
		}
		if _, err := bc.runActions(blk.RunnableActions(), ws); err != nil {
			return err
		}

		if err := bc.sfHistory.Commit(ws); err != nil {
			return err
		}
	}
	return nil
}

func (bc *blockchainHistory) commitBlock(blk *block.Block) error {
	// early exit if block already exists
	blkHash, err := bc.dao.GetBlockHash(blk.Height())
	if err == nil && blkHash != hash.ZeroHash256 {
		log.L().Debug("Block already exists.", zap.Uint64("height", blk.Height()))
		return nil
	}
	// early exit if it's a db io error
	if err != nil && errors.Cause(err) != db.ErrNotExist && errors.Cause(err) != db.ErrBucketNotExist {
		return err
	}
	// write block into DB
	putTimer := bc.timerFactory.NewTimer("putBlock")
	if err = bc.dao.PutBlock(blk); err == nil {
		err = bc.dao.Commit()
	}
	putTimer.End()
	if err != nil {
		return err
	}

	// update tip hash and height
	atomic.StoreUint64(&bc.tipHeight, blk.Height())
	bc.tipHash = blk.HashBlock()

	// commit state/contract changes
	sfTimer := bc.timerFactory.NewTimer("sf.Commit")
	err = bc.sf.Commit(blk.WorkingSet)
	sfTimer.End()
	// detach working set so it can be freed by GC
	blk.WorkingSet = nil
	if err != nil {
		log.L().Panic("Error when committing states.", zap.Error(err))
	}
	blk.HeaderLogger(log.L()).Info("Committed a block.", log.Hex("tipHash", bc.tipHash[:]))

	// emit block to all block subscribers
	bc.emitToSubscribers(blk)

	// run actions with history retention
	ws, err := bc.sfHistory.NewWorkingSet(true)
	if err != nil {
		return errors.Wrap(err, "Failed to obtain working set from state factory")
	}
	if _, err := bc.runActions(blk.RunnableActions(), ws); err != nil {
		log.L().Error("Failed to update state.", zap.Uint64("tipHeight", bc.tipHeight), zap.Error(err))
	}
	if err = bc.sfHistory.Commit(ws); err != nil {
		log.L().Error("Error when committing states with history.", zap.Error(err))
	}
	return nil
}

// RefreshStateDB deletes the existing state DB and creates a new one with state changes from genesis block
func (bc *blockchainHistory) refreshStateDB() error {
	var err error
	if bc.config.Chain.EnableTrielessStateDB {
		bc.sfHistory, err = factory.NewStateDB(bc.config, factory.DefaultHistoryDBOption())
	} else {
		bc.sfHistory, err = factory.NewFactory(bc.config, factory.DefaultHistoryTrieOption())
	}
	if err != nil {
		log.L().Panic("Failed to create state factory.", zap.Error(err))
	}
	p, ok := bc.registry.Find(account.ProtocolID)
	if !ok {
		return errors.New("can not find account protocol")
	}
	bc.sfHistory.AddActionHandlers(p)
	p, ok = bc.registry.Find(execution.ProtocolID)
	if !ok {
		return errors.New("can not find execution protocol")
	}
	bc.sfHistory.AddActionHandlers(p)

	// Delete existing state DB and reinitialize it
	if fileutil.FileExists(bc.config.Chain.TrieDBPath) && os.Remove(bc.config.Chain.TrieDBPath) != nil {
		return errors.New("failed to delete existing state DB")
	}
	if fileutil.FileExists(bc.config.Chain.HistoryDBPath) && os.Remove(bc.config.Chain.HistoryDBPath) != nil {
		return errors.New("failed to delete existing state DB 2")
	}
	if err := DefaultStateFactoryOption()(bc.blockchain, bc.config); err != nil {
		return errors.Wrap(err, "failed to reinitialize state DB")
	}

	for _, p := range bc.registry.All() {
		bc.sf.AddActionHandlers(p)
	}

	if err := bc.sf.Start(context.Background()); err != nil {
		return errors.Wrap(err, "failed to start state factory")
	}
	if err := bc.sfHistory.Start(context.Background()); err != nil {
		return errors.Wrap(err, "failed to start state factory")
	}
	if err := bc.startEmptyBlockchain(); err != nil {
		return err
	}
	if err := bc.sf.Stop(context.Background()); err != nil {
		return errors.Wrap(err, "failed to stop state factory")
	}
	if err := bc.sfHistory.Stop(context.Background()); err != nil {
		return errors.Wrap(err, "failed to stop state factory")
	}
	return nil
}
