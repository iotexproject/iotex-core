// Copyright (c) 2026 IoTeX Foundation
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package bucketguard

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzVaultActionFromEthData(f *testing.F) {
	for _, method := range []string{"vaultCreate", "vaultWithdraw", "vaultRotateGuard", "vaultInitiateExit", "bindBucketGuardV2", "guardUnbindCancel"} {
		m := &VaultAction{Method: method, Amount: big.NewInt(100), UnbindDelay: UnbindDelayBlocks, Guard: *zeroNewGuard(), Call: VaultCall{Generation: 1, Deadline: 1010, AnchorHeight: testAnchorHeight, AnchorHash: testAnchorHash(), Amount: big.NewInt(1), Recipient: bytes.Repeat([]byte{1}, 20), Sig: zeroSig()}}
		data, err := m.EthData()
		require.NoError(f, err)
		decoded, err := VaultActionFromEthData(data)
		require.NoError(f, err)
		require.NoError(f, decoded.Validate())
		f.Add(data)
	}
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := VaultActionFromEthData(data)
		if err != nil {
			return
		}
		encoded, err := m.EthData()
		require.NoError(t, err)
		require.Equal(t, data, encoded)
		back, err := VaultActionFromEthData(encoded)
		require.NoError(t, err)
		again, err := back.EthData()
		require.NoError(t, err)
		require.Equal(t, encoded, again)
		if m.Call.Op != 0 {
			_ = m.Call.Validate()
		}
	})
}

func FuzzVaultRecordDeserialize(f *testing.F) {
	type record interface {
		Serialize() ([]byte, error)
		Deserialize([]byte) error
	}
	for _, r := range []record{
		&VaultBucket{VaultIndex: 7, Owner: bytes.Repeat([]byte{1}, 20), Amount: []byte{100}},
		&BucketGuardV2{AlgID: alg, PublicKey: zeroPub(), BucketClass: ClassAVault, Generation: 1, GuardNonce: 2, NextRequestID: 3, BoundHeight: 4, UnbindDelay: UnbindDelayBlocks, CancelCooldown: 5, Pending: &PendingUnbind{2, 4, 4 + UnbindDelayBlocks}},
		&BucketGuardV2{BucketClass: ClassAVault, Generation: 3},
		&BucketGuardV2{AlgID: alg, PublicKey: zeroPub(), BucketClass: ClassBStaking, Generation: 1, CancelCooldown: 42},
	} {
		data, err := r.Serialize()
		require.NoError(f, err)
		f.Add(data)
	}
	f.Add([]byte{})
	f.Add([]byte{0x80})
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, r := range []record{new(VaultBucket), new(BucketGuardV2)} {
			if err := r.Deserialize(data); err == nil {
				encoded, err := r.Serialize()
				require.NoError(t, err)
				require.True(t, bytes.Equal(data, encoded), "accepted noncanonical record")
			}
		}
	})
}

func FuzzGuardedBucketOpFromEthData(f *testing.F) {
	bind := &BindBucketGuard{BucketIndex: 9, AlgID: alg, PublicKey: zeroPub(), AcceptSig: zeroSig()}
	data, err := bind.EthData()
	require.NoError(f, err)
	f.Add(data)
	for _, op := range []Op{OpUnstake, OpWithdraw, OpTransfer, OpChangeCandidate, OpRestake, OpRotate} {
		m := &GuardedBucketOp{BucketIndex: 9, Generation: 1, Deadline: 1010, AnchorHeight: testAnchorHeight, AnchorHash: testAnchorHash(), Op: op, Sig: zeroSig()}
		addr := bytes.Repeat([]byte{1}, 20)
		switch op {
		case OpWithdraw:
			m.Recipient = addr
		case OpTransfer:
			m.NewOwner, m.ClearGuard = addr, true
		case OpChangeCandidate:
			m.CandidateName, m.ExpectedCandidateID = "delegateab", addr
		case OpRestake:
			m.Duration, m.AutoStake = 91, true
		case OpRotate:
			m.NewGuard = zeroNewGuard()
		}
		require.NoError(f, m.Validate(ParamsV1()))
		data, err := m.EthData()
		require.NoError(f, err)
		f.Add(data)
	}
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if m, err := BindBucketGuardFromEthData(data); err == nil {
			encoded, err := m.EthData()
			require.NoError(t, err)
			back, err := BindBucketGuardFromEthData(encoded)
			require.NoError(t, err)
			again, err := back.EthData()
			require.NoError(t, err)
			require.Equal(t, encoded, again)
			_ = m.Validate(ParamsV1())
		}
		if m, err := GuardedBucketOpFromEthData(data); err == nil {
			// Decoding permits semantically invalid operations, such as a
			// rotation without a new key. Validation and encoding may reject them.
			_ = m.Validate(ParamsV1())
			if encoded, err := m.EthData(); err == nil {
				back, err := GuardedBucketOpFromEthData(encoded)
				require.NoError(t, err)
				again, err := back.EthData()
				require.NoError(t, err)
				require.Equal(t, encoded, again)
			}
		}
	})
}
