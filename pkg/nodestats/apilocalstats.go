package nodestats

import (
	"fmt"
	"strings"
	"time"

	"github.com/iotexproject/iotex-core/pkg/generic"
)

// APIReport is the report of an API call
type APIReport struct {
	Method       string
	HandlingTime time.Duration
	Success      bool
}

type apiMethodStats struct {
	Successes          int
	Errors             int
	AvgTimeOfErrors    int64
	AvgTimeOfSuccesses int64
	MaxTimeOfError     int64
	MaxTimeOfSuccess   int64
	TotalSize          int64
}

// AvgSize returns the average size of the api call
func (m *apiMethodStats) AvgSize() int64 {
	if m.Successes+m.Errors == 0 {
		return 0
	}
	return m.TotalSize / int64(m.Successes+m.Errors)
}

// APILocalStats is the struct for getting API stats
type APILocalStats struct {
	allTimeStats *generic.Map[string, *apiMethodStats]
	currentStats *generic.Map[string, *apiMethodStats]
}

// NewAPILocalStats creates a new APILocalStats
func NewAPILocalStats() *APILocalStats {
	return &APILocalStats{
		allTimeStats: generic.NewMap[string, *apiMethodStats](),
		currentStats: generic.NewMap[string, *apiMethodStats](),
	}
}

// ReportCall reports a call to the API
func (s *APILocalStats) ReportCall(report APIReport, size int64) {
	if report.Method == "" {
		return
	}
	methodStats, _ := s.currentStats.LoadOrStore(report.Method, &apiMethodStats{})
	allTimeMethodStats, _ := s.allTimeStats.LoadOrStore(report.Method, &apiMethodStats{})
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

// BuildReport builds a report of the API stats
func (s *APILocalStats) BuildReport() string {
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
	stringBuilder.WriteString("***** API CALL report *****\n")
	divider := strings.Repeat("-", len(reportHeader)-4)
	stringBuilder.WriteString(divider + "\n")
	stringBuilder.WriteString(reportHeader + "\n")
	stringBuilder.WriteString(divider + "\n")
	total := &apiMethodStats{}
	snapshot.Range(func(key string, value *apiMethodStats) bool {
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
	stringBuilder.WriteString(s.prepareReportLine("TOTAL", total) + "\n")
	stringBuilder.WriteString(divider + "\n")
	return stringBuilder.String()

}

func (s *APILocalStats) prepareReportLine(method string, stats *apiMethodStats) string {
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

func byteCountIEC(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
