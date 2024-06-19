package action

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
)

const (
	// MigrateStakePayloadGas represents the MigrateStake payload gas per uint
	MigrateStakePayloadGas = uint64(100)
	// MigrateStakeBaseIntrinsicGas represents the base intrinsic gas for MigrateStake
	MigrateStakeBaseIntrinsicGas = uint64(10000)

	migrateStakeInterfaceABI = `[
		{
			"inputs": [
				{
					"internalType": "uint64",
					"name": "bucketIndex",
					"type": "uint64"
				}
			],
			"name": "migrateStake",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
)

var (
	// migrateStakeMethod is the interface of the abi encoding of migrate stake action
	migrateStakeMethod abi.Method
	_                  EthCompatibleAction = (*MigrateStake)(nil)
)

type MigrateStake struct {
	AbstractAction

	bucketIndex uint64
}

func init() {
	migrateInterface, err := abi.JSON(strings.NewReader(migrateStakeInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	migrateStakeMethod, ok = migrateInterface.Methods["migrateStake"]
	if !ok {
		panic("fail to load the migrateStake method")
	}
}

// NewMigrateStake returns a MigrateStake instance
func NewMigrateStake(
	nonce uint64,
	index uint64,
	gasLimit uint64,
	gasPrice *big.Int,
) (*MigrateStake, error) {
	return &MigrateStake{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		bucketIndex: index,
	}, nil
}

// BucketIndex returns bucket index
func (ms *MigrateStake) BucketIndex() uint64 { return ms.bucketIndex }

// IntrinsicGas returns the intrinsic gas of a Restake
func (ms *MigrateStake) IntrinsicGas() (uint64, error) {
	return CalculateIntrinsicGas(MigrateStakeBaseIntrinsicGas, MigrateStakePayloadGas, 0)
}

// Cost returns the total cost of a MigrateStake
func (ms *MigrateStake) Cost() (*big.Int, error) {
	maxExecFee := big.NewInt(0).Mul(ms.GasPrice(), big.NewInt(0).SetUint64(ms.GasLimit()))
	return maxExecFee, nil
}

// Serialize returns a raw byte stream of the Stake again struct
func (ms *MigrateStake) Serialize() []byte {
	return byteutil.Must(proto.Marshal(ms.Proto()))
}

// Proto converts to protobuf Restake Action
func (ms *MigrateStake) Proto() *iotextypes.StakeMigrate {
	act := &iotextypes.StakeMigrate{
		BucketIndex: ms.bucketIndex,
	}
	return act
}

// LoadProto converts a protobuf's Action to Restake
func (ms *MigrateStake) LoadProto(pbAct *iotextypes.StakeMigrate) error {
	if pbAct == nil {
		return ErrNilProto
	}
	ms.bucketIndex = pbAct.GetBucketIndex()
	return nil
}

func (ms *MigrateStake) encodeABIBinary() ([]byte, error) {
	data, err := migrateStakeMethod.Inputs.Pack(ms.bucketIndex)
	if err != nil {
		return nil, err
	}
	return append(migrateStakeMethod.ID, data...), nil
}

// NewMigrateStakeFromABIBinary decodes data into MigrateStake
func NewMigrateStakeFromABIBinary(data []byte) (*MigrateStake, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		rs        MigrateStake
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(migrateStakeMethod.ID, data[:4]) {
		return nil, errDecodeFailure
	}
	if err := migrateStakeMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if rs.bucketIndex, ok = paramsMap["bucketIndex"].(uint64); !ok {
		return nil, errDecodeFailure
	}
	return &rs, nil
}

// ToEthTx converts action to eth-compatible tx
func (ms *MigrateStake) ToEthTx(_ uint32) (*types.Transaction, error) {
	data, err := ms.encodeABIBinary()
	if err != nil {
		return nil, err
	}
	return types.NewTx(&types.LegacyTx{
		Nonce:    ms.Nonce(),
		GasPrice: ms.GasPrice(),
		Gas:      ms.GasLimit(),
		To:       &_stakingProtocolEthAddr,
		Value:    big.NewInt(0),
		Data:     data,
	}), nil
}
