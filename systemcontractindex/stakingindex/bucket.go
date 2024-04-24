package stakingindex

import (
	"math/big"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/systemcontractindex/stakingindex/stakingpb"
)

type VoteBucket = staking.VoteBucket

type Bucket struct {
	Candidate                 address.Address
	Owner                     address.Address
	StakedAmount              *big.Int
	StakedDurationBlockNumber uint64
	CreatedAt                 uint64
	UnlockedAt                uint64
	UnstakedAt                uint64
}

func (bi *Bucket) Serialize() []byte {
	return byteutil.Must(proto.Marshal(bi.toProto()))
}

// Deserialize deserializes the bucket info
func (bi *Bucket) Deserialize(b []byte) error {
	m := stakingpb.Bucket{}
	if err := proto.Unmarshal(b, &m); err != nil {
		return err
	}
	return bi.loadProto(&m)
}

// clone clones the bucket info
func (bi *Bucket) toProto() *stakingpb.Bucket {
	return &stakingpb.Bucket{
		Candidate:  bi.Candidate.String(),
		CreatedAt:  bi.CreatedAt,
		Owner:      bi.Owner.String(),
		UnlockedAt: bi.UnlockedAt,
		UnstakedAt: bi.UnstakedAt,
		Amount:     bi.StakedAmount.String(),
		Duration:   bi.StakedDurationBlockNumber,
	}
}

func (bi *Bucket) loadProto(p *stakingpb.Bucket) error {
	candidate, err := address.FromString(p.Candidate)
	if err != nil {
		return err
	}
	owner, err := address.FromString(p.Owner)
	if err != nil {
		return err
	}
	amount, ok := new(big.Int).SetString(p.Amount, 10)
	if !ok {
		return errors.Errorf("invalid staked amount %s", p.Amount)
	}
	bi.CreatedAt = p.CreatedAt
	bi.UnlockedAt = p.UnlockedAt
	bi.UnstakedAt = p.UnstakedAt
	bi.Candidate = candidate
	bi.Owner = owner
	bi.StakedAmount = amount
	bi.StakedDurationBlockNumber = p.Duration
	return nil
}

func (b *Bucket) Clone() *Bucket {
	clone := &Bucket{
		StakedAmount:              b.StakedAmount,
		StakedDurationBlockNumber: b.StakedDurationBlockNumber,
		CreatedAt:                 b.CreatedAt,
		UnlockedAt:                b.UnlockedAt,
		UnstakedAt:                b.UnstakedAt,
	}
	candidate, _ := address.FromBytes(b.Candidate.Bytes())
	clone.Candidate = candidate
	owner, _ := address.FromBytes(b.Owner.Bytes())
	clone.Owner = owner
	stakingAmount := new(big.Int).Set(b.StakedAmount)
	clone.StakedAmount = stakingAmount
	return clone
}

func assembleVoteBucket(token uint64, bkt *Bucket, contractAddr string, blockInterval time.Duration) *VoteBucket {
	vb := VoteBucket{
		Index:                     token,
		StakedAmount:              bkt.StakedAmount,
		StakedDuration:            time.Duration(bkt.StakedDurationBlockNumber) * blockInterval,
		StakedDurationBlockNumber: bkt.StakedDurationBlockNumber,
		CreateBlockHeight:         bkt.CreatedAt,
		StakeStartBlockHeight:     bkt.CreatedAt,
		UnstakeStartBlockHeight:   bkt.UnstakedAt,
		AutoStake:                 bkt.UnlockedAt == maxBlockNumber,
		Candidate:                 bkt.Candidate,
		Owner:                     bkt.Owner,
		ContractAddress:           contractAddr,
	}
	if bkt.UnlockedAt != maxBlockNumber {
		vb.StakeStartBlockHeight = bkt.UnlockedAt
	}
	return &vb
}

func batchAssembleVoteBucket(idxs []uint64, bkts []*Bucket, contractAddr string, blockInterval time.Duration) []*VoteBucket {
	vbs := make([]*VoteBucket, 0, len(idxs))
	for i := range idxs {
		if bkts[i] == nil {
			vbs = append(vbs, nil)
			continue
		}
		vbs = append(vbs, assembleVoteBucket(idxs[i], bkts[i], contractAddr, blockInterval))
	}
	return vbs
}
