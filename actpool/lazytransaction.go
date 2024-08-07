// Copyright 2023 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package actpool

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/iotexproject/iotex-core/action"
)

// LazyTransaction contains a small subset of the transaction properties that is
// enough for the miner and other APIs to handle large batches of transactions;
// and supports pulling up the entire transaction when really needed.
type LazyTransaction struct {
	Pool LazyResolver           // Transaction resolver to pull the real transaction up
	Hash common.Hash            // Transaction hash to pull up if needed
	Tx   *action.SealedEnvelope // Transaction if already resolved

	Time      time.Time // Time when the transaction was first seen
	GasFeeCap *big.Int  // Maximum fee per gas the transaction may consume
	GasTipCap *big.Int  // Maximum miner tip per gas the transaction can pay

	Gas     uint64 // Amount of gas required by the transaction
	BlobGas uint64 // Amount of blob gas required by the transaction
}

// Resolve retrieves the full transaction belonging to a lazy handle if it is still
// maintained by the transaction pool.
//
// Note, the method will *not* cache the retrieved transaction if the original
// pool has not cached it. The idea being, that if the tx was too big to insert
// originally, silently saving it will cause more trouble down the line (and
// indeed seems to have caused a memory bloat in the original implementation
// which did just that).
func (ltx *LazyTransaction) Resolve() *action.SealedEnvelope {
	if ltx.Tx != nil {
		return ltx.Tx
	}
	return ltx.Pool.Get(ltx.Hash)
}

// LazyResolver is a minimal interface needed for a transaction pool to satisfy
// resolving lazy transactions. It's mostly a helper to avoid the entire sub-
// pool being injected into the lazy transaction.
type LazyResolver interface {
	// Get returns a transaction if it is contained in the pool, or nil otherwise.
	Get(hash common.Hash) *action.SealedEnvelope
}
