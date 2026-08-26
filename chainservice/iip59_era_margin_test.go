// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

// testnetRollDPoS builds the epoch calculator from the live testnet genesis
// (iotex-bootstrap/genesis_testnet.yaml): 24 delegates, 15 sub-epochs, with the
// Dardanelles and Wake sub-epoch changes at their real heights.
func testnetRollDPoS() *rolldpos.Protocol {
	return rolldpos.NewProtocol(
		36, 24, 15,
		rolldpos.EnableDardanellesSubEpoch(100081, 30),
		rolldpos.EnableWakeSubEpoch(31943521, 60),
	)
}

func testnetGenesis(activation uint64) genesis.Genesis {
	g := genesis.TestDefault()
	g.NumDelegates = 24
	g.NumSubEpochs = 15
	g.DardanellesBlockHeight = 100081
	g.DardanellesNumSubEpochs = 30
	g.WakeBlockHeight = 31943521
	g.WakeNumSubEpochs = 60
	g.Rewarding.EpochsPerRewardEra = 24
	g.ToBeEnabledBlockHeight = activation
	return g
}

// The arithmetic is calibrated against a settlement that actually happened.
// TestNet froze era 54888 at height 46,892,881 and settled it at 46,895,040 --
// values read off the chain, not derived here. If eraFreezeHeight disagrees with
// them the rule is measuring the wrong thing, and every other case below is
// meaningless.
func TestEraFreezeHeightMatchesObservedTestnetSettlement(t *testing.T) {
	r := require.New(t)
	rp := testnetRollDPoS()

	r.Equal(uint64(54879), rp.GetEpochNum(46880641), "IIP-59 activated in epoch 54879 on the test network")
	r.Equal(uint64(46893601), rp.GetEpochHeight(54888), "era boundary epoch 54888 starts here")
	r.Equal(uint64(46892881), eraFreezeHeight(rp, 54888), "observed on TestNet")
	r.Equal(uint64(46895040),
		rp.GetEpochHeight(54888)+rp.NumBlocksByEpoch(54888)-1, "observed settlement height")

	// The next two eras, also observed: freeze exactly one era length apart.
	r.Equal(uint64(46927441), eraFreezeHeight(rp, 54912))
	r.Equal(uint64(46962001), eraFreezeHeight(rp, 54936))
}

// The rule must not reject the configuration the fleet is running. This check
// runs at startup, and Genesis.validate() is skipped for defaultConfig() and
// test literals, so a rule stricter than the mechanism would take every node
// down on restart with nothing having caught it first.
func TestLiveConfigurationsPass(t *testing.T) {
	r := require.New(t)

	r.NoError(validateIIP59EraMargin(testnetGenesis(46880641), testnetRollDPoS()),
		"a real deployed activation height must remain valid")

	mainnet := testnetGenesis(math.MaxUint64)
	r.NoError(validateIIP59EraMargin(mainnet, testnetRollDPoS()),
		"an unscheduled fork is not constrained")
}

func TestIIP59EraMarginBoundary(t *testing.T) {
	r := require.New(t)
	rp := testnetRollDPoS()
	freeze := eraFreezeHeight(rp, 54888) // 46,892,881

	r.NoError(validateIIP59EraMargin(testnetGenesis(freeze), rp),
		"activating exactly at the freeze still opens the window")

	err := validateIIP59EraMargin(testnetGenesis(freeze+1), rp)
	r.Error(err, "one block past the freeze loses era 54888's epoch reward entirely")
	r.ErrorContains(err, "past the freeze at height 46892881")
	r.ErrorContains(err, "era 54888")
	// The message has to be actionable: an operator should not have to rederive
	// the epoch arithmetic to pick a working height.
	r.ErrorContains(err, "at or below 46892881")

	// Landing inside the boundary epoch itself is the easiest way to misconfigure
	// this, and the intuitive rule ("activate at the start of an era") produces
	// exactly it.
	r.Error(validateIIP59EraMargin(testnetGenesis(rp.GetEpochHeight(54888)), rp),
		"the first block of a boundary epoch is already too late")

	// Just after the boundary epoch ends, the next era is the first one this
	// height owns, and its freeze is a full era away.
	afterBoundary := rp.GetEpochHeight(54888) + rp.NumBlocksByEpoch(54888)
	r.NoError(validateIIP59EraMargin(testnetGenesis(afterBoundary), rp))
}

// Degenerate inputs must not turn a startup check into a startup crash.
func TestIIP59EraMarginDegenerateInputs(t *testing.T) {
	r := require.New(t)

	r.NoError(validateIIP59EraMargin(testnetGenesis(46880641), nil),
		"no rolldpos protocol means no epochs to reason about")

	g := testnetGenesis(46880641)
	g.Rewarding.EpochsPerRewardEra = 0
	r.NoError(validateIIP59EraMargin(g, testnetRollDPoS()),
		"era length 0 never settles an era; Genesis.validate rejects it separately")

	r.NoError(validateIIP59EraMargin(testnetGenesis(0), testnetRollDPoS()))
}
