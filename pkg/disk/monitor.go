// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package disk

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/routine"
)

type monitor struct {
	task *routine.RecurringTask
}

// NewMonitor creates a timer task
func NewMonitor(internal time.Duration) *monitor {
	return &monitor{
		task: routine.NewRecurringTask(checkDiskSpace, internal),
	}
}

// Start starts timer task
func (m *monitor) Start(ctx context.Context) error {
	return m.task.Start(ctx)
}

// Stop stops timer task
func (m *monitor) Stop(ctx context.Context) error {
	return m.task.Stop(ctx)
}

func checkDiskSpace() {
	usage, err := disk.Usage("/")
	if err != nil {
		log.L().Error("Failed to get disk usage.", zap.Error(err))
		return
	}
	// panic if left less than 2%
	if usage.UsedPercent > 98.0 || usage.InodesUsedPercent > 98.0 {
		log.L().Fatal("No space in device.", zap.Float64("UsedPercent", usage.UsedPercent), zap.Float64("InodesUsedPercent", usage.InodesUsedPercent))
	}
}
