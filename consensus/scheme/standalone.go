// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package scheme

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// Standalone is the consensus scheme that periodically create blocks
type Standalone struct {
	task *routine.RecurringTask
}

type standaloneHandler struct {
	bc       blockchain.Blockchain
	createCb CreateBlockCB
	commitCb ConsensusDoneCB
	pubCb    BroadcastCB
}

func (s *standaloneHandler) Run() {
	blk, err := s.createCb()
	if err != nil {
		log.L().Error("Failed to create.", zap.Error(err))
		return
	}

	if err := s.commitCb(blk); err != nil {
		log.L().Error("Failed to commit.", zap.Error(err))
		return
	}
	if err := s.pubCb(blk); err != nil {
		log.L().Error("Failed to publish event.", zap.Error(err))
		return
	}
}

// NewStandalone creates a Standalone struct.
func NewStandalone(create CreateBlockCB, commit ConsensusDoneCB, pub BroadcastCB, bc blockchain.Blockchain, interval time.Duration) Scheme {
	h := &standaloneHandler{
		bc:       bc,
		createCb: create,
		commitCb: commit,
		pubCb:    pub,
	}
	return &Standalone{
		task: routine.NewRecurringTask(h.Run, interval),
	}
}

// Start starts the service for a standalone
func (n *Standalone) Start(ctx context.Context) error {
	return n.task.Start(ctx)
}

// Stop stops the service for a standalone
func (n *Standalone) Stop(ctx context.Context) error {
	return n.task.Stop(ctx)
}

// HandleConsensusMsg handles incoming consensus message
func (n *Standalone) HandleConsensusMsg(msg *iotextypes.ConsensusMessage) error {
	log.L().Warn("Noop scheme does not handle incoming block propose requests.")
	return nil
}

// Calibrate triggers an event to calibrate consensus context
func (n *Standalone) Calibrate(uint64) {}

// ValidateBlockFooter validates signatures in block footer
func (n *Standalone) ValidateBlockFooter(*block.Block) error {
	log.L().Warn("Standalone scheme always return true for block footer validation")
	return nil
}

// Metrics is not implemented for standalone scheme
func (n *Standalone) Metrics() (ConsensusMetrics, error) {
	return ConsensusMetrics{}, errors.Wrapf(
		ErrNotImplemented,
		"standalone scheme does not supported metrics yet",
	)
}
