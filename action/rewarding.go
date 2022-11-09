package action

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

const (
	// RewardingDataGas represents the rewarding data gas per uint
	RewardingDataGas = uint64(100)
	// RewardingBaseIntrinsicGas represents the base intrinsic gas for rewarding
	RewardingBaseIntrinsicGas = uint64(10000)

	_rewardingInterfaceABI = `[
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				},
				{
					"internalType": "uint8[]",
					"name": "data",
					"type": "uint8[]"
				}
			],
			"name": "claim",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				},
				{
					"internalType": "uint8[]",
					"name": "data",
					"type": "uint8[]"
				}
			],
			"name": "deposit",
			"outputs": [],
			"stateMutability": "payable",
			"type": "function"
		}
	]`
)

var (
	_rewardingClaimMethod   abi.Method
	_rewardingDepositMethod abi.Method
)

type (
	// Rewarding base struct for rewarding
	Rewarding struct {
		AbstractAction

		amount *big.Int
		data   []byte
	}

	// RewardingClaim struct for rewarding claim
	RewardingClaim struct {
		Rewarding
	}

	// RewardingDeposit struct for rewarding deposit
	RewardingDeposit struct {
		Rewarding
	}
)

func init() {
	rewardingInterface, err := abi.JSON(strings.NewReader(_rewardingInterfaceABI))
	if err != nil {
		panic(err)
	}
	var ok bool
	_rewardingClaimMethod, ok = rewardingInterface.Methods["claim"]
	if !ok {
		panic("fail to load the claim method")
	}
	_rewardingDepositMethod, ok = rewardingInterface.Methods["deposit"]
	if !ok {
		panic("fail to load the deposit method")
	}
}

// Amount returns the amount
func (r *Rewarding) Amount() *big.Int {
	if r.amount == nil {
		return big.NewInt(0)
	}
	return r.amount
}

// Data returns the data bytes
func (r *Rewarding) Data() []byte { return r.data }

// IntrinsicGas returns the intrinsic gas
func (r *Rewarding) IntrinsicGas() (uint64, error) {
	dataSize := uint64(len(r.Data()))
	return CalculateIntrinsicGas(RewardingBaseIntrinsicGas, RewardingDataGas, dataSize)
}

// SanityCheck validates the variables in the action
func (r *Rewarding) SanityCheck() error {
	if r.Amount().Sign() <= 0 {
		return errors.Wrap(ErrInvalidAmount, "negative value")
	}

	return r.AbstractAction.SanityCheck()
}

func (r *Rewarding) encodeABIBinary(method abi.Method) ([]byte, error) {
	data, err := method.Inputs.Pack(r.Amount(), r.Data())
	if err != nil {
		return nil, err
	}
	return append(method.ID, data...), nil
}

// NewRewardingClaim returns a RewardingClaim instance
func NewRewardingClaim(
	nonce uint64,
	amount *big.Int,
	data []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*RewardingClaim, error) {
	return &RewardingClaim{
		Rewarding{
			AbstractAction: AbstractAction{
				version:  version.ProtocolVersion,
				nonce:    nonce,
				gasLimit: gasLimit,
				gasPrice: gasPrice,
			},
			amount: amount,
			data:   data,
		},
	}, nil
}

// Serialize returns a raw byte stream of the RewardingClaim struct
func (r *RewardingClaim) Serialize() []byte {
	return byteutil.Must(proto.Marshal(r.Proto()))
}

// Proto converts to protobuf RewardingClaim Action
func (r *RewardingClaim) Proto() *iotextypes.ClaimFromRewardingFund {
	act := &iotextypes.ClaimFromRewardingFund{
		Data: r.data,
	}

	if r.amount != nil {
		act.Amount = r.amount.String()
	}
	return act
}

// LoadProto converts a protobuf's Action to RewardingClaim
func (r *RewardingClaim) LoadProto(pbAct *iotextypes.ClaimFromRewardingFund) error {
	if pbAct == nil {
		return ErrNilProto
	}

	r.data = pbAct.GetData()
	if pbAct.GetAmount() == "" {
		r.amount = big.NewInt(0)
	} else {
		amount, ok := new(big.Int).SetString(pbAct.GetAmount(), 10)
		if !ok {
			return errors.Errorf("invalid amount %s", pbAct.GetAmount())
		}
		r.amount = amount
	}

	return nil
}

// EncodeABIBinary encodes data in abi encoding
func (r *RewardingClaim) EncodeABIBinary() ([]byte, error) {
	return r.encodeABIBinary(_rewardingClaimMethod)
}

