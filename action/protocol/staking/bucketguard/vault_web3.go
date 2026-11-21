// Copyright (c) 2026 IoTeX Foundation
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package bucketguard

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

// Draft5ABI extends the native staking envelope without changing protobufs.
const Draft5ABI = `[{"type":"function","name":"vaultCreate","inputs":[{"name":"amount","type":"uint256"},{"name":"ownerTxNonce","type":"uint64"},{"name":"algID","type":"uint32"},{"name":"publicKey","type":"bytes"},{"name":"acceptSig","type":"bytes"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"vaultAddDeposit","inputs":[{"name":"vaultIndex","type":"uint64"},{"name":"amount","type":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"vaultWithdraw","inputs":[{"name":"vaultIndex","type":"uint64"},{"name":"generation","type":"uint64"},{"name":"guardNonce","type":"uint64"},{"name":"deadline","type":"uint64"},{"name":"anchorHeight","type":"uint64"},{"name":"anchorHash","type":"bytes32"},{"name":"amount","type":"uint256"},{"name":"recipient","type":"address"},{"name":"sponsor","type":"address"},{"name":"sig","type":"bytes"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"vaultRotateOwner","inputs":[{"name":"vaultIndex","type":"uint64"},{"name":"generation","type":"uint64"},{"name":"guardNonce","type":"uint64"},{"name":"deadline","type":"uint64"},{"name":"anchorHeight","type":"uint64"},{"name":"anchorHash","type":"bytes32"},{"name":"newOwner","type":"address"},{"name":"sponsor","type":"address"},{"name":"sig","type":"bytes"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"vaultRotateGuard","inputs":[{"name":"vaultIndex","type":"uint64"},{"name":"generation","type":"uint64"},{"name":"guardNonce","type":"uint64"},{"name":"deadline","type":"uint64"},{"name":"anchorHeight","type":"uint64"},{"name":"anchorHash","type":"bytes32"},{"name":"algID","type":"uint32"},{"name":"publicKey","type":"bytes"},{"name":"acceptSig","type":"bytes"},{"name":"sponsor","type":"address"},{"name":"sig","type":"bytes"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"vaultRevokeSignatures","inputs":[{"name":"vaultIndex","type":"uint64"},{"name":"generation","type":"uint64"},{"name":"guardNonce","type":"uint64"},{"name":"deadline","type":"uint64"},{"name":"anchorHeight","type":"uint64"},{"name":"anchorHash","type":"bytes32"},{"name":"sponsor","type":"address"},{"name":"sig","type":"bytes"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"vaultInitiateExit","inputs":[{"name":"vaultIndex","type":"uint64"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"vaultCancelExit","inputs":[{"name":"vaultIndex","type":"uint64"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"vaultFinalizeExit","inputs":[{"name":"vaultIndex","type":"uint64"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"guardUnbindInitiate","inputs":[{"name":"vaultIndex","type":"uint64"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"guardUnbindFinalize","inputs":[{"name":"vaultIndex","type":"uint64"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"guardUnbindCancel","inputs":[{"name":"vaultIndex","type":"uint64"},{"name":"generation","type":"uint64"},{"name":"requestID","type":"uint64"},{"name":"deadline","type":"uint64"},{"name":"anchorHeight","type":"uint64"},{"name":"anchorHash","type":"bytes32"},{"name":"sponsor","type":"address"},{"name":"sig","type":"bytes"}],"outputs":[],"stateMutability":"nonpayable"},{"type":"function","name":"bindBucketGuardV2","inputs":[{"name":"vaultIndex","type":"uint64"},{"name":"unbindDelay","type":"uint64"},{"name":"algID","type":"uint32"},{"name":"publicKey","type":"bytes"},{"name":"acceptSig","type":"bytes"}],"outputs":[],"stateMutability":"nonpayable"}]`

var Draft5Contract = func() abi.ABI {
	a, err := abi.JSON(strings.NewReader(Draft5ABI))
	if err != nil {
		panic(err)
	}
	return a
}()

