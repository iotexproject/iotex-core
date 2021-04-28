// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"context"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type (
	tipBlockKey struct{}
	// TipBlockContext contains the blockchain status context
	TipBlockContext struct {
		Height    uint64
		Hash      hash.Hash256
		Timestamp time.Time
	}
)

// WithTipBlockContext add TipBlockContext into context.
func WithTipBlockContext(ctx context.Context, bc TipBlockContext) context.Context {
	return context.WithValue(ctx, tipBlockKey{}, bc)
}

// ExtractTipBlockContext gets TipBlockContext
func ExtractTipBlockContext(ctx context.Context) (TipBlockContext, bool) {
	bc, ok := ctx.Value(tipBlockKey{}).(TipBlockContext)
	return bc, ok
}

// MustExtractTipBlockContext must get TipBlockContext.
// If context doesn't exist, this function panic.
func MustExtractTipBlockContext(ctx context.Context) TipBlockContext {
	bc, ok := ctx.Value(tipBlockKey{}).(TipBlockContext)
	if !ok {
		log.S().Panic("Miss blockchain context")
	}
	return bc
}
