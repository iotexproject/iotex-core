package types

// Application is an interface that enables any finite, deterministic state machine
// to be driven by a blockchain-based replication engine via the ABCI.
type Application interface {

	// Mempool Connection
	CheckTx(*RequestCheckTx) (*ResponseCheckTx, error) // Validate a tx for the mempool

	// Consensus Connection
	BeginBlock(*RequestBeginBlock) (*ResponseBeginBlock, error) // Signals the beginning of a block
	DeliverTx(*RequestDeliverTx) (*ResponseDeliverTx, error)    // Deliver a tx for full processing
}
