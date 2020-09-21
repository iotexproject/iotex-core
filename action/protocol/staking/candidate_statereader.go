// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// CandidateStateReader contains candidate center and bucket pool
	CandidateStateReader interface {
		Height() uint64
		SR() protocol.StateReader
		BaseView() *ViewData
		GetCandidateByName(string) *Candidate
		GetCandidateByOwner(address.Address) *Candidate
		AllCandidates() CandidateList
		TotalStakedAmount() *big.Int
		ActiveBucketsCount() uint64
	}

	candSR struct {
		protocol.StateReader
		height uint64
		view   *ViewData
	}

	// ViewData is the data that need to be stored in protocol's view
	ViewData struct {
		candCenter *CandidateCenter
		bucketPool *BucketPool
	}
)

func (c *candSR) Height() uint64 {
	return c.height
}

func (c *candSR) SR() protocol.StateReader {
	return c.StateReader
}

func (c *candSR) BaseView() *ViewData {
	return c.view
}

func (c *candSR) GetCandidateByName(name string) *Candidate {
	return c.view.candCenter.GetByName(name)
}

func (c *candSR) GetCandidateByOwner(owner address.Address) *Candidate {
	return c.view.candCenter.GetByOwner(owner)
}

func (c *candSR) AllCandidates() CandidateList {
	return c.view.candCenter.All()
}

func (c *candSR) TotalStakedAmount() *big.Int {
	return c.view.bucketPool.Total()
}

func (c *candSR) ActiveBucketsCount() uint64 {
	return c.view.bucketPool.Count()
}

// GetStakingStateReader returns a candidate state reader that reflects the base view
func GetStakingStateReader(sr protocol.StateReader) (CandidateStateReader, error) {
	c, err := ConstructBaseView(sr)
	if err != nil {
		if errors.Cause(err) == protocol.ErrNoName {
			// the view does not exist yet, create it
			view, height, err := CreateBaseView(sr, true)
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

	height, err := sr.Height()
	if err != nil {
		return nil, err
	}
	v, err := sr.ReadView(protocolID)
	if err != nil {
		return nil, err
	}

	view, ok := v.(*ViewData)
	if !ok {
		return nil, errors.Wrap(ErrTypeAssertion, "expecting *ViewData")
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
func CreateBaseView(sr protocol.StateReader, enableSMStorage bool) (*ViewData, uint64, error) {
	if sr == nil {
		return nil, 0, ErrMissingField
	}

	all, height, err := getAllCandidates(sr)
	if err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return nil, height, err
	}

	center, err := NewCandidateCenter(all)
	if err != nil {
		return nil, height, err
	}

	pool, err := NewBucketPool(sr, enableSMStorage)
	if err != nil {
		return nil, height, err
	}

	return &ViewData{
		candCenter: center,
		bucketPool: pool,
	}, height, nil
}
