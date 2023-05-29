package nodestats

import "time"

type RpcReport struct {
	Method       string
	HandlingTime time.Duration
	Success      bool
}
