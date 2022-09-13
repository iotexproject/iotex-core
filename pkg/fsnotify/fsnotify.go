// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package fsnotify

import (
	"context"

	"github.com/fsnotify/fsnotify"
	"github.com/shirou/gopsutil/v3/disk"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
)

// Watch creates dir watcher to hanlde events from os in loop
func Watch(ctx context.Context, dirs ...string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.L().Fatal("Failed to create fsnotify watcher.", zap.Error(err))
	}
	defer watcher.Close()

	for _, dir := range dirs {
		if err = watcher.Add(dir); err != nil {
			log.L().Fatal("Failed to watch dir: "+dir, zap.Error(err))
		}
	}
	log.S().Infof("Watch dirs: %+v", dirs)

	watchLoop(ctx, watcher)
}

// watchLoop receives events and errors from os
func watchLoop(ctx context.Context, watcher *fsnotify.Watcher) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-watcher.Events:
			if !ok {
				log.L().Error("Failed to get watcher events.")
				continue
			}
			// check disk space per write operation
			if ev.Has(fsnotify.Write) {
				checkDiskSpace(ctx)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				log.L().Error("Failed to get watcher errors.")
				continue
			}
			log.L().Error("watch file error", zap.Error(err))
		}
	}
}

// checkDiskSpace returns the disk usage info
func checkDiskSpace(ctx context.Context) {
	usage, err := disk.Usage("/")
	if err != nil {
		log.L().Error("Failed to get disk usage.", zap.Error(err))
		return
	}
	// panic if no space or no inodes
	if usage.Free == 0 || usage.InodesFree == 0 {
		log.L().Fatal("No space in device.", zap.Float64("UsedPercent", usage.UsedPercent), zap.Float64("InodesUsedPercent", usage.InodesUsedPercent))
	}
}
