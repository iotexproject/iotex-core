package action

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
)

const (
	// TransferActionType is an enum type of transfer action
	TransferActionType = iota + 1
	// ExecutionActionType is an enum type of execution action
	ExecutionActionType
	// StakingActionType is an enum type of staking action
	StakingActionType
)

var (
	errInvalidAct = errors.New("invalid action")
)

// RlpTransaction is an interface which makes native action compatible with eth tx
type RlpTransaction interface {
	Nonce() uint64
	GasPrice() *big.Int
	GasLimit() uint64
	Recipient() string
	Amount() *big.Int
	Payload() []byte
}

func rlpRawHash(tx RlpTransaction, chainID uint32) (hash.Hash256, error) {
	rawTx, err := generateRlpTx(tx)
	if err != nil {
		return hash.ZeroHash256, err
	}
	h := types.NewEIP155Signer(big.NewInt(int64(chainID))).Hash(rawTx)
	return hash.BytesToHash256(h[:]), nil
}

func rlpSignedHash(tx RlpTransaction, chainID uint32, sig []byte) (hash.Hash256, error) {
	signedTx, err := reconstructSignedRlpTxFromSig(tx, chainID, sig)
	if err != nil {
		return hash.ZeroHash256, err
	}
	h := sha3.NewLegacyKeccak256()
	rlp.Encode(h, signedTx)
	return hash.BytesToHash256(h.Sum(nil)), nil
}

func generateRlpTx(act RlpTransaction) (*types.Transaction, error) {
	if act == nil {
		return nil, errors.New("nil action to generate RLP tx")
	}

	// generate raw tx
	if to := act.Recipient(); to != EmptyAddress {
		addr, err := address.FromString(to)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid recipient address %s", to)
		}
		ethAddr := common.BytesToAddress(addr.Bytes())
		return types.NewTransaction(act.Nonce(), ethAddr, act.Amount(), act.GasLimit(), act.GasPrice(), act.Payload()), nil
	}
	return types.NewContractCreation(act.Nonce(), act.Amount(), act.GasLimit(), act.GasPrice(), act.Payload()), nil
}

func reconstructSignedRlpTxFromSig(tx RlpTransaction, chainID uint32, sig []byte) (*types.Transaction, error) {
	if len(sig) != 65 {
		return nil, errors.Errorf("invalid signature length = %d, expecting 65", len(sig))
	}
	sc := make([]byte, 65)
	copy(sc, sig)
	if sc[64] >= 27 {
		sc[64] -= 27
	}

	rawTx, err := generateRlpTx(tx)
	if err != nil {
		return nil, err
	}
	signedTx, err := rawTx.WithSignature(types.NewEIP155Signer(big.NewInt(int64(chainID))), sc)
	if err != nil {
		return nil, err
	}
	return signedTx, nil
}

// DecodeRawTx decodes raw data string into eth tx
func DecodeRawTx(rawData string, chainID uint32) (tx *types.Transaction, sig []byte, pubkey crypto.PublicKey, err error) {
	//remove Hex prefix and decode string to byte
	rawData = strings.Replace(rawData, "0x", "", -1)
	rawData = strings.Replace(rawData, "0X", "", -1)
	var dataInString []byte
	dataInString, err = hex.DecodeString(rawData)
	if err != nil {
		return
	}

	// decode raw data into rlp tx
	tx = &types.Transaction{}
	err = rlp.DecodeBytes(dataInString, tx)
	if err != nil {
		return
	}

	// extract signature and recover pubkey
	v, r, s := tx.RawSignatureValues()
	recID := uint32(v.Int64()) - 2*chainID - 8
	sig = make([]byte, 64, 65)
	rSize := len(r.Bytes())
	copy(sig[32-rSize:32], r.Bytes())
	sSize := len(s.Bytes())
	copy(sig[64-sSize:], s.Bytes())
	sig = append(sig, byte(recID))

	// recover public key
	rawHash := types.NewEIP155Signer(big.NewInt(int64(chainID))).Hash(tx)
	pubkey, err = crypto.RecoverPubkey(rawHash[:], sig)
	return
}

// EthTxExportToNativeProto converts eth tx to protobuf type ActionCore
func EthTxExportToNativeProto(chainID uint32, actType int, tx *types.Transaction) (*iotextypes.ActionCore, error) {
	if tx == nil {
		return nil, errInvalidAct
	}
	to, value, gasPrice := "", "0", "0"
	if tx.GasPrice() != nil {
		gasPrice = tx.GasPrice().String()
	}
	if tx.Value() != nil {
		value = tx.Value().String()
	}
	if tx.To() != nil {
		ioAddr, _ := address.FromBytes(tx.To().Bytes())
		to = ioAddr.String()
	}

	core := &iotextypes.ActionCore{
		Version:  1,
		Nonce:    tx.Nonce(),
		GasLimit: tx.Gas(),
		GasPrice: gasPrice,
		ChainID:  chainID,
	}
	switch actType {
	case TransferActionType:
		core.Action = &iotextypes.ActionCore_Transfer{
			Transfer: &iotextypes.Transfer{
				Recipient: to,
				Payload:   tx.Data(),
				Amount:    value,
			},
		}
	case ExecutionActionType:
		core.Action = &iotextypes.ActionCore_Execution{
			Execution: &iotextypes.Execution{
				Contract: to,
				Data:     tx.Data(),
				Amount:   value,
			},
		}
	case StakingActionType:
		data := tx.Data()
		if len(data) <= 4 {
			return nil, errInvalidAct
		}
		if act, err := NewCreateStakeFromABIBinary(data); err == nil {
			core.Action = &iotextypes.ActionCore_StakeCreate{StakeCreate: act.Proto()}
			return core, nil
		}
		if act, err := NewDepositToStakeFromABIBinary(data); err == nil {
			core.Action = &iotextypes.ActionCore_StakeAddDeposit{StakeAddDeposit: act.Proto()}
			return core, nil
		}
		if act, err := NewChangeCandidateFromABIBinary(data); err == nil {
			core.Action = &iotextypes.ActionCore_StakeChangeCandidate{StakeChangeCandidate: act.Proto()}
			return core, nil
		}
		if act, err := NewUnstakeFromABIBinary(data); err == nil {
			core.Action = &iotextypes.ActionCore_StakeUnstake{StakeUnstake: act.Proto()}
			return core, nil
		}
		if act, err := NewWithdrawStakeFromABIBinary(data); err == nil {
			core.Action = &iotextypes.ActionCore_StakeWithdraw{StakeWithdraw: act.Proto()}
			return core, nil
		}
		if act, err := NewRestakeFromABIBinary(data); err == nil {
			core.Action = &iotextypes.ActionCore_StakeRestake{StakeRestake: act.Proto()}
			return core, nil
		}
		if act, err := NewTransferStakeFromABIBinary(data); err == nil {
			core.Action = &iotextypes.ActionCore_StakeTransferOwnership{StakeTransferOwnership: act.Proto()}
			return core, nil
		}
		if act, err := NewCandidateRegisterFromABIBinary(data); err == nil {
			core.Action = &iotextypes.ActionCore_CandidateRegister{CandidateRegister: act.Proto()}
			return core, nil
		}
		if act, err := NewCandidateUpdateFromABIBinary(data); err == nil {
			core.Action = &iotextypes.ActionCore_CandidateUpdate{CandidateUpdate: act.Proto()}
			return core, nil
		}
		return nil, errInvalidAct
	default:
		return nil, errInvalidAct
	}
	return core, nil
}
