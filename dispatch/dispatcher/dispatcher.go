// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// Package dispatcher defines Dispatcher interface which is used to dispatching incoming request and event. The main reason to pick this as an independent package is to resolve cyclic dependency.
package dispatcher

import (
	"net"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	pb "github.com/iotexproject/iotex-core/proto"
)

// Subscriber is the dispatcher subscriber interface
type Subscriber interface {
	HandleAction(*pb.ActionPb) error
	HandleBlock(*blockchain.Block) error
	HandleBlockSync(*blockchain.Block) error
	HandleSyncRequest(string, *pb.BlockSync) error
	HandleViewChange(proto.Message) error
	HandleBlockPropose(proto.Message) error
}

// Dispatcher is used by peers, handles incoming block and header notifications and relays announcements of new blocks.
type Dispatcher interface {
	lifecycle.StartStopper

	// AddSubscriber adds to dispatcher
	AddSubscriber(uint32, Subscriber)
	// HandleBroadcast handles the incoming broadcast message. The transportation layer semantics is at least once.
	// That said, the handler is likely to receive duplicate messages.
	HandleBroadcast(uint32, proto.Message, chan bool)
	// HandleTell handles the incoming tell message. The transportation layer semantics is exact once. The sender is
	// given for the sake of replying the message
	HandleTell(uint32, net.Addr, proto.Message, chan bool)
}
