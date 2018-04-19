// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package common

import (
	"net"

	"github.com/golang/protobuf/proto"
)

// Dispatcher is used by peers, handles incoming block and header notifications and relays announcements of new blocks.
type Dispatcher interface {
	// Start starts a dispatcher
	Start() error
	// Stop stops a dispatcher
	Stop() error
	// HandleBroadcast handles the incoming broadcast message. The transportation layer semantics is at least once.
	// That said, the handler is likely to receive duplicate messages.
	HandleBroadcast(proto.Message, chan bool)
	// HandleTell handles the incoming tell message. The transportation layer semantics is exact once. The sender is
	// given for the sake of replying the message
	HandleTell(net.Addr, proto.Message, chan bool)
}
