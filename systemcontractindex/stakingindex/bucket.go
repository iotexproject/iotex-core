package stakingindex

import (
	"math/big"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/systemcontractindex/stakingindex/stakingpb"
)

type VoteBucket = staking.VoteBucket

type Bucket struct {
	Candidate    address.Address
	Owner        address.Address
	StakedAmount *big.Int

	// Timestamped indicates whether the bucket time is in block height or timestamp
	Timestamped bool

	// these fields value are determined by the timestamped field
	StakedDuration uint64 // in seconds if timestamped, in block number if not
	CreatedAt      uint64 // in unix timestamp if timestamped, in block height if not
	UnlockedAt     uint64 // in unix timestamp if timestamped, in block height if not
	UnstakedAt     uint64 // in unix timestamp if timestamped, in block height if not

	// Muted indicates whether the bucket is vote weight muted
	Muted bool
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
		Candidate:   bi.Candidate.String(),
		CreatedAt:   bi.CreatedAt,
		Owner:       bi.Owner.String(),
		UnlockedAt:  bi.UnlockedAt,
		UnstakedAt:  bi.UnstakedAt,
		Amount:      bi.StakedAmount.String(),
		Duration:    bi.StakedDuration,
		Muted:       bi.Muted,
		Timestamped: bi.Timestamped,
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
	bi.StakedDuration = p.Duration
	bi.Muted = p.Muted
	bi.Timestamped = p.Timestamped
	return nil
}

func (b *Bucket) Clone() *Bucket {
	clone := &Bucket{
		StakedDuration: b.StakedDuration,
		CreatedAt:      b.CreatedAt,
		UnlockedAt:     b.UnlockedAt,
		UnstakedAt:     b.UnstakedAt,
		Muted:          b.Muted,
		Timestamped:    b.Timestamped,
	}
	candidate, _ := address.FromBytes(b.Candidate.Bytes())
	clone.Candidate = candidate
	owner, _ := address.FromBytes(b.Owner.Bytes())
	clone.Owner = owner
	clone.StakedAmount = new(big.Int).Set(b.StakedAmount)
	return clone
}

func assembleVoteBucket(token uint64, bkt *Bucket, contractAddr string, blocksToDurationFn blocksDurationFn) *VoteBucket {
	vb := VoteBucket{
		Index:           token,
		StakedAmount:    bkt.StakedAmount,
		AutoStake:       bkt.UnlockedAt == maxStakingNumber,
		Candidate:       bkt.Candidate,
		Owner:           bkt.Owner,
		ContractAddress: contractAddr,
		Timestamped:     bkt.Timestamped,
	}
	if bkt.Timestamped {
		vb.StakedDuration = time.Duration(bkt.StakedDuration) * time.Second
		vb.StakeStartTime = time.Unix(int64(bkt.CreatedAt), 0)
		vb.CreateTime = time.Unix(int64(bkt.CreatedAt), 0)
		if bkt.UnlockedAt != maxStakingNumber {
			vb.StakeStartTime = time.Unix(int64(bkt.UnlockedAt), 0)
		}
		if bkt.UnstakedAt == maxStakingNumber {
			vb.UnstakeStartTime = time.Unix(0, 0)
		} else {
			vb.UnstakeStartTime = time.Unix(int64(bkt.UnstakedAt), 0)
		}
	} else {
		vb.StakedDuration = blocksToDurationFn(bkt.CreatedAt, bkt.CreatedAt+bkt.StakedDuration)
		vb.StakedDurationBlockNumber = bkt.StakedDuration
		vb.CreateBlockHeight = bkt.CreatedAt
		vb.StakeStartBlockHeight = bkt.CreatedAt
		vb.UnstakeStartBlockHeight = bkt.UnstakedAt
		if bkt.UnlockedAt != maxStakingNumber {
			vb.StakeStartBlockHeight = bkt.UnlockedAt
		}
	}
	if bkt.Muted {
		vb.Candidate, _ = address.FromString(address.ZeroAddress)
	}
	return &vb
}

func batchAssembleVoteBucket(idxs []uint64, bkts []*Bucket, contractAddr string, blocksToDurationFn blocksDurationFn) []*VoteBucket {
	vbs := make([]*VoteBucket, 0, len(bkts))
	for i := range bkts {
		if bkts[i] == nil {
			vbs = append(vbs, nil)
			continue
		}
		vbs = append(vbs, assembleVoteBucket(idxs[i], bkts[i], contractAddr, blocksToDurationFn))
	}
	return vbs
}
