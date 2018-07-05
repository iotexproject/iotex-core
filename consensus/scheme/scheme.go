// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package scheme

import (
	"net"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

// CreateBlockCB defines the callback to create a new block
type CreateBlockCB func() (*blockchain.Block, error)

// TellPeerCB defines the callback to tell (which is a unicast) message to peers on P2P network
type TellPeerCB func(proto.Message) error

// ConsensusDoneCB defines the callback when consensus is reached
type ConsensusDoneCB func(*blockchain.Block) error

// BroadcastCB defines the callback to publish the consensus result
type BroadcastCB func(*blockchain.Block) error

// GetProposerCB defines the callback to check the if itself is the the proposer for the coming round
type GetProposerCB func([]net.Addr, []byte, uint64, uint64) (net.Addr, error)

// GenerateDKGCB defines the callback to generate DKG bytes
type GenerateDKGCB func() (hash.DKGHash, error)

// StartNextEpochCB defines the callback to check if the next epoch should start
type StartNextEpochCB func(net.Addr, uint64, delegate.Pool) (bool, error)

// Scheme is the interface that consensus schemes should implement
type Scheme interface {
	lifecycle.StartStopper

	Handle(msg proto.Message) error
	SetDoneStream(chan bool)
	Metrics() (ConsensusMetrics, error)
}

// ConsensusMetrics contains consensus metrics to expose
type ConsensusMetrics struct {
	LatestEpoch         uint64
	LatestDelegates     []net.Addr
	LatestBlockProducer net.Addr
	Candidates          []net.Addr
}
