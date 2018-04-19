// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package scheme

import (
	"time"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/common/routine"
)

// Standalone is the consensus scheme that periodically create blocks
type Standalone struct {
	task *routine.RecurringTask
}

type standaloneHandler struct {
	bc       blockchain.IBlockchain
	createCb CreateBlockCB
	commitCb ConsensusDoneCB
	pubCb    BroadcastCB
}

func (s *standaloneHandler) Do() {
	glog.Info("create a new block at ", time.Now())
	blk, err := s.createCb()
	if err != nil {
		glog.Error(err)
		return
	}

	if err := s.commitCb(blk); err != nil {
		glog.Error(err)
		return
	}

	s.pubCb(blk)
}

// NewStandalone creates a Standalone struct.
func NewStandalone(create CreateBlockCB, commit ConsensusDoneCB, pub BroadcastCB, bc blockchain.IBlockchain, interval time.Duration) Scheme {
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

// Stop stops the service for a standalone
func (n *Standalone) Stop() error {
	n.task.Stop()
	return nil
}

// Handle handles incoming requests
func (n *Standalone) Handle(message proto.Message) error {
	glog.Warning("Standalone scheme does not handle incoming requests")
	return nil
}