// VaultAction is the typed ABI envelope shared by Draft 5 entry points.
// Only fields declared by Method are encoded; canonical round trips reject
// noncanonical ABI encodings.
type VaultAction struct {
	Method                                      string
	Index, OwnerTxNonce, UnbindDelay, RequestID uint64
	Amount                                      *big.Int
	Guard                                       NewGuard
	Call                                        VaultCall
}

func (m *VaultAction) EthData() ([]byte, error) {
	if m == nil {
		return nil, ErrMalformed
	}
	method, ok := Draft5Contract.Methods[m.Method]
	if !ok {
		return nil, ErrWeb3Decode
	}
	var args []any
	for _, f := range method.Inputs {
		var v any
		switch f.Name {
		case "vaultIndex":
			v = m.Index
		case "amount":
			v = m.Amount
			if m.Method == "vaultWithdraw" {
				v = m.Call.Amount
			}
			if !validAmount(v.(*big.Int)) {
				return nil, ErrMalformed
			}
		case "ownerTxNonce":
			v = m.OwnerTxNonce
		case "unbindDelay":
			v = m.UnbindDelay
		case "requestID":
			v = m.RequestID
		case "generation":
			v = m.Call.Generation
		case "guardNonce":
			v = m.Call.GuardNonce
		case "deadline":
			v = m.Call.Deadline
		case "anchorHeight":
			v = m.Call.AnchorHeight
		case "anchorHash":
			if len(m.Call.AnchorHash) != 32 {
				return nil, ErrMalformed
			}
			v = [32]byte(m.Call.AnchorHash)
		case "algID":
			v = uint32(m.Guard.AlgID)
		case "publicKey":
			v = m.Guard.PublicKey
		case "acceptSig":
			v = m.Guard.AcceptSig
		case "recipient", "newOwner", "sponsor":
			b := m.Call.Sponsor
			if f.Name == "recipient" {
				b = m.Call.Recipient
			}
			if f.Name == "newOwner" {
				b = m.Call.NewOwner
			}
			if f.Name != "sponsor" || len(b) > 0 {
				if len(b) != 20 {
					return nil, ErrMalformed
				}
			}
			v = common.BytesToAddress(b)
		case "sig":
			v = m.Call.Sig
		}
		args = append(args, v)
	}
	data, err := method.Inputs.Pack(args...)
	if err != nil {
		return nil, err
	}
	return append(bytes.Clone(method.ID), data...), nil
}

func VaultActionFromEthData(data []byte) (*VaultAction, error) {
	if len(data) < 4 {
		return nil, ErrWeb3Decode
	}
	method, err := Draft5Contract.MethodById(data[:4])
	if err != nil {
		return nil, ErrWeb3Decode
	}
	vals := &abiValues{m: map[string]any{}}
	if err = method.Inputs.UnpackIntoMap(vals.m, data[4:]); err != nil {
		return nil, ErrWeb3Decode
	}
	m := &VaultAction{Method: method.Name}
	c := &m.Call
	c.Amount = new(big.Int)
	for _, f := range method.Inputs {
		switch f.Name {
		case "vaultIndex":
			m.Index = vals.u64(f.Name)
			c.VaultIndex = m.Index
		case "amount":
			n := get[*big.Int](vals, f.Name)
			if method.Name == "vaultWithdraw" {
				c.Amount = n
			} else {
				m.Amount = n
			}
		case "ownerTxNonce":
			m.OwnerTxNonce = vals.u64(f.Name)
		case "unbindDelay":
			m.UnbindDelay = vals.u64(f.Name)
		case "requestID":
			m.RequestID = vals.u64(f.Name)
		case "generation":
			c.Generation = vals.u64(f.Name)
		case "guardNonce":
			c.GuardNonce = vals.u64(f.Name)
		case "deadline":
			c.Deadline = vals.u64(f.Name)
		case "anchorHeight":
			c.AnchorHeight = vals.u64(f.Name)
		case "anchorHash":
			c.AnchorHash = vals.bytes32(f.Name)
		case "algID":
			m.Guard.AlgID = pqcrypto.AlgID(vals.u32(f.Name))
		case "publicKey":
			m.Guard.PublicKey = vals.bytes(f.Name)
		case "acceptSig":
			m.Guard.AcceptSig = vals.bytes(f.Name)
		case "recipient":
			c.Recipient = vals.addr(f.Name)
		case "newOwner":
			c.NewOwner = vals.addr(f.Name)
		case "sponsor":
			if b := vals.addr(f.Name); !bytes.Equal(b, make([]byte, 20)) {
				c.Sponsor = b
			}
		case "sig":
			c.Sig = vals.bytes(f.Name)
		}
	}
	switch m.Method {
	case "vaultWithdraw":
		c.Op = VaultOpWithdraw
	case "vaultRotateOwner":
		c.Op = VaultOpRotateOwner
	case "vaultRotateGuard":
		c.Op = VaultOpRotateGuard
		c.NewGuard = &m.Guard
	case "vaultRevokeSignatures":
		c.Op = VaultOpRevokeSignatures
	}
	if vals.err != nil {
		return nil, vals.err
	}
	enc, err := m.EthData()
	if err != nil || !bytes.Equal(enc, data) {
		return nil, ErrWeb3Decode
	}
	return m, nil
}

