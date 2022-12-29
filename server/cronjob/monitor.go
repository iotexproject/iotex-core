// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package cronjob

import (
	"context"
	"time"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// CronJob interface
type CronJob interface {
	Run()
	Interval() time.Duration
}

type cronInterval time.Duration

func (i cronInterval) Interval() time.Duration {
	return time.Duration(i)
}

type monitorJob struct {
	cronInterval
	agent p2p.Agent
	bc    blockchain.Blockchain
}

// NewMonitorCronjob new monitor cronjob
func NewMonitorCronjob(agent p2p.Agent, bc blockchain.Blockchain, interval time.Duration) CronJob {
	return &monitorJob{
		cronInterval: cronInterval(interval),
		agent:        agent,
		bc:           bc,
	}
}

func (j *monitorJob) Run() {
	// ver := version.PackageVersion
	j.agent.BroadcastOutbound(context.Background(), &iotextypes.ConsensusMessage{
		Height: j.bc.TipHeight(),
	})
}

// NewCronJobs create cronjobs
func NewCronJobs(cfg Config, agent p2p.Agent, bc blockchain.Blockchain) []CronJob {
	res := []CronJob{}
	if cfg.MonitorInterval > 0 {
		res = append(res, NewMonitorCronjob(agent, bc, cfg.MonitorInterval))
	}
	return res
}
