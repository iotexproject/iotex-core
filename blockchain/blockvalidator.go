// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/statefactory"
)

type Validator interface {
	// Validate validates the given block's content
	Validate(block *Block, tipHeight uint64, tipHash common.Hash32B) error
}

type validator struct {
	sf  statefactory.StateFactory
	utk *UtxoTracker
}

var (
	// ErrInvalidTipHeight is the error returned when the block height is not valid
	ErrInvalidTipHeight = errors.New("invalid tip height")
	// ErrInvalidBlock is the error returned when the block is not valid
	ErrInvalidBlock = errors.New("failed to validate the block")
)

// Validate validates the given block's content
func (v *validator) Validate(blk *Block, tipHeight uint64, tipHash common.Hash32B) error {
	if blk == nil {
		return ErrInvalidBlock
	}

	if blk.Header.height != 0 && blk.Header.height != tipHeight+1 {
		return errors.Wrapf(
			ErrInvalidBlock,
			"Wrong block height %d, expecting %d",
			blk.Header.height,
			tipHeight+1)
	}

	if blk.Header.prevBlockHash != tipHash {
		return errors.Wrapf(
			ErrInvalidBlock,
			"Wrong prev hash %x, expecting %x",
			blk.Header.prevBlockHash,
			tipHash)
	}

	if v.utk != nil {
		if err := v.utk.ValidateUtxo(blk); err != nil {
			return err
		}
	}

	if v.sf != nil {
		// TODO: exam txs based on states
	}

	return nil
}
