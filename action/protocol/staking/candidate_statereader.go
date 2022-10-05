// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

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
		getBucketIndices(addr address.Address, prefix byte) (*BucketIndices, uint64, error)
		voterBucketIndices(addr address.Address) (*BucketIndices, uint64, error)
		candBucketIndices(addr address.Address) (*BucketIndices, uint64, error)
	}
	// CandidateGet related to obtaining Candidate
	CandidateGet interface {
		getCandidate(name address.Address) (*Candidate, uint64, error)
		getAllCandidates() (CandidateList, uint64, error)
	}
	// ReadState related to read bucket and candidate by request
	ReadState interface {
		readStateBuckets(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBuckets) (*iotextypes.VoteBucketList, uint64, error)
		readStateBucketsByVoter(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByVoter) (*iotextypes.VoteBucketList, uint64, error)
		readStateBucketsByCandidate(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByCandidate) (*iotextypes.VoteBucketList, uint64, error)
		readStateBucketByIndices(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes) (*iotextypes.VoteBucketList, uint64, error)
		readStateBucketCount(ctx context.Context, _ *iotexapi.ReadStakingDataRequest_BucketsCount) (*iotextypes.BucketsCount, uint64, error)
		readStateCandidates(ctx context.Context, req *iotexapi.ReadStakingDataRequest_Candidates) (*iotextypes.CandidateListV2, uint64, error)
		readStateCandidateByName(ctx context.Context, req *iotexapi.ReadStakingDataRequest_CandidateByName) (*iotextypes.CandidateV2, uint64, error)
		readStateCandidateByAddress(ctx context.Context, req *iotexapi.ReadStakingDataRequest_CandidateByAddress) (*iotextypes.CandidateV2, uint64, error)
		readStateTotalStakingAmount(ctx context.Context, _ *iotexapi.ReadStakingDataRequest_TotalStakingAmount) (*iotextypes.AccountMeta, uint64, error)
	}
	// CandidateStateReader contains candidate center and bucket pool
	CandidateStateReader interface {
		BucketGet
		CandidateGet
		ReadState
		Height() uint64
		SR() protocol.StateReader
		BaseView() *ViewData
		NewBucketPool(enableSMStorage bool) (*BucketPool, error)
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

func newCandidateStateReader(sr protocol.StateReader) CandidateStateReader {
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
	v, err := sr.ReadView(_protocolID)
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

	csr := newCandidateStateReader(sr)
	all, height, err := csr.getAllCandidates()
	if err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return nil, height, err
	}

	println("CreateBaseView =======", height)
	println("len(db) =", len(all))
	for _, d := range all {
		if d.Name == "binancevote" {
			println("db has vote")
			d.print()
		}
		if d.Name == "binancenode" {
			println("db has node")
			d.print()
		}
	}
	center, err := NewCandidateCenter(all)
	if err != nil {
		return nil, height, err
	}

	pool, err := csr.NewBucketPool(enableSMStorage)
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
	var (
		vb  VoteBucket
		err error
	)
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
	height, iter, err := c.States(
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeysOption(func() ([][]byte, error) {
			// TODO (zhi): fix potential racing issue
			count, err := c.getTotalBucketCount()
			if err != nil {
				return nil, err
			}
			keys := [][]byte{}
			for i := uint64(0); i < count; i++ {
				keys = append(keys, bucketKey(i))
			}
			return keys, nil
		}),
	)
	if err != nil {
		return nil, height, err
	}

	buckets := make([]*VoteBucket, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		vb := &VoteBucket{}
		switch err := iter.Next(vb); errors.Cause(err) {
		case nil:
			buckets = append(buckets, vb)
		case state.ErrNilValue:
		default:
			return nil, height, errors.Wrapf(err, "failed to deserialize bucket")
		}
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

func (c *candSR) getBucketIndices(addr address.Address, prefix byte) (*BucketIndices, uint64, error) {
	var (
		bis BucketIndices
		key = AddrKeyWithPrefix(addr, prefix)
	)
	height, err := c.State(
		&bis,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(key))
	if err != nil {
		return nil, height, err
	}
	return &bis, height, nil
}

func (c *candSR) voterBucketIndices(addr address.Address) (*BucketIndices, uint64, error) {
	return c.getBucketIndices(addr, _voterIndex)
}

func (c *candSR) candBucketIndices(addr address.Address) (*BucketIndices, uint64, error) {
	return c.getBucketIndices(addr, _candIndex)
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

func (c *candSR) NewBucketPool(enableSMStorage bool) (*BucketPool, error) {
	bp := BucketPool{
		enableSMStorage: enableSMStorage,
		total: &totalAmount{
			amount: big.NewInt(0),
		},
	}

	if bp.enableSMStorage {
		switch _, err := c.State(bp.total, protocol.NamespaceOption(StakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey)); errors.Cause(err) {
		case nil:
			return &bp, nil
		case state.ErrStateNotExist:
			// fall back to load all buckets
		default:
			return nil, err
		}
	}

	// sum up all existing buckets
	all, _, err := c.getAllBuckets()
	if err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return nil, err
	}

	for _, v := range all {
		if v.StakedAmount.Cmp(big.NewInt(0)) <= 0 {
			return nil, state.ErrNotEnoughBalance
		}
		bp.total.amount.Add(bp.total.amount, v.StakedAmount)
	}
	bp.total.count = uint64(len(all))
	return &bp, nil
}

func (c *candSR) readStateBuckets(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBuckets) (*iotextypes.VoteBucketList, uint64, error) {
	all, height, err := c.getAllBuckets()
	if err != nil {
		return nil, height, err
	}

	offset := int(req.GetPagination().GetOffset())
	limit := int(req.GetPagination().GetLimit())
	buckets := getPageOfBuckets(all, offset, limit)
	pbBuckets, err := toIoTeXTypesVoteBucketList(buckets)
	return pbBuckets, height, err
}

func (c *candSR) readStateBucketsByVoter(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByVoter) (*iotextypes.VoteBucketList, uint64, error) {
	voter, err := address.FromString(req.GetVoterAddress())
	if err != nil {
		return nil, 0, err
	}

	indices, height, err := c.voterBucketIndices(voter)
	if errors.Cause(err) == state.ErrStateNotExist {
		return &iotextypes.VoteBucketList{}, height, nil
	}
	if indices == nil || err != nil {
		return nil, height, err
	}
	buckets, err := c.getBucketsWithIndices(*indices)
	if err != nil {
		return nil, height, err
	}

	offset := int(req.GetPagination().GetOffset())
	limit := int(req.GetPagination().GetLimit())
	buckets = getPageOfBuckets(buckets, offset, limit)
	pbBuckets, err := toIoTeXTypesVoteBucketList(buckets)
	return pbBuckets, height, err
}

func (c *candSR) readStateBucketsByCandidate(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByCandidate) (*iotextypes.VoteBucketList, uint64, error) {
	cand := c.GetCandidateByName(req.GetCandName())
	if c == nil {
		return &iotextypes.VoteBucketList{}, 0, nil
	}

	indices, height, err := c.candBucketIndices(cand.Owner)
	if errors.Cause(err) == state.ErrStateNotExist {
		return &iotextypes.VoteBucketList{}, height, nil
	}
	if indices == nil || err != nil {
		return nil, height, err
	}
	buckets, err := c.getBucketsWithIndices(*indices)
	if err != nil {
		return nil, height, err
	}

	offset := int(req.GetPagination().GetOffset())
	limit := int(req.GetPagination().GetLimit())
	buckets = getPageOfBuckets(buckets, offset, limit)
	pbBuckets, err := toIoTeXTypesVoteBucketList(buckets)
	return pbBuckets, height, err
}

func (c *candSR) readStateBucketByIndices(ctx context.Context, req *iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes) (*iotextypes.VoteBucketList, uint64, error) {
	height, err := c.SR().Height()
	if err != nil {
		return &iotextypes.VoteBucketList{}, height, err
	}
	buckets, err := c.getBucketsWithIndices(BucketIndices(req.GetIndex()))
	if err != nil {
		return nil, height, err
	}
	pbBuckets, err := toIoTeXTypesVoteBucketList(buckets)
	return pbBuckets, height, err
}

func (c *candSR) readStateBucketCount(ctx context.Context, _ *iotexapi.ReadStakingDataRequest_BucketsCount) (*iotextypes.BucketsCount, uint64, error) {
	total, err := c.getTotalBucketCount()
	if errors.Cause(err) == state.ErrStateNotExist {
		return &iotextypes.BucketsCount{}, c.Height(), nil
	}
	if err != nil {
		return nil, 0, err
	}
	active, h, err := getActiveBucketsCount(ctx, c)
	if err != nil {
		return nil, h, err
	}
	return &iotextypes.BucketsCount{
		Total:  total,
		Active: active,
	}, h, nil
}

func (c *candSR) readStateCandidates(ctx context.Context, req *iotexapi.ReadStakingDataRequest_Candidates) (*iotextypes.CandidateListV2, uint64, error) {
	offset := int(req.GetPagination().GetOffset())
	limit := int(req.GetPagination().GetLimit())
	candidates := getPageOfCandidates(c.AllCandidates(), offset, limit)

	return toIoTeXTypesCandidateListV2(candidates), c.Height(), nil
}

func (c *candSR) readStateCandidateByName(ctx context.Context, req *iotexapi.ReadStakingDataRequest_CandidateByName) (*iotextypes.CandidateV2, uint64, error) {
	cand := c.GetCandidateByName(req.GetCandName())
	if c == nil {
		return &iotextypes.CandidateV2{}, c.Height(), nil
	}
	return cand.toIoTeXTypes(), c.Height(), nil
}

func (c *candSR) readStateCandidateByAddress(ctx context.Context, req *iotexapi.ReadStakingDataRequest_CandidateByAddress) (*iotextypes.CandidateV2, uint64, error) {
	owner, err := address.FromString(req.GetOwnerAddr())
	if err != nil {
		return nil, 0, err
	}
	cand := c.GetCandidateByOwner(owner)
	if c == nil {
		return &iotextypes.CandidateV2{}, c.Height(), nil
	}
	return cand.toIoTeXTypes(), c.Height(), nil
}

func (c *candSR) readStateTotalStakingAmount(ctx context.Context, _ *iotexapi.ReadStakingDataRequest_TotalStakingAmount) (*iotextypes.AccountMeta, uint64, error) {
	meta := iotextypes.AccountMeta{}
	meta.Address = address.StakingBucketPoolAddr
	total, h, err := getTotalStakedAmount(ctx, c)
	if err != nil {
		return nil, h, err
	}
	meta.Balance = total.String()
	return &meta, h, nil
}
