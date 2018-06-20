// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package scheme

import (
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/routine"
	"github.com/iotexproject/iotex-core/logger"
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

func (s *standaloneHandler) Do() {
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
	return &Standalone{
		task: routine.NewRecurringTask(
			&standaloneHandler{bc: bc, createCb: create, commitCb: commit, pubCb: pub}, interval),
	}
}

// Start starts the service for a standalone
func (n *Standalone) Start() error {
	n.task.Init()
	n.task.Start()

	return nil
}

// SetDoneStream does nothing in Standalone (only used in simulator)
func (n *Standalone) SetDoneStream(done chan bool) {}

// Stop stops the service for a standalone
func (n *Standalone) Stop() error {
	n.task.Stop()
	return nil
}

// Handle handles incoming requests
func (n *Standalone) Handle(message proto.Message) error {
	logger.Warn().Msg("Standalone scheme does not handle incoming requests")
	return nil
}

// Metrics is not implemented for standalone scheme
func (n *Standalone) Metrics() (ConsensusMetrics, error) {
	return ConsensusMetrics{}, errors.Wrapf(
		common.ErrNotImplemented,
		"standalone scheme does not supported metrics yet",
	)
}
