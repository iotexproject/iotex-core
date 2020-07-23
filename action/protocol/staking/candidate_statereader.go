// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/pkg/errors"
)

type (
	// CandidateStateReader contains candidate center and bucket pool
	CandidateStateReader interface {
		// TODO: remove CandidateCenter interface, return *candCenter
		Height() uint64
		CandCenter() CandidateCenter
		BucketPool() *BucketPool
	}

	candSR struct {
		protocol.StateReader
		height uint64
		view   *ViewData
	}

	// ViewData is the data that need to be stored in protocol's view
	ViewData struct {
		candCenter *candCenter
		bucketPool *BucketPool
	}
)

func (c *candSR) Height() uint64 {
	return c.height
}

func (c *candSR) CandCenter() CandidateCenter {
	return c.view.candCenter
}

func (c *candSR) BucketPool() *BucketPool {
	return c.view.bucketPool
}

// GetStakingStateReader returns a candidate state reader that reflects the base view
func GetStakingStateReader(sr protocol.StateReader) (CandidateStateReader, error) {
	c, err := ConstructBaseView(sr)
	if err != nil {
		if errors.Cause(err) == protocol.ErrNoName {
			// the view does not exist yet, create it
			height, view, err := CreateBaseView(sr, true)
			if err != nil {
				return nil, err
			}
			return &candSR{
				StateReader: sr,
				height:      height,
				view:        view,
			}, nil
		}
		return nil, err
	}
	return c, nil
}

// ConstructBaseView returns a candidate state reader that reflects the base view
// it will be used read-only
func ConstructBaseView(sr protocol.StateReader) (CandidateStateReader, error) {
	if sr == nil {
		return nil, ErrMissingField
	}

	height, v, err := sr.ReadView(protocolID)
	if err != nil {
		return nil, err
	}

	view, ok := v.(*ViewData)
	if !ok {
		return nil, errors.Wrap(protocol.ErrTypeAssertion, "expecting *ViewData")
	}

	return &candSR{
		StateReader: sr,
		height:      height,
		view: &ViewData{
			candCenter: view.candCenter,
			bucketPool: view.bucketPool,
		},
	}, nil
}

// CreateBaseView creates the base view from state reader
func CreateBaseView(sr protocol.StateReader, enableSMStorage bool) (uint64, *ViewData, error) {
	if sr == nil {
		return 0, nil, ErrMissingField
	}

	all, height, err := getAllCandidates(sr)
	if err != nil {
		return height, nil, err
	}

	center, err := NewCandidateCenter(all)
	if err != nil {
		return height, nil, err
	}

	pool, err := NewBucketPool(sr, enableSMStorage)
	if err != nil {
		return height, nil, err
	}

	// TODO: remove CandidateCenter interface, no need for (*candCenter)
	return height, &ViewData{
		candCenter: center.(*candCenter),
		bucketPool: pool,
	}, nil
}

// ConvertToViewData converts state manager to ViewData
func ConvertToViewData(csm CandidateStateManager) *ViewData {
	return &ViewData{
		candCenter: csm.CandCenter().(*candCenter),
		bucketPool: csm.BucketPool(),
	}
}
