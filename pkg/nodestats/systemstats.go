package nodestats

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/memory"
)

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

// BuildReport builds the report
func (s *systemStats) BuildReport() string {
	stringBuilder := strings.Builder{}
	cpuValue, _ := cpu.Get()
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

	stringBuilder.WriteString("\n")
	memValue, _ := memory.Get()
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
	return stringBuilder.String()
}
