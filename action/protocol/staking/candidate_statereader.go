// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// BucketGetByIndex related to obtaining bucket by index
	BucketGetByIndex interface {
		getBucket(index uint64) (*VoteBucket, error)
	}
	// BucketGet related to obtaining bucket
	BucketGet interface {
		BucketGetByIndex
		getTotalBucketCount() (uint64, error)
		getAllBuckets() ([]*VoteBucket, uint64, error)
		getBucketsWithIndices(indices BucketIndices) ([]*VoteBucket, error)
		getBucketIndices(key []byte) (*BucketIndices, uint64, error)
	}
	// CandidateGet related to obtaining Candidate
	CandidateGet interface {
		getCandidate(name address.Address) (*Candidate, uint64, error)
		getAllCandidates() (CandidateList, uint64, error)
	}
	// CandidateStateReader contains candidate center and bucket pool
	CandidateStateReader interface {
		BucketGet
		CandidateGet
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

func srToCsr(sr protocol.StateReader) CandidateStateReader {
	return &candSR{
		StateReader: sr,
	}
}

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

	all, height, err := srToCsr(sr).getAllCandidates()
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

func (c *candSR) getTotalBucketCount() (uint64, error) {
	var tc totalBucketCount
	_, err := c.State(
		&tc,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey))
	return tc.count, err
}

func (c *candSR) getBucket(index uint64) (*VoteBucket, error) {
	var vb VoteBucket
	var err error
	if _, err = c.State(
		&vb,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(bucketKey(index))); err != nil {
		return nil, err
	}
	var tc totalBucketCount
	if _, err := c.State(
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

func (c *candSR) getAllBuckets() ([]*VoteBucket, uint64, error) {
	// bucketKey is prefixed with const bucket = '0', all bucketKey will compare less than []byte{bucket+1}
	maxKey := []byte{_bucket + 1}
	height, iter, err := c.States(
		protocol.NamespaceOption(StakingNameSpace),
		protocol.FilterOption(func(k, v []byte) bool {
			return bytes.HasPrefix(k, []byte{_bucket})
		}, bucketKey(0), maxKey))
	if err != nil {
		return nil, height, err
	}

	buckets := make([]*VoteBucket, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		vb := &VoteBucket{}
		if err := iter.Next(vb); err != nil {
			return nil, height, errors.Wrapf(err, "failed to deserialize bucket")
		}
		buckets = append(buckets, vb)
	}
	return buckets, height, nil
}

func (c *candSR) getBucketsWithIndices(indices BucketIndices) ([]*VoteBucket, error) {
	buckets := make([]*VoteBucket, 0, len(indices))
	for _, i := range indices {
		b, err := c.getBucket(i)
		if err != nil && err != ErrWithdrawnBucket {
			return buckets, err
		}
		buckets = append(buckets, b)
	}
	return buckets, nil
}

func (c *candSR) getBucketIndices(key []byte) (*BucketIndices, uint64, error) {
	var bis BucketIndices
	height, err := c.State(
		&bis,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(key))
	if err != nil {
		return nil, height, err
	}
	return &bis, height, nil
}

func (c *candSR) getCandidate(name address.Address) (*Candidate, uint64, error) {
	if name == nil {
		return nil, 0, ErrNilParameters
	}
	var d Candidate
	height, err := c.State(&d, protocol.NamespaceOption(CandidateNameSpace), protocol.KeyOption(name.Bytes()))
	return &d, height, err
}

func (c *candSR) getAllCandidates() (CandidateList, uint64, error) {
	height, iter, err := c.States(protocol.NamespaceOption(CandidateNameSpace))
	if err != nil {
		return nil, height, err
	}

	cands := make(CandidateList, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		c := &Candidate{}
		if err := iter.Next(c); err != nil {
			return nil, height, errors.Wrapf(err, "failed to deserialize candidate")
		}
		cands = append(cands, c)
	}
	return cands, height, nil
}
