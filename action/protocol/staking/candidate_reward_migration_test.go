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

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/delegateprofile"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// migrationCtx builds the feature context the migration reads, with the profile
// requirement either live or still ahead.
func migrationCtx(requireProfile bool) context.Context {
	g := genesis.TestDefault()
	if requireProfile {
		g.ToBeEnabledBlockHeight = 0
	} else {
		g.ToBeEnabledBlockHeight = 100
	}
	return protocol.WithFeatureCtx(protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), g),
		protocol.BlockCtx{BlockHeight: 1},
	))
}

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

	r.NoError(migrateHermesRewardOptIn(migrationCtx(false), sm, []string{vault.String()}, nil, nil))
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
	r.NoError(TestOnlyPutCandidateRewardAddress(context.Background(), sm, candidateID, candidateID, vault, false, false))

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
	r.NoError(TestOnlyPutCandidateRewardAddress(context.Background(), sm, lateCandidateID, lateCandidateID, vault, false, false))
	r.NoError(p.CreatePreStates(backfillPreStatesCtx(g, backfillActivationHeight+1), sm))
	r.True(readOptIn(), "the persisted migration must survive later blocks")
	lateCandidate, _, err := NewCandidateByAddressReader(sm).CandidateByAddress(lateCandidateID)
	r.NoError(err)
	r.False(lateCandidate.VoterRewardOnchainOptIn,
		"using a Hermes vault after activation must not implicitly opt in")
}

// Once the gate is live, a Hermes candidate with no usable commission
// configuration stays opted out.
//
// Being migrated and having portions published are independent facts, and the
// gap between them is silent: the candidate is frozen at 100% commission and its
// voters receive nothing, with no error and no event. TestNet froze its first era
// with three of four migrated delegates in that state.
func TestMigrateHermesRewardOptInRequiresProfileOnceGated(t *testing.T) {
	r := require.New(t)
	sm := testdb.NewMockStateManager(gomock.NewController(t))
	vault := identityset.Address(10)

	full := &Candidate{
		Owner: identityset.Address(1), Operator: identityset.Address(2), Reward: vault,
		Name: "full", Votes: big.NewInt(1), SelfStakeBucketIdx: 1, SelfStake: big.NewInt(1),
	}
	// Half a configuration is the same as none -- this is TestNet's uuu, which
	// had set blockRewardPortion and not epochRewardPortion and was frozen at
	// 100% commission exactly like the candidates that had set neither.
	half := &Candidate{
		Owner: identityset.Address(3), Operator: identityset.Address(4), Reward: vault,
		Name: "half", Votes: big.NewInt(1), SelfStakeBucketIdx: 2, SelfStake: big.NewInt(1),
	}
	none := &Candidate{
		Owner: identityset.Address(5), Operator: identityset.Address(6), Reward: vault,
		Name: "none", Votes: big.NewInt(1), SelfStakeBucketIdx: 3, SelfStake: big.NewInt(1),
	}
	installCandCenter(t, sm, full, half, none)

	store := newFakeProfileStore()
	store.setPortion(full.GetIdentifier(), "blockRewardPortion", 8500)
	store.setPortion(full.GetIdentifier(), "epochRewardPortion", 8600)
	store.setPortion(half.GetIdentifier(), "blockRewardPortion", 8500)

	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)

	r.NoError(migrateHermesRewardOptIn(
		migrationCtx(true), sm, []string{vault.String()}, bridge, store.reader(t)))

	csr, err := ConstructBaseView(sm)
	r.NoError(err)
	r.True(csr.GetByIdentifier(full.GetIdentifier()).VoterRewardOnchainOptIn,
		"a complete profile migrates")
	r.False(csr.GetByIdentifier(half.GetIdentifier()).VoterRewardOnchainOptIn,
		"block portion without epoch portion is not a usable configuration")
	r.False(csr.GetByIdentifier(none.GetIdentifier()).VoterRewardOnchainOptIn,
		"no profile means stay on the Hermes off-chain payout")
}

// Before the gate the migration must behave exactly as it did, profile or not.
// A chain that already ran this block has it committed; changing what it did
// would alter its receipt root and fork any node replaying history.
func TestMigrateHermesRewardOptInUnchangedBeforeGate(t *testing.T) {
	r := require.New(t)
	sm := testdb.NewMockStateManager(gomock.NewController(t))
	vault := identityset.Address(10)
	none := &Candidate{
		Owner: identityset.Address(5), Operator: identityset.Address(6), Reward: vault,
		Name: "none", Votes: big.NewInt(1), SelfStakeBucketIdx: 3, SelfStake: big.NewInt(1),
	}
	installCandCenter(t, sm, none)

	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)
	reader := delegateprofile.ContractReaderFunc(func(context.Context, string, []byte) ([]byte, error) {
		t.Fatal("the profile must not be consulted before the gate")
		return nil, nil
	})

	r.NoError(migrateHermesRewardOptIn(migrationCtx(false), sm, []string{vault.String()}, bridge, reader))

	csr, err := ConstructBaseView(sm)
	r.NoError(err)
	r.True(csr.GetByIdentifier(none.GetIdentifier()).VoterRewardOnchainOptIn,
		"pre-gate behaviour is migrate-on-reward-address, unchanged")
}

// A chain with no DelegateProfile contract, or a node with no reader injected,
// must not filter: treating an unreadable contract as "nobody is configured"
// would opt out every candidate rather than the misconfigured ones.
func TestMigrateHermesRewardOptInWithoutProfileSourceDoesNotFilter(t *testing.T) {
	r := require.New(t)
	for _, tc := range []struct {
		name   string
		bridge *delegateprofile.Bridge
		reader delegateprofile.ContractReader
	}{
		{"no contract configured", nil, nil},
		{"no reader injected", mustBridge(t), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sm := testdb.NewMockStateManager(gomock.NewController(t))
			vault := identityset.Address(10)
			cand := &Candidate{
				Owner: identityset.Address(1), Operator: identityset.Address(2), Reward: vault,
				Name: "c", Votes: big.NewInt(1), SelfStakeBucketIdx: 1, SelfStake: big.NewInt(1),
			}
			installCandCenter(t, sm, cand)
			r.NoError(migrateHermesRewardOptIn(
				migrationCtx(true), sm, []string{vault.String()}, tc.bridge, tc.reader))
			csr, err := ConstructBaseView(sm)
			r.NoError(err)
			r.True(csr.GetByIdentifier(cand.GetIdentifier()).VoterRewardOnchainOptIn)
		})
	}
}

func mustBridge(t *testing.T) *delegateprofile.Bridge {
	t.Helper()
	b, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	require.NoError(t, err)
	return b
}
