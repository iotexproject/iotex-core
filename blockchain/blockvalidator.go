// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"errors"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/statefactory"
)

type Validator interface {
	// ValidateBody validates the given block's content.
	Validate(block *Block, tipHeight uint64, tipHash common.Hash32B, sf statefactory.StateFactory) error
}

type validator struct {
	sf statefactory.StateFactory
}

var (
	ErrInvalidTipHeight = errors.New("invalid tip height")
)

// NewValidator returns a new block validator which is safe for re-use
func NewValidator() Validator {
	return &validator{}
}

func (v *validator) Validate(blk *Block, tipHeight uint64, tipHash common.Hash32B, sf statefactory.StateFactory) error {
	if blk.Header.height == 0 || blk.Header.height != tipHeight+1 {
		return ErrInvalidTipHeight
	}
	return nil
}
