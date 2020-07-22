// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
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
		height     uint64
		candCenter *candCenter
		bucketPool *BucketPool
	}
)

func (c *candSR) Height() uint64 {
	return c.height
}

func (c *candSR) CandCenter() CandidateCenter {
	return c.candCenter
}

func (c *candSR) BucketPool() *BucketPool {
	return c.bucketPool
}

// GetStakingStateReader returns a candidate state reader that reflects the base view
func GetStakingStateReader(sr protocol.StateReader) (CandidateStateReader, error) {
	c, err := ConstructBaseView(sr)
	if err != nil {
		if errors.Cause(err) == protocol.ErrNoName {
			// the view does not exist yet, create it
			return CreateBaseView(sr)
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

	csr, ok := v.(CandidateStateReader)
	if !ok {
		return nil, errors.Wrap(protocol.ErrTypeAssertion, "expecting CandidateStateReader")
	}

	return &candSR{
		StateReader: sr,
		height:      height,
		candCenter:  csr.CandCenter().(*candCenter),
		bucketPool:  csr.BucketPool(),
	}, nil
}

// CreateBaseView creates the base view from state reader
func CreateBaseView(sr protocol.StateReader) (CandidateStateReader, error) {
	if sr == nil {
		return nil, ErrMissingField
	}

	all, err := loadCandidatesFromSR(sr)
	if err != nil {
		return nil, err
	}

	center, err := NewCandidateCenter(all)
	if err != nil {
		return nil, err
	}

	pool, err := NewBucketPool(sr)
	if err != nil {
		return nil, err
	}

	// TODO: remove CandidateCenter interface, no need for (*candCenter)
	return &candSR{
		candCenter: center.(*candCenter),
		bucketPool: pool,
	}, nil
}

func loadCandidatesFromSR(sr protocol.StateReader) (CandidateList, error) {
	_, iter, err := sr.States(protocol.NamespaceOption(CandidateNameSpace))
	if errors.Cause(err) == state.ErrStateNotExist {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	cands := make(CandidateList, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		c := &Candidate{}
		if err := iter.Next(c); err != nil {
			return nil, errors.Wrapf(err, "failed to deserialize candidate")
		}
		cands = append(cands, c)
	}
	return cands, nil
}

// ConvertToCandSR converts csm to csr
func ConvertToCandSR(csm CandidateStateManager) CandidateStateReader {
	return &candSR{
		candCenter: csm.CandCenter().(*candCenter),
		bucketPool: csm.BucketPool(),
	}
}
