// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package scheme

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// Noop is the consensus scheme that does NOT create blocks
type Noop struct {
}

// NewNoop creates a Noop struct
func NewNoop() Scheme {
	return &Noop{}
}

// Start does nothing here
func (n *Noop) Start(_ context.Context) error { return nil }

// Stop does nothing here
func (n *Noop) Stop(_ context.Context) error { return nil }

// HandleConsensusMsg handles incoming consensus message
func (n *Noop) HandleConsensusMsg(*iotextypes.ConsensusMessage) error {
	log.Logger("consensus").Warn("Noop scheme does not handle incoming consensus message.")
	return nil
}

// Calibrate triggers an event to calibrate consensus context
func (n *Noop) Calibrate(uint64) {}

// ValidateBlockFooter validates the block footer
func (n *Noop) ValidateBlockFooter(*block.Block) error {
	log.Logger("consensus").Warn("Noop scheme could not calculate delegates by height")
	return nil
}

// Metrics is not implemented for noop scheme
func (n *Noop) Metrics() (ConsensusMetrics, error) {
	return ConsensusMetrics{}, errors.Wrapf(
		ErrNotImplemented,
		"noop scheme does not supported metrics yet",
	)
}

// Activate is not implemented for noop scheme
func (n *Noop) Activate(_ bool) {
	log.S().Warn("Noop scheme could not support activate")
}

// Active is always true for noop scheme
func (n *Noop) Active() bool { return true }
