// Copyright (c) 2026 IoTeX Foundation
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package bucketguard

import "math/big"

// LoadVaultTransition hydrates one isolated transition from authenticated
// native state. Records are cloned; no host state or shared caches are used.
func LoadVaultTransition(d Domain, params VaultParams, total uint64, class uint32, index uint64, v *VaultBucket, g *BucketGuardV2, ownerBalance, poolBalance *big.Int) (*VaultEngine, error) {
	e, err := NewVaultEngine(d, params.MinVaultBalance)
	if err != nil {
		return nil, err
	}
	e.totalVaultCount = total
	if v != nil {
		if class == ClassAVault {
			e.vaults[VaultStateKey(VaultBucketPrefix, index)] = v.clone()
		} else {
			e.staking[index] = v.clone()
		}
		e.accounts[string(v.Owner)] = new(big.Int).Set(ownerBalance)
	}
	if g != nil {
		prefix := VaultGuardPrefix
		if class == ClassBStaking {
			prefix = 9
		}
		e.guards[VaultStateKey(prefix, index)] = g.clone()
	}
	pool := VaultPool()
	e.accounts[string(pool[:])] = new(big.Int).Set(poolBalance)
	return e, nil
}

// AuthorizeStakingV2 verifies the recovery-aware transition before issuing a
// native bucket write grant. It also computes the next isolated guard record.
func (e *VaultEngine) AuthorizeStakingV2(env OpEnv, m *GuardedBucketOp) (*Grant, *BucketGuardV2, error) {
	if err := e.StakingGuardedOp(env, m, func() error { return nil }); err != nil {
		return nil, nil, err
	}
	return &Grant{bucketIndex: m.BucketIndex, generation: m.Generation, nonce: m.Nonce, op: m.Op}, e.GuardV2(ClassBStaking, m.BucketIndex), nil
}
