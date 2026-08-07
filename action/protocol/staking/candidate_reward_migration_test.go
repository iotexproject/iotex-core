// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

func TestMigrateHermesRewardOptIn(t *testing.T) {
	r := require.New(t)
	sm := testdb.NewMockStateManager(gomock.NewController(t))
	vault := identityset.Address(10)
	hermes := &Candidate{
		Owner: identityset.Address(1), Operator: identityset.Address(2), Reward: vault,
		Name: "hermes", Votes: big.NewInt(1), SelfStakeBucketIdx: 1, SelfStake: big.NewInt(1),
	}
	legacy := &Candidate{
		Owner: identityset.Address(3), Operator: identityset.Address(4), Reward: identityset.Address(11),
		Name: "legacy", Votes: big.NewInt(1), SelfStakeBucketIdx: 2, SelfStake: big.NewInt(1),
	}
	explicit := &Candidate{
		Owner: identityset.Address(5), Operator: identityset.Address(6), Reward: identityset.Address(12),
		Name: "explicit", Votes: big.NewInt(1), SelfStakeBucketIdx: 3, SelfStake: big.NewInt(1),
		VoterRewardOnchainOptIn: true,
	}
	installCandCenter(t, sm, legacy, explicit, hermes)

	r.NoError(migrateHermesRewardOptIn(context.Background(), sm, []string{vault.String()}))
	csr, err := ConstructBaseView(sm)
	r.NoError(err)
	r.True(csr.GetByIdentifier(hermes.GetIdentifier()).VoterRewardOnchainOptIn)
	r.False(csr.GetByIdentifier(legacy.GetIdentifier()).VoterRewardOnchainOptIn)
	r.True(csr.GetByIdentifier(explicit.GetIdentifier()).VoterRewardOnchainOptIn)

	stored, _, err := NewCandidateByAddressReader(sm).CandidateByAddress(hermes.GetIdentifier())
	r.NoError(err)
	r.True(stored.VoterRewardOnchainOptIn)
}

func TestMigrateHermesRewardOptInThroughCreatePreStates(t *testing.T) {
	r := require.New(t)
	p, sm, g := backfillPreStatesEnv(t)
	candidateID := identityset.Address(1)
	vault := identityset.Address(10)
	g.HermesRewardVaultAddresses = []string{vault.String()}
	r.NoError(TestOnlyPutCandidateRewardAddress(sm, candidateID, candidateID, vault, false, false))

	readOptIn := func() bool {
		candidate, _, err := NewCandidateByAddressReader(sm).CandidateByAddress(candidateID)
		r.NoError(err)
		return candidate.VoterRewardOnchainOptIn
	}

	r.NoError(p.CreatePreStates(backfillPreStatesCtx(g, backfillActivationHeight-1), sm))
	r.False(readOptIn(), "candidate must not be migrated before activation")

	r.NoError(p.CreatePreStates(backfillPreStatesCtx(g, backfillActivationHeight), sm))
	r.True(readOptIn(), "candidate must be migrated at activation")

	lateCandidateID := identityset.Address(2)
	r.NoError(TestOnlyPutCandidateRewardAddress(sm, lateCandidateID, lateCandidateID, vault, false, false))
	r.NoError(p.CreatePreStates(backfillPreStatesCtx(g, backfillActivationHeight+1), sm))
	r.True(readOptIn(), "the persisted migration must survive later blocks")
	lateCandidate, _, err := NewCandidateByAddressReader(sm).CandidateByAddress(lateCandidateID)
	r.NoError(err)
	r.False(lateCandidate.VoterRewardOnchainOptIn,
		"using a Hermes vault after activation must not implicitly opt in")
}
