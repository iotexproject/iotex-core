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

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

// Dispatcher is used by peers, handles incoming block and header notifications and relays announcements of new blocks.
type Dispatcher interface {
	lifecycle.StartStopper

	// HandleBroadcast handles the incoming broadcast message. The transportation layer semantics is at least once.
	// That said, the handler is likely to receive duplicate messages.
	HandleBroadcast(proto.Message, chan bool)
	// HandleTell handles the incoming tell message. The transportation layer semantics is exact once. The sender is
	// given for the sake of replying the message
	HandleTell(net.Addr, proto.Message, chan bool)
}
