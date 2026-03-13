package ioswarm

// ActPoolReader is a read-only interface to the action pool.
// It provides access to pending transactions and current block height.
type ActPoolReader interface {
    // PendingActions returns all pending transactions from the action pool.
    PendingActions() []*pb.PendingAction

    // BlockHeight returns the current block height.
    BlockHeight() uint64
}

// ActPoolReader is a read-only interface to the action pool.
// It provides access to pending transactions and current block height.
type ActPoolReader interface {
	// PendingActions returns all pending transactions from the action pool.
	PendingActions() []*pb.PendingAction

	// BlockHeight returns the current block height.
	BlockHeight() uint64
}
