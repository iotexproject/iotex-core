package types

import (
	"context"
	"time"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/infra/blockchain/block"
)

type Context struct {
	baseCtx context.Context
	// ms         MultiStore
	header  *block.Header
	chainID string
	txBytes []byte
	// logger  *zap.Logger
	// voteInfo      []abci.VoteInfo
	// gasMeter      GasMeter
	// blockGasMeter GasMeter
	checkTx   bool
	recheckTx bool // if recheckTx == true, then checkTx must also be true
	// minGasPrice  DecCoins
	// consParams   *tmproto.ConsensusParams
	eventManager *EventManager
	priority     int64 // The tx priority, only relevant in CheckTx
}

// Read-only accessors
func (c Context) Context() context.Context { return c.baseCtx }

// func (c Context) MultiStore() MultiStore      { return c.ms }
func (c Context) BlockHeight() uint64  { return c.header.Height() }
func (c Context) BlockTime() time.Time { return c.header.Timestamp() }
func (c Context) ChainID() string      { return c.chainID }
func (c Context) TxBytes() []byte      { return c.txBytes }

// func (c Context) Logger() log.Logger          { return c.logger }
// func (c Context) VoteInfos() []abci.VoteInfo  { return c.voteInfo }
// func (c Context) GasMeter() GasMeter          { return c.gasMeter }
// func (c Context) BlockGasMeter() GasMeter     { return c.blockGasMeter }
func (c Context) IsCheckTx() bool   { return c.checkTx }
func (c Context) IsReCheckTx() bool { return c.recheckTx }

// func (c Context) MinGasPrices() DecCoins      { return c.minGasPrice }
func (c Context) EventManager() *EventManager { return c.eventManager }
func (c Context) Priority() int64             { return c.priority }

// HeaderHash returns a copy of the header hash obtained during abci.RequestBeginBlock
func (c Context) HeaderHash() hash.Hash256 {
	return c.header.HashBlock()
}

// func (c Context) ConsensusParams() *tmproto.ConsensusParams {
// return proto.Clone(c.consParams).(*tmproto.ConsensusParams)
// }

func (c Context) Deadline() (deadline time.Time, ok bool) {
	return c.baseCtx.Deadline()
}

func (c Context) Done() <-chan struct{} {
	return c.baseCtx.Done()
}

func (c Context) Err() error {
	return c.baseCtx.Err()
}
