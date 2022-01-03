// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"context"

	"github.com/iotexproject/iotex-core/action"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
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
	if err := v.validateActions(ctx, actions); err != nil {
		return errors.Wrap(err, "failed to validate action")
	}
	if v.subValidator != nil {
		return v.subValidator.Validate(ctx, blk)
	}
	return nil
}

func (v *validator) validateActions(ctx context.Context, actions []action.SealedEnvelope) error {
	var eg = new(errgroup.Group)
	for _, act := range actions {
		selp := act // https://golang.org/doc/faq#closures_and_goroutines
		for _, sev := range v.validators {
			validator := sev
			eg.Go(func() error {
				return validator.Validate(ctx, selp)
			})
		}
	}
	return eg.Wait()
}
