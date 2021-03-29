package action

import (
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// Envelope defines an envelope wrapped on action with some envelope metadata.
type Envelope struct {
	version  uint32
	nonce    uint64
	gasLimit uint64
	payload  actionPayload
	gasPrice *big.Int
}

// Version returns the version
func (elp *Envelope) Version() uint32 { return elp.version }

// Nonce returns the nonce
func (elp *Envelope) Nonce() uint64 { return elp.nonce }

// Destination returns the destination address
func (elp *Envelope) Destination() (string, bool) {
	r, ok := elp.payload.(hasDestination)
	if !ok {
		return "", false
	}

	return r.Destination(), true
}

// GasLimit returns the gas limit
func (elp *Envelope) GasLimit() uint64 { return elp.gasLimit }

// GasPrice returns the gas price
func (elp *Envelope) GasPrice() *big.Int {
	p := &big.Int{}
	if elp.gasPrice == nil {
		return p
	}
	return p.Set(elp.gasPrice)
}

// Cost returns cost of actions
func (elp *Envelope) Cost() (*big.Int, error) {
	return elp.payload.Cost()
}

// IntrinsicGas returns intrinsic gas of action.
func (elp *Envelope) IntrinsicGas() (uint64, error) {
	return elp.payload.IntrinsicGas()
}

// Action returns the action payload.
func (elp *Envelope) Action() Action { return elp.payload }

// Proto convert Envelope to protobuf format.
func (elp *Envelope) Proto() *iotextypes.ActionCore {
	actCore := &iotextypes.ActionCore{
		Version:  elp.version,
		Nonce:    elp.nonce,
		GasLimit: elp.gasLimit,
	}
	if elp.gasPrice != nil {
		actCore.GasPrice = elp.gasPrice.String()
	}

	// TODO assert each action
	act := elp.Action()
	switch act := act.(type) {
	case *Transfer:
		actCore.Action = &iotextypes.ActionCore_Transfer{Transfer: act.Proto()}
	case *Execution:
		actCore.Action = &iotextypes.ActionCore_Execution{Execution: act.Proto()}
	case *GrantReward:
		actCore.Action = &iotextypes.ActionCore_GrantReward{GrantReward: act.Proto()}
	case *ClaimFromRewardingFund:
		actCore.Action = &iotextypes.ActionCore_ClaimFromRewardingFund{ClaimFromRewardingFund: act.Proto()}
	case *DepositToRewardingFund:
		actCore.Action = &iotextypes.ActionCore_DepositToRewardingFund{DepositToRewardingFund: act.Proto()}
	case *PutPollResult:
		actCore.Action = &iotextypes.ActionCore_PutPollResult{PutPollResult: act.Proto()}
	case *CreateStake:
		actCore.Action = &iotextypes.ActionCore_StakeCreate{StakeCreate: act.Proto()}
	case *Unstake:
		actCore.Action = &iotextypes.ActionCore_StakeUnstake{StakeUnstake: act.Proto()}
	case *WithdrawStake:
		actCore.Action = &iotextypes.ActionCore_StakeWithdraw{StakeWithdraw: act.Proto()}
	case *DepositToStake:
		actCore.Action = &iotextypes.ActionCore_StakeAddDeposit{StakeAddDeposit: act.Proto()}
	case *Restake:
		actCore.Action = &iotextypes.ActionCore_StakeRestake{StakeRestake: act.Proto()}
	case *ChangeCandidate:
		actCore.Action = &iotextypes.ActionCore_StakeChangeCandidate{StakeChangeCandidate: act.Proto()}
	case *TransferStake:
		actCore.Action = &iotextypes.ActionCore_StakeTransferOwnership{StakeTransferOwnership: act.Proto()}
	case *CandidateRegister:
		actCore.Action = &iotextypes.ActionCore_CandidateRegister{CandidateRegister: act.Proto()}
	case *CandidateUpdate:
		actCore.Action = &iotextypes.ActionCore_CandidateUpdate{CandidateUpdate: act.Proto()}
	default:
		log.S().Panicf("Cannot convert type of action %T.\r\n", act)
	}
	return actCore
}

// LoadProto loads fields from protobuf format.
func (elp *Envelope) LoadProto(pbAct *iotextypes.ActionCore) error {
	if pbAct == nil {
		return errors.New("empty action proto to load")
	}
	if elp == nil {
		return errors.New("nil action to load proto")
	}
	*elp = Envelope{}
	elp.version = pbAct.GetVersion()
	elp.nonce = pbAct.GetNonce()
	elp.gasLimit = pbAct.GetGasLimit()
	elp.gasPrice = &big.Int{}
	elp.gasPrice.SetString(pbAct.GetGasPrice(), 10)

	switch {
	case pbAct.GetTransfer() != nil:
		act := &Transfer{}
		if err := act.LoadProto(pbAct.GetTransfer()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetExecution() != nil:
		act := &Execution{}
		if err := act.LoadProto(pbAct.GetExecution()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetGrantReward() != nil:
		act := &GrantReward{}
		if err := act.LoadProto(pbAct.GetGrantReward()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetClaimFromRewardingFund() != nil:
		act := &ClaimFromRewardingFund{}
		if err := act.LoadProto(pbAct.GetClaimFromRewardingFund()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetDepositToRewardingFund() != nil:
		act := &DepositToRewardingFund{}
		if err := act.LoadProto(pbAct.GetDepositToRewardingFund()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetPutPollResult() != nil:
		act := &PutPollResult{}
		if err := act.LoadProto(pbAct.GetPutPollResult()); err != nil {
			return err
		}
		elp.payload = act

	case pbAct.GetStakeCreate() != nil:
		act := &CreateStake{}
		if err := act.LoadProto(pbAct.GetStakeCreate()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetStakeUnstake() != nil:
		act := &Unstake{}
		if err := act.LoadProto(pbAct.GetStakeUnstake()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetStakeWithdraw() != nil:
		act := &WithdrawStake{}
		if err := act.LoadProto(pbAct.GetStakeWithdraw()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetStakeAddDeposit() != nil:
		act := &DepositToStake{}
		if err := act.LoadProto(pbAct.GetStakeAddDeposit()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetStakeRestake() != nil:
		act := &Restake{}
		if err := act.LoadProto(pbAct.GetStakeRestake()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetStakeChangeCandidate() != nil:
		act := &ChangeCandidate{}
		if err := act.LoadProto(pbAct.GetStakeChangeCandidate()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetStakeTransferOwnership() != nil:
		act := &TransferStake{}
		if err := act.LoadProto(pbAct.GetStakeTransferOwnership()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetCandidateRegister() != nil:
		act := &CandidateRegister{}
		if err := act.LoadProto(pbAct.GetCandidateRegister()); err != nil {
			return err
		}
		elp.payload = act
	case pbAct.GetCandidateUpdate() != nil:
		act := &CandidateUpdate{}
		if err := act.LoadProto(pbAct.GetCandidateUpdate()); err != nil {
			return err
		}
		elp.payload = act
	default:
		return errors.Errorf("no applicable action to handle in action proto %+v", pbAct)
	}
	return nil
}

// Serialize returns encoded binary.
func (elp *Envelope) Serialize() []byte {
	return byteutil.Must(proto.Marshal(elp.Proto()))
}

// Hash returns the hash value of SealedEnvelope.
func (elp *Envelope) Hash() hash.Hash256 {
	return hash.Hash256b(elp.Serialize())
}

// SetNonce sets the nonce value
func (elp *Envelope) SetNonce(n uint64) { elp.nonce = n }
