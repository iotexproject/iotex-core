package nodestats

import (
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/memory"
)

// DiskStatus is the status of the disk
type DiskStatus struct {
	All  uint64 `json:"All"`
	Used uint64 `json:"Used"`
	Free uint64 `json:"Free"`
}

// diskUsage of path/disk
func diskUsage(path string) (disk DiskStatus) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &fs)
	if err != nil {
		return
	}
	disk.All = fs.Blocks * uint64(fs.Bsize)
	disk.Free = fs.Bfree * uint64(fs.Bsize)
	disk.Used = disk.All - disk.Free
	return
}

var _ StatsReporter = (*systemStats)(nil)

type systemStats struct {
	cpuValue *cpu.Stats
}

func newSystemStats() *systemStats {
	cpuValue, _ := cpu.Get()
	return &systemStats{
		cpuValue: cpuValue,
	}
}

// GetMemory returns the memory stats
func (s *systemStats) GetMemory() (*memory.Stats, error) { return memory.Get() }

// GetCPU returns the cpu stats
func (s *systemStats) GetCPU() (*cpu.Stats, error) { return cpu.Get() }

// GetDisk returns the disk stats
func (s *systemStats) GetDisk() DiskStatus { return diskUsage("/") }

// BuildReport builds the report
func (s *systemStats) BuildReport() string {
	stringBuilder := strings.Builder{}
	cpuValue, _ := s.GetCPU()
	total := float64(cpuValue.Total - s.cpuValue.Total)
	user := float64(cpuValue.User - s.cpuValue.User)
	system := float64(cpuValue.System - s.cpuValue.System)
	idle := float64(cpuValue.Idle - s.cpuValue.Idle)

	s.cpuValue = cpuValue
	stringBuilder.WriteString("CPU: ")
	stringBuilder.WriteString(strconv.Itoa(runtime.NumCPU()))
	stringBuilder.WriteString("CPUs ")
	stringBuilder.WriteString("User: ")
	stringBuilder.WriteString(strconv.FormatFloat(user/total*100, 'f', 2, 64))
	stringBuilder.WriteString("% ")
	stringBuilder.WriteString("System: ")
	stringBuilder.WriteString(strconv.FormatFloat(system/total*100, 'f', 2, 64))
	stringBuilder.WriteString("% ")
	stringBuilder.WriteString("Idle: ")
	stringBuilder.WriteString(strconv.FormatFloat(idle/total*100, 'f', 2, 64))
	stringBuilder.WriteString("% ")

	stringBuilder.WriteString("\n")
	memValue, _ := s.GetMemory()
	stringBuilder.WriteString("Memory: ")
	stringBuilder.WriteString("Total: ")
	stringBuilder.WriteString(byteCountIEC(memValue.Total))
	stringBuilder.WriteString(" ")
	stringBuilder.WriteString("Free: ")
	stringBuilder.WriteString(byteCountIEC(memValue.Free))
	stringBuilder.WriteString(" ")
	stringBuilder.WriteString("Used: ")
	stringBuilder.WriteString(byteCountIEC(memValue.Used))
	stringBuilder.WriteString(" ")
	stringBuilder.WriteString("Cached: ")
	stringBuilder.WriteString(byteCountIEC(memValue.Cached))

	stringBuilder.WriteString("\n")
	diskValue := s.GetDisk()
	stringBuilder.WriteString("Disk: ")
	stringBuilder.WriteString("Total: ")
	stringBuilder.WriteString(byteCountIEC(diskValue.All))
	stringBuilder.WriteString(" ")
	stringBuilder.WriteString("Free: ")
	stringBuilder.WriteString(byteCountIEC(diskValue.Free))
	stringBuilder.WriteString(" ")
	stringBuilder.WriteString("Used: ")
	stringBuilder.WriteString(byteCountIEC(diskValue.Used))
	return stringBuilder.String()
}