// ToEthTx converts action to eth-compatible tx
func (r *RewardingClaim) ToEthTx() (*types.Transaction, error) {
	addr, err := address.FromString(address.RewardingProtocol)
	if err != nil {
		return nil, err
	}
	ethAddr := common.BytesToAddress(addr.Bytes())
	data, err := r.EncodeABIBinary()
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(r.Nonce(), ethAddr, big.NewInt(0), r.GasLimit(), r.GasPrice(), data), nil
}

// Cost returns the total cost
func (r *RewardingClaim) Cost() (*big.Int, error) {
	intrinsicGas, err := r.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for rewarding claim")
	}
	fee := big.NewInt(0).Mul(r.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return fee, nil
}

// NewRewardingClaimFromABIBinary decodes data into action
func NewRewardingClaimFromABIBinary(data []byte) (*RewardingClaim, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		ac        RewardingClaim
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(_rewardingClaimMethod.ID[:], data[:4]) {
		return nil, errDecodeFailure
	}
	if err := _rewardingClaimMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if ac.amount, ok = paramsMap["amount"].(*big.Int); !ok {
		return nil, errDecodeFailure
	}
	if ac.data, ok = paramsMap["data"].([]byte); !ok {
		return nil, errDecodeFailure
	}
	return &ac, nil
}

// NewRewardingDeposit returns a RewardingDeposit instance
func NewRewardingDeposit(
	nonce uint64,
	amount *big.Int,
	data []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*RewardingDeposit, error) {
	return &RewardingDeposit{
		Rewarding{
			AbstractAction: AbstractAction{
				version:  version.ProtocolVersion,
				nonce:    nonce,
				gasLimit: gasLimit,
				gasPrice: gasPrice,
			},
			amount: amount,
			data:   data,
		},
	}, nil
}

// Serialize returns a raw byte stream of the RewardingDeposit struct
func (r *RewardingDeposit) Serialize() []byte {
	return byteutil.Must(proto.Marshal(r.Proto()))
}

// Proto converts to protobuf RewardingDeposit Action
func (r *RewardingDeposit) Proto() *iotextypes.DepositToRewardingFund {
	act := &iotextypes.DepositToRewardingFund{
		Data: r.data,
	}

	if r.amount != nil {
		act.Amount = r.amount.String()
	}
	return act
}

// LoadProto converts a protobuf's Action to RewardingDeposit
func (r *RewardingDeposit) LoadProto(pbAct *iotextypes.DepositToRewardingFund) error {
	if pbAct == nil {
		return ErrNilProto
	}

	r.data = pbAct.GetData()
	if pbAct.GetAmount() == "" {
		r.amount = big.NewInt(0)
	} else {
		amount, ok := new(big.Int).SetString(pbAct.GetAmount(), 10)
		if !ok {
			return errors.Errorf("invalid amount %s", pbAct.GetAmount())
		}
		r.amount = amount
	}

	return nil
}

// EncodeABIBinary encodes data in abi encoding
func (r *RewardingDeposit) EncodeABIBinary() ([]byte, error) {
	return r.encodeABIBinary(_rewardingDepositMethod)
}

// ToEthTx converts action to eth-compatible tx
func (r *RewardingDeposit) ToEthTx() (*types.Transaction, error) {
	addr, err := address.FromString(address.RewardingProtocol)
	if err != nil {
		return nil, err
	}
	ethAddr := common.BytesToAddress(addr.Bytes())
	data, err := r.EncodeABIBinary()
	if err != nil {
		return nil, err
	}
	return types.NewTransaction(r.Nonce(), ethAddr, r.Amount(), r.GasLimit(), r.GasPrice(), data), nil
}

// Cost returns the total cost
func (r *RewardingDeposit) Cost() (*big.Int, error) {
	intrinsicGas, err := r.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for rewarding deposit")
	}
	fee := big.NewInt(0).Mul(r.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return new(big.Int).Add(r.Amount(), fee), nil
}

// NewRewardingDepositFromABIBinary decodes data into action
func NewRewardingDepositFromABIBinary(data []byte) (*RewardingDeposit, error) {
	var (
		paramsMap = map[string]interface{}{}
		ok        bool
		ac        RewardingDeposit
	)
	// sanity check
	if len(data) <= 4 || !bytes.Equal(_rewardingDepositMethod.ID[:], data[:4]) {
		return nil, errDecodeFailure
	}
	if err := _rewardingDepositMethod.Inputs.UnpackIntoMap(paramsMap, data[4:]); err != nil {
		return nil, err
	}
	if ac.amount, ok = paramsMap["amount"].(*big.Int); !ok {
		return nil, errDecodeFailure
	}
	if ac.data, ok = paramsMap["data"].([]byte); !ok {
		return nil, errDecodeFailure
	}
	return &ac, nil
}
