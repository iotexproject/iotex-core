// Copyright (c) 2026 IoTeX Foundation
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"
	"math/big"
)

// VaultAction wraps a Draft 5 native staking ABI envelope. The existing
// GuardedBucketOp protobuf carries ABI data under a new envelope op tag.
// Previously valid Execution messages keep their historical interpretation.
type VaultAction struct {
	stake_common
	data []byte
}

var _ EthCompatibleAction = (*VaultAction)(nil)

func NewVaultAction(m *bucketguard.VaultAction) (*VaultAction, error) {
	b, err := m.EthData()
	if err != nil {
		return nil, err
	}
	return NewVaultActionFromABIBinary(b)
}
func NewVaultActionFromABIBinary(b []byte) (*VaultAction, error) {
	if _, err := bucketguard.VaultActionFromEthData(b); err != nil {
		return nil, err
	}
	return &VaultAction{data: append([]byte(nil), b...)}, nil
}
func (a *VaultAction) Vault() (*bucketguard.VaultAction, error) {
	return bucketguard.VaultActionFromEthData(a.data)
}
func (a *VaultAction) EthData() ([]byte, error) { return append([]byte(nil), a.data...), nil }
func (a *VaultAction) Value() *big.Int          { return new(big.Int) }
func (a *VaultAction) IntrinsicGas() (uint64, error) {
	m, err := a.Vault()
	if err != nil {
		return 0, err
	}
	return m.IntrinsicGas()
}
func (a *VaultAction) SanityCheck() error {
	m, err := a.Vault()
	if err != nil {
		return err
	}
	return m.Validate()
}
func (a *VaultAction) FillAction(core *iotextypes.ActionCore) {
	core.Action = &iotextypes.ActionCore_GuardedBucketOp{GuardedBucketOp: &iotextypes.GuardedBucketOp{Op: draft5EnvelopeOp, Sig: append([]byte(nil), a.data...)}}
}

// Amount includes principal in the account admission balance check.
func (a *VaultAction) Amount() *big.Int {
	m, err := a.Vault()
	if err != nil {
		return nil
	}
	if m.Method == "vaultCreate" || m.Method == "vaultAddDeposit" {
		return new(big.Int).Set(m.Amount)
	}
	return nil
}

// draft5EnvelopeOp is outside the Draft 4 operation range (1–6). Its carrier
// was invalid before EnableBucketGuard, so decoding cannot reinterpret a
// historically valid native execution. No deployed protobuf field changes.
const draft5EnvelopeOp uint32 = 0x6505

func loadVaultProto(pb *iotextypes.GuardedBucketOp) (*VaultAction, error) {
	canonical := &iotextypes.GuardedBucketOp{Op: draft5EnvelopeOp, Sig: pb.GetSig()}
	if !proto.Equal(pb, canonical) {
		return nil, ErrInvalidAct
	}
	return NewVaultActionFromABIBinary(pb.GetSig())
}
