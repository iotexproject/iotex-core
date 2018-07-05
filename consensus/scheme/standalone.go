// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package scheme

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/errcode"
	"github.com/iotexproject/iotex-core/pkg/routine"
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
	logger.Info().
		Str("at", time.Now().String()).
		Msg("created a new block")
	blk, err := s.createCb()
	if err != nil {
		logger.Error().Err(err)
		return
	}

	if err := s.commitCb(blk); err != nil {
		logger.Error().Err(err)
		return
	}

	s.pubCb(blk)
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

// SetDoneStream does nothing in Standalone (only used in simulator)
func (n *Standalone) SetDoneStream(done chan bool) {}

// Stop stops the service for a standalone
func (n *Standalone) Stop(ctx context.Context) error {
	return n.task.Stop(ctx)
}

// Handle handles incoming requests
func (n *Standalone) Handle(message proto.Message) error {
	logger.Warn().Msg("Standalone scheme does not handle incoming requests")
	return nil
}

// Metrics is not implemented for standalone scheme
func (n *Standalone) Metrics() (ConsensusMetrics, error) {
	return ConsensusMetrics{}, errors.Wrapf(
		errcode.ErrNotImplemented,
		"standalone scheme does not supported metrics yet",
	)
}
