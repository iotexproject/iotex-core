// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"sort"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// migrateHermesRewardOptIn converts the pre-IIP-59 Hermes convention into the
// persisted opt-in bit. After this one-time migration, reward routing never
// needs to infer opt-in from a candidate's reward address again.
func migrateHermesRewardOptIn(
	ctx context.Context,
	sm protocol.StateManager,
	hermesVaults []string,
) error {
	if len(hermesVaults) == 0 {
		return nil
	}
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
	for _, candidate := range candidates {
		if candidate.Reward == nil || candidate.VoterRewardOnchainOptIn {
			continue
		}
		if _, ok := vaults[candidate.Reward.String()]; !ok {
			continue
		}
		candidate.VoterRewardOnchainOptIn = true
		if err := csm.Upsert(candidate); err != nil {
			return errors.Wrapf(err, "staking: migrate Hermes candidate %s to on-chain rewards", candidate.GetIdentifier())
		}
	}
	return nil
}
