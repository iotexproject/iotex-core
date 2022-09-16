// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package watch

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/routine"
)

// Start creates a timer task to check device per watchInternal
func Start(ctx context.Context, watchInternal time.Duration) func() {
	task := routine.NewRecurringTask(checkDiskSpace, watchInternal)
	if err := task.Start(ctx); err != nil {
		log.L().Panic("Failed to start watch disk space", zap.Error(err))
	}
	return func() {
		if err := task.Stop(ctx); err != nil {
			log.L().Panic("Failed to stop watch disk space.", zap.Error(err))
		}
	}
}

func checkDiskSpace() {
	usage, err := disk.Usage("/")
	if err != nil {
		log.L().Error("Failed to get disk usage.", zap.Error(err))
		return
	}
	// panic if left less than 2%
	if usage.UsedPercent > 98.0 || usage.InodesUsedPercent > 98.0 {
		log.L().Fatal("No space in device, pls clean up.", zap.Float64("UsedPercent", usage.UsedPercent), zap.Float64("InodesUsedPercent", usage.InodesUsedPercent))
	}
}
