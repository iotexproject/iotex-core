package nodestats

import (
	"fmt"
	"strings"

	"github.com/iotexproject/iotex-core/pkg/generic"
)

var _ IRPCLocalStats = (*RPCLocalStats)(nil)

type RPCLocalStats struct {
	allTimeStats *generic.Map[string, *MethodStats]
	currentStats *generic.Map[string, *MethodStats]
}

func NewRPCLocalStats() *RPCLocalStats {
	return &RPCLocalStats{
		allTimeStats: generic.NewMap[string, *MethodStats](),
		currentStats: generic.NewMap[string, *MethodStats](),
	}
}

func (s *RPCLocalStats) GetMethodStats(methodName string) *MethodStats {
	stats, ok := s.allTimeStats.Load(methodName)
	if !ok {
		stats = &MethodStats{}
	}
	return stats
}

func (s *RPCLocalStats) ReportCall(report RpcReport, size int64) {
	if report.Method == "" {
		return
	}
	methodStats, _ := s.currentStats.LoadOrStore(report.Method, &MethodStats{})
	allTimeMethodStats, _ := s.allTimeStats.LoadOrStore(report.Method, &MethodStats{})
	reportHandlingTimeMicroseconds := report.HandlingTime.Microseconds()
	if report.Success {
		methodStats.Successes++
		methodStats.AvgTimeOfSuccesses = (methodStats.AvgTimeOfSuccesses*int64(methodStats.Successes-1) + reportHandlingTimeMicroseconds) / int64(methodStats.Successes)
		if reportHandlingTimeMicroseconds > methodStats.MaxTimeOfSuccess {
			methodStats.MaxTimeOfSuccess = reportHandlingTimeMicroseconds
		}

		allTimeMethodStats.Successes++
		allTimeMethodStats.AvgTimeOfSuccesses = (allTimeMethodStats.AvgTimeOfSuccesses*int64(allTimeMethodStats.Successes-1) + reportHandlingTimeMicroseconds) / int64(allTimeMethodStats.Successes)
		if reportHandlingTimeMicroseconds > allTimeMethodStats.MaxTimeOfSuccess {
			allTimeMethodStats.MaxTimeOfSuccess = reportHandlingTimeMicroseconds
		}
	} else {
		methodStats.Errors++
		methodStats.AvgTimeOfErrors = (methodStats.AvgTimeOfErrors*int64(methodStats.Errors-1) + reportHandlingTimeMicroseconds) / int64(methodStats.Errors)
		if reportHandlingTimeMicroseconds > methodStats.MaxTimeOfError {
			methodStats.MaxTimeOfError = reportHandlingTimeMicroseconds
		}

		allTimeMethodStats.Errors++
		allTimeMethodStats.AvgTimeOfErrors = (allTimeMethodStats.AvgTimeOfErrors*int64(allTimeMethodStats.Errors-1) + reportHandlingTimeMicroseconds) / int64(allTimeMethodStats.Errors)
		if reportHandlingTimeMicroseconds > allTimeMethodStats.MaxTimeOfError {
			allTimeMethodStats.MaxTimeOfError = reportHandlingTimeMicroseconds
		}
	}
	methodStats.TotalSize += size
	allTimeMethodStats.TotalSize += size

	s.currentStats.Store(report.Method, methodStats)
	s.allTimeStats.Store(report.Method, allTimeMethodStats)
}

func (s *RPCLocalStats) BuildReport() string {
	snapshot := s.currentStats.Clone()
	defer s.currentStats.Clear()
	stringBuilder := strings.Builder{}

	if snapshot.Total() == 0 {
		return stringBuilder.String()
	}
	const reportHeader = "method                                  | " +
		"successes | " +
		" avg time (µs) | " +
		" max time (µs) | " +
		"   errors | " +
		" avg time (µs) | " +
		" max time (µs) |" +
		" avg size |" +
		" total size |"
	stringBuilder.WriteString("***** RPC report *****\n")
	divider := strings.Repeat("-", len(reportHeader)-4)
	stringBuilder.WriteString(divider + "\n")
	stringBuilder.WriteString(reportHeader + "\n")
	stringBuilder.WriteString(divider + "\n")
	total := &MethodStats{}
	snapshot.Range(func(key string, value *MethodStats) bool {
		if total.Successes+value.Successes > 0 {
			total.AvgTimeOfSuccesses = (total.AvgTimeOfSuccesses*int64(total.Successes) + int64(value.Successes)*value.AvgTimeOfSuccesses) / int64(total.Successes+value.Successes)
		} else {
			total.AvgTimeOfSuccesses = 0
		}
		if total.Errors+value.Errors > 0 {
			total.AvgTimeOfErrors = (total.AvgTimeOfErrors*int64(total.Errors) + int64(value.Errors)*value.AvgTimeOfErrors) / int64(total.Errors+value.Errors)
		} else {
			total.AvgTimeOfErrors = 0
		}
		total.Successes += value.Successes
		total.Errors += value.Errors
		if value.MaxTimeOfError > total.MaxTimeOfError {
			total.MaxTimeOfError = value.MaxTimeOfError
		}
		if value.MaxTimeOfSuccess > total.MaxTimeOfSuccess {
			total.MaxTimeOfSuccess = value.MaxTimeOfSuccess
		}
		total.TotalSize += value.TotalSize
		stringBuilder.WriteString(s.prepareReportLine(key, value) + "\n")
		return true
	})
	stringBuilder.WriteString(divider + "\n")
	stringBuilder.WriteString(s.prepareReportLine("Total", total) + "\n")
	stringBuilder.WriteString(divider + "\n")
	return stringBuilder.String()

}

func (s *RPCLocalStats) prepareReportLine(method string, stats *MethodStats) string {
	return fmt.Sprintf("%-40s| %9d | %14d | %14d | %9d | %14d | %14d | %8s | %10s |",
		method,
		stats.Successes,
		stats.AvgTimeOfSuccesses,
		stats.MaxTimeOfSuccess,
		stats.Errors,
		stats.AvgTimeOfErrors,
		stats.MaxTimeOfError,
		byteCountSI(stats.AvgSize()),
		byteCountSI(stats.TotalSize),
	)
}

func byteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}
