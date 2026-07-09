// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// envelope accessors delegate to the embedded Execution payload only; a
// non-Execution payload (Transfer) must yield nil for Value/To/Data.
func TestEnvelope_NonExecutionAccessorsNil(t *testing.T) {
	r := require.New(t)
	elp, _ := createEnvelope(1) // Transfer payload
	r.Nil(elp.Value())
	r.Nil(elp.To())
	r.Nil(elp.Data())
}

func TestEnvelope_ExecutionAccessors(t *testing.T) {
	r := require.New(t)
	exec := NewExecution(identityset.Address(1).String(), big.NewInt(7), []byte{0x1, 0x2})
	elp := (&EnvelopeBuilder{}).SetNonce(1).SetGasLimit(100000).SetGasPrice(big.NewInt(1)).SetAction(exec).Build()
	r.Equal(big.NewInt(7), elp.Value())
	r.NotNil(elp.To())
	r.Equal([]byte{0x1, 0x2}, elp.Data())
}

func TestEnvelope_DynamicFeeAccessors(t *testing.T) {
	r := require.New(t)
	feeCap := big.NewInt(100)
	tipCap := big.NewInt(5)
	acl := types.AccessList{{
		Address:     common.BytesToAddress([]byte{0x1}),
		StorageKeys: []common.Hash{{}, {}},
	}}
	exec := NewExecution("", big.NewInt(0), nil)
	elp := (&EnvelopeBuilder{}).SetTxType(DynamicFeeTxType).SetChainID(1).SetNonce(1).
		SetGasLimit(100000).SetDynamicGas(feeCap, tipCap).SetAccessList(acl).
		SetAction(exec).Build()

	r.Equal(0, elp.GasFeeCap().Cmp(feeCap))
	r.Equal(0, elp.GasTipCap().Cmp(tipCap))
	r.Len(elp.AccessList(), 1)

	// effective gas price = min(feeCap, tipCap+baseFee)
	baseFee := big.NewInt(10)
	r.Equal(0, elp.EffectiveGasPrice(baseFee).Cmp(big.NewInt(15)))
	bigBaseFee := big.NewInt(1000)
	r.Equal(0, elp.EffectiveGasPrice(bigBaseFee).Cmp(feeCap))
}

// IntrinsicGas and Cost must account for the access list surcharge.
func TestEnvelope_IntrinsicGasAndCostWithAccessList(t *testing.T) {
	r := require.New(t)
	acl := types.AccessList{{
		Address:     common.BytesToAddress([]byte{0x1}),
		StorageKeys: []common.Hash{{}, {}},
	}}
	exec := NewExecution("", big.NewInt(0), nil)

	base := (&EnvelopeBuilder{}).SetTxType(AccessListTxType).SetChainID(1).SetNonce(1).
		SetGasLimit(1000000).SetGasPrice(big.NewInt(1)).SetAction(exec).Build()
	withACL := (&EnvelopeBuilder{}).SetTxType(AccessListTxType).SetChainID(1).SetNonce(1).
		SetGasLimit(1000000).SetGasPrice(big.NewInt(1)).SetAccessList(acl).SetAction(exec).Build()

	gBase, err := base.IntrinsicGas()
	r.NoError(err)
	gACL, err := withACL.IntrinsicGas()
	r.NoError(err)
	// one address + two storage keys of surcharge
	r.Equal(gBase+TxAccessListAddressGas+2*TxAccessListStorageKeyGas, gACL)

	cBase, err := base.Cost()
	r.NoError(err)
	cACL, err := withACL.Cost()
	r.NoError(err)
	r.Equal(1, cACL.Cmp(cBase))
}

func TestEnvelope_Setters(t *testing.T) {
	r := require.New(t)
	elp, _ := createEnvelope(1)
	elp.SetNonce(99)
	elp.SetGas(88)
	elp.SetChainID(7)
	r.Equal(uint64(99), elp.Nonce())
	r.Equal(uint64(88), elp.Gas())
	r.Equal(uint32(7), elp.ChainID())
}

func TestEnvelope_Size(t *testing.T) {
	r := require.New(t)
	elp, _ := createEnvelope(1)
	// VersionSizeInBytes(4)+NonceSizeInBytes(8)+GasSizeInBytes(8) plus gas price
	// bytes plus payload size; must be strictly larger than the fixed prefix.
	r.Greater(elp.Size(), uint32(20))
}

// SanityCheck and ValidateSidecar for a legacy (non-blob) envelope: sanity
// passes and there is no sidecar to validate.
func TestEnvelope_SanityCheckAndValidateSidecar(t *testing.T) {
	r := require.New(t)
	elp, _ := createEnvelope(1)
	r.NoError(elp.SanityCheck())
	r.NoError(elp.ValidateSidecar())
}

// ToEthTx must reject payloads that are not EthCompatibleAction.
func TestEnvelope_ToEthTxUnsupported(t *testing.T) {
	r := require.New(t)
	putPollResult := NewPutPollResult(1, state.CandidateList{})
	elp := (&EnvelopeBuilder{}).SetNonce(1).SetGasLimit(100000).SetGasPrice(big.NewInt(1)).
		SetAction(putPollResult).Build()
	_, err := elp.ToEthTx()
	r.ErrorIs(err, ErrInvalidAct)
}

func TestEnvelope_ToEthTxSupported(t *testing.T) {
	r := require.New(t)
	elp, _ := createEnvelope(1) // Transfer is EthCompatible
	tx, err := elp.ToEthTx()
	r.NoError(err)
	r.NotNil(tx)
}

func TestEnvelope_LoadProtoNilAndNilReceiver(t *testing.T) {
	r := require.New(t)
	elp := &envelope{}
	r.ErrorIs(elp.LoadProto(nil), ErrNilProto)

	var nilElp *envelope
	r.ErrorIs(nilElp.LoadProto(&iotextypes.ActionCore{}), ErrNilAction)
}
