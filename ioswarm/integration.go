// Package ioswarm provides integration points for embedding
// the IOSwarm coordinator into an iotex-core node.
//
// Integration into iotex-core requires:
//
// 1. Add to config/config.go:
//
//	type Config struct {
//	    ...
//	    IOSwarm ioswarm.Config `yaml:"ioswarm"`
//	}
//
// 2. Add to server/itx/server.go — newServer():
//
//	if cfg.IOSwarm.Enabled {
//	    svr.ioswarmCoord = ioswarm.NewCoordinator(
//	        cfg.IOSwarm,
//	        ioswarm.NewActPoolAdapter(cs.ActionPool(), cs.Blockchain()),
//	        ioswarm.NewStateReaderAdapter(cs.StateFactory()),
//	    )
//	}
//
// 3. Add to server/itx/server.go — Start():
//
//	if svr.ioswarmCoord != nil {
//	    svr.ioswarmCoord.Start(ctx)
//	}
//
// 4. Add to server/itx/server.go — Stop():
//
//	if svr.ioswarmCoord != nil {
//	    svr.ioswarmCoord.Stop()
//	}
//
// 5. Add to config YAML:
//
//	ioswarm:
//	  enabled: true
//	  grpcPort: 14689
//	  maxAgents: 100
//	  taskLevel: "L2"
//	  shadowMode: true
//	  pollIntervalMs: 1000
//
// Production adapters (ActPoolAdapter, StateReaderAdapter) are in adapter.go.
package ioswarm

import (
	"github.com/iotexproject/iotex-core/v2/state/factory"
)

// SetupStateDiffCallback wires the state diff callback from the state factory
// to the coordinator's diff broadcaster. This enables L4 agents to receive
// per-block state diffs for stateful validation.
func SetupStateDiffCallback(sf factory.Factory, coord *Coordinator) {
	ok := factory.SetDiffCallback(sf, func(height uint64, entries []factory.WriteQueueEntry, digest []byte) {
		// Convert factory.WriteQueueEntry → ioswarm.StateDiffEntry
		diffEntries := make([]StateDiffEntry, len(entries))
		for i, e := range entries {
			diffEntries[i] = StateDiffEntry{
				Type:      e.WriteType,
				Namespace: e.Namespace,
				Key:       e.Key,
				Value:     e.Value,
			}
		}
		coord.ReceiveStateDiff(height, diffEntries, digest)
	})
	if ok {
		coord.logger.Info("state diff callback wired to state factory")
	} else {
		coord.logger.Warn("state diff callback not supported by this state factory implementation")
	}
}
