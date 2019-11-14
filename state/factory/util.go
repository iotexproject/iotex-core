// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/pkg/errors"
)

// createGenesisStates initialize the genesis states
func createGenesisStates(ctx context.Context, ws WorkingSet) error {
	raCtx, ok := protocol.GetRunActionsCtx(ctx)
	if !ok || raCtx.Registry == nil {
		return nil
	}
	for _, p := range raCtx.Registry.All() {
		if gsc, ok := p.(protocol.GenesisStateCreator); ok {
			if err := gsc.CreateGenesisStates(ctx, ws); err != nil {
				return errors.Wrap(err, "failed to create genesis states for protocol")
			}
		}
	}
	_ = ws.UpdateBlockLevelInfo(0)

	return nil
}
