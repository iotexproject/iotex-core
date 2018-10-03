// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package scheme

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/errcode"
	"github.com/iotexproject/iotex-core/proto"
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

// SetDoneStream does nothing for Noop (only used in simulator)
func (n *Noop) SetDoneStream(done chan bool) {}

// HandleBlockPropose handles incoming block propose
func (n *Noop) HandleBlockPropose(propose *iproto.ProposePb) error {
	logger.Warn().Msg("Noop scheme does not handle incoming block propose requests")
	return nil
}

// HandleEndorse handles incoming block propose
func (n *Noop) HandleEndorse(endorse *iproto.EndorsePb) error {
	logger.Warn().Msg("Noop scheme does not handle incoming endorse requests")
	return nil
}

// Metrics is not implemented for standalone scheme
func (n *Noop) Metrics() (ConsensusMetrics, error) {
	return ConsensusMetrics{}, errors.Wrapf(
		errcode.ErrNotImplemented,
		"noop scheme does not supported metrics yet",
	)
}
