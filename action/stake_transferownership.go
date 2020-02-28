package action

import (
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/byteutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/pkg/version"
)

// TransferStake defines the action of transfering stake ownership ts the other
type TransferStake struct {
	AbstractAction

	voterAddress string
	bucketIndex  uint64
	payload      []byte
}

// NewTransferStake returns a TransferStake instance
func NewTransferStake(
	nonce uint64,
	voterName string,
	bucketIndex uint64,
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*TransferStake, error) {
	return &TransferStake{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		voterAddress: voterName,
		bucketIndex:  bucketIndex,
		payload:      payload,
	}, nil
}

// VoterAddress returns the address of recipient
func (ts *TransferStake) VoterAddress() string { return ts.voterAddress }

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
		VoterAddress: ts.voterAddress,
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

	ts.voterAddress = pbAct.GetVoterAddress()
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
	intrinsicGas, err := ts.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed ts get intrinsic gas for the TransferStake")
	}
	transferStakeFee := big.NewInt(0).Mul(ts.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return transferStakeFee, nil
}
