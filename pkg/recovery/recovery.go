// Copyright (c) 2022 IoTeX
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package recovery

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"runtime/pprof"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
)

type (
	// CPUInfo stat cpu infos
	CPUInfo struct {
		PhysicalCount int             `json:"physical_count"`
		LogicalCount  int             `json:"logical_count"`
		TotalPercent  []float64       `json:"total_use_percent"`
		PerPercent    []float64       `json:"per_use_percent"`
		Loads         *load.AvgStat   `json:"average_loads"`
		Times         []cpu.TimesStat `json:"running_times"`
		Infos         []cpu.InfoStat  `json:"infos"`
	}
	// MemInfo stat memory infos
	MemInfo struct {
		Virtual *mem.VirtualMemoryStat `json:"virtaul"`
		Swap    *mem.SwapMemoryStat    `json:"swap"`
	}
	// DiskInfo stat disk infos
	DiskInfo struct {
		IOCounters map[string]disk.IOCountersStat `json:"io_counters"`
		Partitions []disk.PartitionStat           `json:"partitions"`
	}
)

// _crashlogDir saves the directory of crashlog
var _crashlogDir string = "/"

// Recover catchs the crashing goroutine
func Recover() {
	if r := recover(); r != nil {
		LogCrash(r)
	}
}

// SetCrashlogDir set the directory of crashlog
func SetCrashlogDir(dir string) error {
	if !fileutil.FileExists(dir) {
		if err := os.MkdirAll(dir, 0744); err != nil {
			log.S().Error(err.Error())
			return err
		}
	}
	_crashlogDir = dir
	return nil
}

// LogCrash write down the current memory and stack info and the cpu/mem/disk infos into log dir
func LogCrash(r interface{}) {
	log.S().Errorf("crashlog: %v", r)
	writeHeapProfile(filepath.Join(_crashlogDir,
		"heapdump_"+time.Now().String()+".out"))
	log.S().Infow("crashlog", "stack", string(debug.Stack()))
	printInfo("cpu", cpuInfo)
	printInfo("memory", memInfo)
	printInfo("disk", diskInfo)
}

func writeHeapProfile(path string) {
	f, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		log.S().Errorf("crashlog: open heap profile error: %v", err)
		return
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.S().Errorf("crashlog: close heap profile error: %v", err)
			return
		}
	}()
	if err := pprof.WriteHeapProfile(f); err != nil {
		log.S().Errorf("crashlog: write heap profile error: %v", err)
		return
	}
}

func printInfo(name string, info func() (interface{}, error)) {
	v, err := info()
	if err != nil {
		log.S().Errorw("crashlog: get %s info occur error: %v", name, err)
		return
	}
	log.S().Infow("crashlog", name, v)
}

func cpuInfo() (interface{}, error) {
	// cpu cores
	physicalCnt, err := cpu.Counts(false)
	if err != nil {
		return nil, err
	}
	logicalCnt, err := cpu.Counts(true)
	if err != nil {
		return nil, err
	}

	// cpu use percent during past 3 seconds
	totalPercent, err := cpu.Percent(100*time.Millisecond, false)
	if err != nil {
		return nil, err
	}
	perPercents, err := cpu.Percent(100*time.Millisecond, true)
	if err != nil {
		return nil, err
	}

	// cpu load stats
	// load1 respect avrage loads of tasks during past 1 minute
	// load5 respect avrage loads of tasks during past 5 minute
	// load15 respect avrage loads of tasks during past 15 minute
	loads, err := load.Avg()
	if err != nil {
		return nil, err
	}

	// cpu run time from startup
	times, err := cpu.Times(false)
	if err != nil {
		return nil, err
	}
	pertimes, err := cpu.Times(true)
	if err != nil {
		return nil, err
	}
	times = append(times, pertimes...)

	// cpu info
	infos, err := cpu.Info()
	if err != nil {
		return nil, err
	}

	return &CPUInfo{
		PhysicalCount: physicalCnt,
		LogicalCount:  logicalCnt,
		TotalPercent:  totalPercent,
		PerPercent:    perPercents,
		Loads:         loads,
		Times:         times,
		Infos:         infos,
	}, nil
}

func memInfo() (interface{}, error) {
	virtual, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	swap, err := mem.SwapMemory()
	if err != nil {
		return nil, err
	}
	return &MemInfo{
		Virtual: virtual,
		Swap:    swap,
	}, nil
}

func diskInfo() (interface{}, error) {
	ioCounters, err := disk.IOCounters()
	if err != nil {
		return nil, err
	}
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}
	return &DiskInfo{
		IOCounters: ioCounters,
		Partitions: partitions,
	}, nil
}
