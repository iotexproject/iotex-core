// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"sort"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/delegateprofile"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

// migrateHermesRewardOptIn converts the pre-IIP-59 Hermes convention into the
// persisted opt-in bit. After this one-time migration, reward routing never
// needs to infer opt-in from a candidate's reward address again.
//
// From ZanzibarBeta on, a candidate is only migrated if the DelegateProfile
// contract holds *both* of its commission portions.
//
// Being migrated and having a usable commission configuration are independent
// facts: migration looks only at the reward address. A candidate that is opted
// in without portions is frozen at 100% commission -- poll_snapshot writes
// _fullCommissionBasisPoints into the snapshot when CommissionConfigured is
// false -- so its voters receive nothing, with no error and no event anywhere on
// chain. TestNet froze its first era with three of four migrated delegates in
// exactly that state, one of them the highest-weighted delegate on the network.
// Mainnet has 90 candidates whose reward address points at a Hermes vault and
// whose portions live in the Hermes service rather than in DelegateProfile.
//
// Leaving them opted out keeps them on the Hermes off-chain payout they are
// already using, which is the outcome they expect, instead of moving them into
// an on-chain path that pays their voters zero.
//
// Half a configuration counts as none: TestNet's uuu had set blockRewardPortion
// and not epochRewardPortion, and the result was identical to setting neither.
// readOne already collapses that case, so this reads Configured and nothing else.
func migrateHermesRewardOptIn(
	ctx context.Context,
	sm protocol.StateManager,
	hermesVaults []string,
	bridge *delegateprofile.Bridge,
	reader delegateprofile.ContractReader,
) error {
	if len(hermesVaults) == 0 {
		return nil
	}
	requireProfile := protocol.MustGetFeatureCtx(ctx).RequireProfileForHermesMigration
	vaults := make(map[string]struct{}, len(hermesVaults))
	for _, vault := range hermesVaults {
		vaults[vault] = struct{}{}
	}

	csm, err := NewCandidateStateManagerWithContext(ctx, sm)
	if err != nil {
		return errors.Wrap(err, "staking: construct candidate manager for Hermes opt-in migration")
	}
	all := csm.DirtyView().candCenter.All()
	candidates := make(CandidateList, 0, len(all))
	for _, candidate := range all {
		if candidate != nil && candidate.GetIdentifier() != nil {
			candidates = append(candidates, candidate)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return bytes.Compare(candidates[i].GetIdentifier().Bytes(), candidates[j].GetIdentifier().Bytes()) < 0
	})
	eligible := make([]*Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Reward == nil || candidate.VoterRewardOnchainOptIn {
			continue
		}
		if _, ok := vaults[candidate.Reward.String()]; !ok {
			continue
		}
		eligible = append(eligible, candidate)
	}

	configured, err := hermesMigrationProfileFilter(ctx, requireProfile, bridge, reader, eligible)
	if err != nil {
		return err
	}

	for _, candidate := range eligible {
		if configured != nil && !configured[candidate.GetIdentifier().String()] {
			log.L().Info("staking: candidate not migrated to on-chain rewards, DelegateProfile portions incomplete",
				zap.String("candidate", candidate.GetIdentifier().String()),
				zap.String("name", candidate.Name))
			continue
		}
		candidate.VoterRewardOnchainOptIn = true
		if err := csm.Upsert(candidate); err != nil {
			return errors.Wrapf(err, "staking: migrate Hermes candidate %s to on-chain rewards", candidate.GetIdentifier())
		}
	}
	return nil
}

// hermesMigrationProfileFilter returns which of the candidates have a complete
// DelegateProfile configuration, or nil when every candidate should be migrated
// regardless.
//
// nil is returned in three cases, all of which mean "do not filter": before
// ZanzibarBeta, so replaying history reproduces the original migration exactly;
// and when the chain has no DelegateProfile contract configured or no reader was
// injected, where filtering on an unreadable contract would opt everyone out
// rather than the ones that are actually misconfigured.
func hermesMigrationProfileFilter(
	ctx context.Context,
	requireProfile bool,
	bridge *delegateprofile.Bridge,
	reader delegateprofile.ContractReader,
	candidates []*Candidate,
) (map[string]bool, error) {
	if !requireProfile || bridge == nil || reader == nil || len(candidates) == 0 {
		return nil, nil
	}
	addrs := make([]address.Address, 0, len(candidates))
	for _, c := range candidates {
		addrs = append(addrs, c.GetIdentifier())
	}
	// Snapshot reads in the order given, and candidates is already sorted by
	// identifier bytes, so every node performs the same calls in the same order.
	rates, err := bridge.Snapshot(ctx, reader, addrs)
	if err != nil {
		return nil, errors.Wrap(err, "staking: read DelegateProfile for Hermes opt-in migration")
	}
	out := make(map[string]bool, len(rates))
	for id, r := range rates {
		out[id] = r != nil && r.Configured
	}
	return out, nil
}
