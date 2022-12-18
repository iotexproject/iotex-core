// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package consensusfsm

import "github.com/iotexproject/iotex-core/endorsement"

// EndorsedMessage defines a message with endorsement
type EndorsedMessage interface {
	Endorsement() *endorsement.Endorsement
	Message() interface{}
}
