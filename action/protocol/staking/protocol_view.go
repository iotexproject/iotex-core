// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/pkg/errors"
)

// stakingView is the view data of staking protocol
type stakingView struct {
	esmView *executorViewData
	csmView *ViewData
}

func createBaseView(sr protocol.StateReader, enableSMStorage bool) (*stakingView, error) {
	candidateView, _, err := createCandidateBaseView(sr, enableSMStorage)
	if err != nil {
		return nil, err
	}
	executorView, err := createExecutorBaseView(sr)
	if err != nil {
		return nil, err
	}
	return &stakingView{
		esmView: executorView,
		csmView: candidateView,
	}, nil
}

// readView reads the staking view from the protocol view
func readView(sm protocol.StateReader) (*stakingView, error) {
	v, err := sm.ReadView(_protocolID)
	if err != nil {
		return nil, err
	}
	view, ok := v.(*stakingView)
	if !ok {
		return nil, errors.Wrap(ErrTypeAssertion, "expecting *stakingView")
	}
	return view, nil
}

// writeView writes the staking view to the protocol view
func writeView(csm CandidateStateManager, esm *executorStateManager) error {
	return esm.WriteView(_protocolID, &stakingView{
		esmView: esm.view(),
		csmView: csm.DirtyView(),
	})
}