func (m *VaultAction) Validate() error {
	data, err := m.EthData()
	if err != nil {
		return ErrMalformed
	}
	decoded, err := VaultActionFromEthData(data)
	if err != nil {
		return err
	}
	// Validate the decoded representation so operation kinds cannot disagree.
	c := &decoded.Call
	if c.Op != 0 {
		return c.Validate()
	}
	switch m.Method {
	case "vaultCreate", "bindBucketGuardV2":
		if m.Guard.AlgID != pqcrypto.AlgSLHDSA192s || len(m.Guard.PublicKey) != 48 || len(m.Guard.AcceptSig) != 16224 {
			return ErrMalformed
		}
		if m.Method == "vaultCreate" && (!validAmount(m.Amount) || m.Amount.Sign() == 0) {
			return ErrMalformed
		}
		if m.Method == "bindBucketGuardV2" && m.UnbindDelay != 0 && m.UnbindDelay != UnbindDelayBlocks {
			return ErrMalformed
		}
	case "vaultAddDeposit":
		if !validAmount(m.Amount) || m.Amount.Sign() == 0 {
			return ErrMalformed
		}
	case "guardUnbindCancel":
		if len(c.Sig) != 16224 || c.AnchorHeight == 0 || c.Deadline < c.AnchorHeight || c.Deadline-c.AnchorHeight > ParamsV1().MaxDeadlineWindow {
			return ErrMalformed
		}
	}
	return nil
}

// IntrinsicGas charges decoded ABI fields consistently on every entry path.
func (m *VaultAction) IntrinsicGas() (uint64, error) {
	b, err := m.EthData()
	if err != nil {
		return 0, err
	}
	k := uint64(0)
	switch m.Method {
	case "vaultCreate", "bindBucketGuardV2", "vaultWithdraw", "vaultRotateOwner", "vaultRevokeSignatures", "guardUnbindCancel":
		k = 1
	case "vaultRotateGuard":
		k = 2
	}
	return ParamsV1().gas(len(b), k), nil
}

// Draft5Events publishes the Solidity event shape consumed by indexers.
var Draft5Events = func() abi.ABI {
	a, err := abi.JSON(strings.NewReader(`[{"type":"event","name":"VaultTransition","inputs":[{"name":"operation","type":"string","indexed":false},{"name":"index","type":"uint64","indexed":false},{"name":"bucketClass","type":"uint32","indexed":false},{"name":"generation","type":"uint64","indexed":false},{"name":"guardNonce","type":"uint64","indexed":false},{"name":"nextRequestID","type":"uint64","indexed":false}]}]`))
	if err != nil {
		panic(err)
	}
	return a
}()

// VaultGettersABI is shared by eth_call dispatch and wallet tooling.
const VaultGettersABI = `[{"type":"function","name":"vaultBucket","stateMutability":"view","inputs":[{"name":"index","type":"uint64"}],"outputs":[{"name":"owner","type":"address"},{"name":"amount","type":"uint256"},{"name":"guardRecord","type":"bytes"}]},{"type":"function","name":"stakingGuardV2","stateMutability":"view","inputs":[{"name":"index","type":"uint64"}],"outputs":[{"name":"guardRecord","type":"bytes"}]}]`
