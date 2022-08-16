package blocksync

import "time"

var (
	//DefaultConfig is the default config for blocksync
	DefaultConfig = Config{
		Interval:              30 * time.Second,
		ProcessSyncRequestTTL: 10 * time.Second,
		BufferSize:            200,
		IntervalSize:          20,
		MaxRepeat:             3,
		RepeatDecayStep:       1,
	}
)

// Config is the configuration for blocksync
type Config struct {
	Interval              time.Duration `yaml:"interval"` // update duration
	ProcessSyncRequestTTL time.Duration `yaml:"processSyncRequestTTL"`
	BufferSize            uint64        `yaml:"bufferSize"`
	IntervalSize          uint64        `yaml:"intervalSize"`
	// MaxRepeat is the maximal number of repeat of a block sync request
	MaxRepeat int `yaml:"maxRepeat"`
	// RepeatDecayStep is the step for repeat number decreasing by 1
	RepeatDecayStep int `yaml:"repeatDecayStep"`
}
