package actsync

import "time"

type (
	// Config is the configuration for action syncer
	Config struct {
		Size     int           `yaml:"syncSize"`
		Interval time.Duration `yaml:"syncInterval"`
	}
	// Helper is the helper for action syncer
	Helper struct {
		P2PNeighbor     Neighbors
		UnicastOutbound UniCastOutbound
	}
)

// DefaultConfig is the default configuration
var DefaultConfig = Config{
	Size:     1000,
	Interval: 5 * time.Second,
}
