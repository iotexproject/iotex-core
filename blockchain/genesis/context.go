// Copyright (c) 2021 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package genesis

import (
	"context"

	"github.com/iotexproject/iotex-core/pkg/log"
)

type genesisKey struct{}

// WithGenesisContext attachs genesis into context
func WithGenesisContext(ctx context.Context, g Genesis) context.Context {
	return context.WithValue(ctx, genesisKey{}, g)
}

// ExtractGenesisContext extracts genesis from context if available
func ExtractGenesisContext(ctx context.Context) (Genesis, bool) {
	gc, ok := ctx.Value(genesisKey{}).(Genesis)
	return gc, ok
}

// MustExtractGenesisContext extracts genesis from context if available, else panic
func MustExtractGenesisContext(ctx context.Context) Genesis {
	gc, ok := ctx.Value(genesisKey{}).(Genesis)
	if !ok {
		log.S().Panic("Miss genesis context")
	}
	return gc
}
