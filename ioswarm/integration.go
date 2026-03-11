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
