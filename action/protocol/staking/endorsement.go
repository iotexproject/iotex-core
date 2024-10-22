package staking

import (
	"math"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
)

// EndorsementStatus
const (
	// EndorseExpired means the endorsement is expired
	EndorseExpired = EndorsementStatus(iota)
	// UnEndorsing means the endorser has submitted unendorsement, but it is not expired yet
	UnEndorsing
	// Endorsed means the endorsement is valid
	Endorsed
)

const (
	endorsementNotExpireHeight = math.MaxUint64
)

type (
	// EndorsementStatus is a uint8 that represents the status of the endorsement
	EndorsementStatus uint8

	// Endorsement is a struct that contains the expire height of the Endorsement
	Endorsement struct {
		// ExpireHeight is the height an endorsement is expired in legacy mode and it is the earliest height that can revoke the endorsement in new mode
		ExpireHeight uint64
	}
)

// String returns a human-readable string of the endorsement status
func (s EndorsementStatus) String() string {
	switch s {
	case EndorseExpired:
		return "Expired"
	case UnEndorsing:
		return "UnEndorsing"
	case Endorsed:
		return "Endorsed"
	default:
		return "Unknown"
	}
}

func (e *Endorsement) LegacyStatus(height uint64) EndorsementStatus {
	if e.ExpireHeight == endorsementNotExpireHeight {
		return Endorsed
	}
	if height >= e.ExpireHeight {
		return EndorseExpired
	}
	return UnEndorsing
}

// Status returns the status of the endorsement
func (e *Endorsement) Status(height uint64) EndorsementStatus {
	if height < e.ExpireHeight {
		return Endorsed
	}
	return UnEndorsing
}

// Serialize serializes endorsement to bytes
func (e *Endorsement) Serialize() ([]byte, error) {
	pb, err := e.toProto()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(pb)
}

// Deserialize deserializes bytes to endorsement
func (e *Endorsement) Deserialize(buf []byte) error {
	pb := &stakingpb.Endorsement{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal endorsement")
	}
	return e.fromProto(pb)
}

func (e *Endorsement) toProto() (*stakingpb.Endorsement, error) {
	return &stakingpb.Endorsement{
		ExpireHeight: e.ExpireHeight,
	}, nil
}

func (e *Endorsement) fromProto(pb *stakingpb.Endorsement) error {
	e.ExpireHeight = pb.ExpireHeight
	return nil
}
