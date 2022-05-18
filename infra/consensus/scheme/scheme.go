// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package scheme

import (
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// CreateBlockCB defines the callback to create a new block
type CreateBlockCB func() (*block.Block, error)

// TellPeerCB defines the callback to tell (which is a unicast) message to peers on P2P network
type TellPeerCB func(proto.Message) error

// ConsensusDoneCB defines the callback when consensus is reached
type ConsensusDoneCB func(*block.Block) error

// BroadcastCB defines the callback to publish the consensus result
type BroadcastCB func(*block.Block) error

// Broadcast sends a broadcast message to the whole network
type Broadcast func(msg proto.Message) error

// Scheme is the interface that consensus schemes should implement
type Scheme interface {
	lifecycle.StartStopper

	HandleConsensusMsg(msg *iotextypes.ConsensusMessage) error
	Calibrate(uint64)
	ValidateBlockFooter(*block.Block) error
	Metrics() (ConsensusMetrics, error)
	Activate(bool)
	Active() bool
}

// ConsensusMetrics contains consensus metrics to expose
type ConsensusMetrics struct {
	LatestEpoch         uint64
	LatestHeight        uint64
	LatestDelegates     []string
	LatestBlockProducer string
}
