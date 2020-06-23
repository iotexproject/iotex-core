// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// CandidateStateReader is candidate state reader on top of StateReader
	CandidateStateReader interface {
		protocol.StateReader

		getBucket(index uint64) (*VoteBucket, error)
		getAllBuckets() ([]*VoteBucket, error)
		getBucketsWithIndices(indices BucketIndices) ([]*VoteBucket, error)

		getOrCreateCandCenter() (CandidateCenter, error)
		getCandCenter() (CandidateCenter, error)
		createCandCenter() (CandidateCenter, error)
		loadCandidatesFromSR() (CandidateList, error)

		getVoterBucketIndices(addr address.Address) (*BucketIndices, error)
		getCandBucketIndices(addr address.Address) (*BucketIndices, error)
	}

	candSR struct {
		protocol.StateReader
	}
)

// NewCandidateStateReader returns a new CandidateStateReader instance
func NewCandidateStateReader(sr protocol.StateReader) (CandidateStateReader, error) {
	if sr == nil {
		return nil, ErrMissingField
	}

	return &candSR{sr}, nil
}

func (csr *candSR) getOrCreateCandCenter() (CandidateCenter, error) {
	c, err := csr.getCandCenter()
	if err != nil {
		if errors.Cause(err) == protocol.ErrNoName {
			// the view does not exist yet, create it
			cc, err := csr.createCandCenter()
			return cc, err
		}
		return nil, err
	}
	return c, nil
}

func (csr *candSR) getCandCenter() (CandidateCenter, error) {
	v, err := csr.ReadView(protocolID)
	if err != nil {
		return nil, err
	}

	if center, ok := v.(CandidateCenter); ok {
		return center, nil
	}
	return nil, errors.Wrap(ErrTypeAssertion, "expecting CandidateCenter")
}

func (csr *candSR) createCandCenter() (CandidateCenter, error) {
	all, err := csr.loadCandidatesFromSR()
	if err != nil {
		return nil, err
	}

	return NewCandidateCenter(all)
}

func (csr *candSR) loadCandidatesFromSR() (CandidateList, error) {
	_, iter, err := csr.States(protocol.NamespaceOption(CandidateNameSpace))
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

func (csr *candSR) getBucket(index uint64) (*VoteBucket, error) {
	var vb VoteBucket
	var err error
	if _, err = csr.State(&vb,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(bucketKey(index))); err != nil {
		return nil, err
	}
	var tc totalBucketCount
	if _, err := csr.State(
		&tc,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return nil, err
	}
	if errors.Cause(err) == state.ErrStateNotExist && index < tc.Count() {
		return nil, ErrWithdrawnBucket
	}
	return &vb, nil
}

func (csr *candSR) getAllBuckets() ([]*VoteBucket, error) {
	// bucketKey is prefixed with const bucket = '0', all bucketKey will compare less than []byte{bucket+1}
	maxKey := []byte{_bucket + 1}
	_, iter, err := csr.States(
		protocol.NamespaceOption(StakingNameSpace),
		protocol.FilterOption(func(k, v []byte) bool {
			return bytes.HasPrefix(k, []byte{_bucket})
		}, bucketKey(0), maxKey))
	if errors.Cause(err) == state.ErrStateNotExist {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	buckets := make([]*VoteBucket, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		vb := &VoteBucket{}
		if err := iter.Next(vb); err != nil {
			return nil, errors.Wrapf(err, "failed to deserialize bucket")
		}
		buckets = append(buckets, vb)
	}
	return buckets, nil
}

func (csr *candSR) getBucketsWithIndices(indices BucketIndices) ([]*VoteBucket, error) {
	buckets := make([]*VoteBucket, 0, len(indices))
	for _, i := range indices {
		b, err := csr.getBucket(i)
		if err != nil && err != ErrWithdrawnBucket {
			return buckets, err
		}
		buckets = append(buckets, b)
	}
	return buckets, nil
}
