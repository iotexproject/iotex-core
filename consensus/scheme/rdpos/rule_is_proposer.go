// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rdpos

import (
	"fmt"

	"github.com/iotexproject/iotex-core/consensus/fsm"
)

// ruleIsProposer checks if the event is init propose.
type ruleIsProposer struct {
	*RDPoS
}

func (r ruleIsProposer) Condition(event *fsm.Event) bool {
	fmt.Printf("ruleIsProposer output: %t\n", event.State == stateInitPropose)
	return event.State == stateInitPropose
}
