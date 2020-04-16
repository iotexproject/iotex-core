package action

import (
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/byteutil"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// TransferStake defines the action of transfering stake ownership ts the other
type TransferStake struct {
	voterAddress address.Address
	bucketIndex  uint64
	payload      []byte
}

// NewTransferStake returns a TransferStake instance
func NewTransferStake(
	voterAddress string,
	bucketIndex uint64,
	payload []byte,
) (*TransferStake, error) {
	voterAddr, err := address.FromString(voterAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load address from string")
	}
	return &TransferStake{
		voterAddress: voterAddr,
		bucketIndex:  bucketIndex,
		payload:      payload,
	}, nil
}

// VoterAddress returns the address of recipient
func (ts *TransferStake) VoterAddress() address.Address { return ts.voterAddress }

// BucketIndex returns bucket index
func (ts *TransferStake) BucketIndex() uint64 { return ts.bucketIndex }

// Payload returns the payload bytes
func (ts *TransferStake) Payload() []byte { return ts.payload }

// Serialize returns a raw byte stream of the transfer stake action struct
func (ts *TransferStake) Serialize() []byte {
	return byteutil.Must(proto.Marshal(ts.Proto()))
}

// Proto converts transfer stake to protobuf
func (ts *TransferStake) Proto() *iotextypes.StakeTransferOwnership {
	act := &iotextypes.StakeTransferOwnership{
		VoterAddress: ts.voterAddress.String(),
		BucketIndex:  ts.bucketIndex,
		Payload:      ts.payload,
	}

	return act
}

// LoadProto loads transfer stake protobuf
func (ts *TransferStake) LoadProto(pbAct *iotextypes.StakeTransferOwnership) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	voterAddress, err := address.FromString(pbAct.GetVoterAddress())
	if err != nil {
		return errors.Wrap(err, "failed to load address from string")
	}
	ts.voterAddress = voterAddress
	ts.bucketIndex = pbAct.GetBucketIndex()
	ts.payload = pbAct.GetPayload()
	return nil
}

// IntrinsicGas returns the intrinsic gas of a TransferStake
func (ts *TransferStake) IntrinsicGas() (uint64, error) {
	payloadSize := uint64(len(ts.Payload()))
	return calculateIntrinsicGas(MoveStakeBaseIntrinsicGas, MoveStakePayloadGas, payloadSize)
}

// Cost returns the tstal cost of a TransferStake
func (ts *TransferStake) Cost() (*big.Int, error) {
	return big.NewInt(0), nil
}

// SanityCheck checks the action
func (ts *TransferStake) SanityCheck() error {
	return nil
}
