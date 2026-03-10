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
//	    cs := svr.rootChainService
//	    coord := ioswarm.NewCoordinator(cfg.IOSwarm, cs.ActPool(), cs.StateFactory())
//	    svr.ioswarmCoord = coord
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
package ioswarm

// ActPoolAdapter wraps iotex-core's actpool.ActPool to implement ActPoolReader.
// Usage in iotex-core fork:
//
//	type ActPoolAdapter struct {
//	    pool actpool.ActPool
//	    bc   blockchain.Blockchain
//	}
//
//	func (a *ActPoolAdapter) PendingActions() []*PendingTx {
//	    actionMap := a.pool.PendingActionMap()
//	    var txs []*PendingTx
//	    for _, actions := range actionMap {
//	        for _, act := range actions {
//	            sealed, ok := act.(*action.SealedEnvelope)
//	            if !ok { continue }
//	            raw, _ := proto.Marshal(sealed.Proto())
//	            txs = append(txs, &PendingTx{
//	                Hash:     hex.EncodeToString(sealed.Hash()),
//	                From:     sealed.SrcPubkey().Address().String(),
//	                To:       sealed.Destination(),
//	                Nonce:    sealed.Nonce(),
//	                Amount:   sealed.Amount().String(),
//	                GasLimit: sealed.GasLimit(),
//	                GasPrice: sealed.GasPrice().String(),
//	                RawBytes: raw,
//	            })
//	        }
//	    }
//	    return txs
//	}
//
//	func (a *ActPoolAdapter) BlockHeight() uint64 {
//	    return a.bc.TipHeight()
//	}
type ActPoolAdapter struct{}

// StateReaderAdapter wraps iotex-core's factory.Factory to implement StateReader.
// Usage in iotex-core fork:
//
//	type StateReaderAdapter struct {
//	    sf factory.Factory
//	}
//
//	func (s *StateReaderAdapter) AccountState(address string) (*pb.AccountSnapshot, error) {
//	    addr, err := iotexaddress.FromString(address)
//	    if err != nil { return nil, err }
//	    ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{})
//	    acct, err := accountutil.AccountState(ctx, s.sf, addr)
//	    if err != nil { return nil, err }
//	    return &pb.AccountSnapshot{
//	        Address: address,
//	        Balance: acct.Balance.String(),
//	        Nonce:   acct.PendingNonce(),
//	        CodeHash: acct.CodeHash,
//	    }, nil
//	}
type StateReaderAdapter struct{}
