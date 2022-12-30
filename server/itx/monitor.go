// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

type MonitorBroadcaster struct {
	agent p2p.Agent
	bc    blockchain.Blockchain
}

// NewMonitorBroadcaster new monitor broadcaster
func NewMonitorBroadcaster(agent p2p.Agent, bc blockchain.Blockchain) *MonitorBroadcaster {
	return &MonitorBroadcaster{
		agent: agent,
		bc:    bc,
	}
}

// Broadcast broadcast monitor msg
func (j *MonitorBroadcaster) Broadcast() {
	j.agent.BroadcastOutbound(context.Background(), &iotextypes.Monitor{
		Version: version.PackageVersion,
		Height:  j.bc.TipHeight(),
	})
}
