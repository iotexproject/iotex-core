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

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-core/proto"
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

// HandleBlockPropose handles incoming block propose
func (n *Standalone) HandleBlockPropose(propose *iproto.ProposePb) error {
	logger.Warn().Msg("Noop scheme does not handle incoming block propose requests")
	return nil
}

// HandleEndorse handles incoming block propose
func (n *Standalone) HandleEndorse(endorse *iproto.EndorsePb) error {
	logger.Warn().Msg("Noop scheme does not handle incoming endorse requests")
	return nil
}

// Metrics is not implemented for standalone scheme
func (n *Standalone) Metrics() (ConsensusMetrics, error) {
	return ConsensusMetrics{}, errors.Wrapf(
		ErrNotImplemented,
		"standalone scheme does not supported metrics yet",
	)
}
