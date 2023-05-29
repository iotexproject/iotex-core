package nodestats

type MethodStats struct {
	Successes          int
	Errors             int
	AvgTimeOfErrors    int64
	AvgTimeOfSuccesses int64
	MaxTimeOfError     int64
	MaxTimeOfSuccess   int64
	TotalSize          int64
}

func NewMethodStats() *MethodStats {
	return &MethodStats{}
}

func (m *MethodStats) AvgSize() int64 {
	if m.Successes+m.Errors == 0 {
		return 0
	}
	return m.TotalSize / int64(m.Successes+m.Errors)
}
