// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"context"
	"sync"

	"github.com/iotexproject/iotex-core/action"
	"github.com/pkg/errors"
)

// Validator is the interface of validator
type Validator interface {
	// Validate validates the given block's content
	Validate(ctx context.Context, block *Block) error
}

type validator struct {
	subValidator Validator
	validators   []action.SealedEnvelopeValidator
}

// NewValidator creates a validator with a set of sealed envelope validators
func NewValidator(subsequenceValidator Validator, validators ...action.SealedEnvelopeValidator) Validator {
	return &validator{subValidator: subsequenceValidator, validators: validators}
}

func (v *validator) Validate(ctx context.Context, blk *Block) error {
	actions := blk.Actions
	// Verify transfers, votes, executions, witness, and secrets
	errChan := make(chan error, len(actions))

	v.validateActions(ctx, actions, errChan)
	close(errChan)
	for err := range errChan {
		return errors.Wrap(err, "failed to validate action")
	}

	if v.subValidator != nil {
		return v.subValidator.Validate(ctx, blk)
	}
	return nil
}

func (v *validator) validateActions(
	ctx context.Context,
	actions []action.SealedEnvelope,
	errChan chan error,
) {
	var wg sync.WaitGroup
	for _, selp := range actions {
		wg.Add(1)
		go func(s action.SealedEnvelope) {
			defer wg.Done()
			for _, sev := range v.validators {
				if err := sev.Validate(ctx, s); err != nil {
					errChan <- err
					return
				}
			}
		}(selp)
	}
	wg.Wait()
}
