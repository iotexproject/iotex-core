package e2etest

import (
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/poll"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func initExitQueueCfg(r *require.Assertions) config.Config {
	cfg := initCfg(r)
	cfg.Genesis.VanuatuBlockHeight = 1
	cfg.Genesis.ToBeEnabledBlockHeight = 2
	cfg.Genesis.ExitAdmissionInterval = 1
	// Use small epoch size for faster epoch transitions
	cfg.Genesis.DardanellesNumSubEpochs = 1
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	return cfg
}

// registerEpochProtocols registers rolldpos and poll protocols into the chain service registry.
// The e2e test framework uses NOOPScheme which skips these registrations.
// The exit queue feature's CreatePostSystemActions depends on rolldpos for epoch
// calculations, and the rewarding protocol's epoch reward handler needs poll protocol.
func registerEpochProtocols(r *require.Assertions, test *e2etest) {
	cfg := test.cfg
	rp := rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.NumSubEpochs,
		rolldpos.EnableDardanellesSubEpoch(cfg.Genesis.DardanellesBlockHeight, cfg.Genesis.DardanellesNumSubEpochs),
	)
	r.NoError(rp.ForceRegister(test.cs.Registry()))
	pp := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
	r.NoError(pp.ForceRegister(test.cs.Registry()))
}

func TestExitQueue(t *testing.T) {
	r := require.New(t)
	registerAmount, _ := new(big.Int).SetString("1200000000000000000000000", 10)
	gasLimit := uint64(10000000)
	gasPrice := big.NewInt(unit.Qev*2 + 1)

	// With NumDelegates=24 (default), DardanellesNumSubEpochs=1, ExitAdmissionInterval=1:
	//   blocks per epoch = 24
	//   Epoch 1: heights 1-24 (start=1)
	//   Epoch 2: heights 25-48 (start=25)
	//   Epoch 3: heights 49-72 (start=49)

	t.Run("exit queue lifecycle: request → schedule → confirm", func(t *testing.T) {
		cfg := initExitQueueCfg(r)
		test := newE2ETest(t, cfg)
		defer test.teardown()
		registerEpochProtocols(r, test)

		ownerID := 1
		chainID := test.cfg.Chain.ID

		test.run([]*testcase{
			{
				// height 1: register candidate with self-stake
				name:   "register candidate with self-stake",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(ownerID).String()), "cand1", identityset.Address(ownerID).String(), identityset.Address(ownerID).String(), identityset.Address(ownerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect},
			},
			{
				// height 2: request exit
				name: "request exit",
				act:  &actionWithTime{mustNoErr(action.SignedCandidateDeactivate(test.nonceMgr.pop(identityset.Address(ownerID).String()), action.CandidateDeactivateOpRequest, gasLimit, gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					successExpect,
					&functionExpect{fn: func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
						r := require.New(test.t)
						cand, err := test.getCandidateByName("cand1")
						r.NoError(err)
						r.Equal(uint64(math.MaxUint64), cand.DeactivatedAt)
						r.Equal(registerAmount.String(), cand.SelfStakingTokens)
					}},
				},
			},
			{
				// Advance to epoch 2 start (height 25). The system generates
				// ScheduleCandidateDeactivation at epoch start, setting
				// DeactivatedAt = 25 + 1*24 = 49.
				name: "system schedules deactivation at epoch 2 start",
				preFunc: func(e *e2etest) {
					bc := e.cs.Blockchain()
					ap := e.cs.ActionPool()
					for bc.TipHeight() < 24 {
						_, err := createAndCommitBlock(bc, ap, time.Now())
						require.NoError(e.t, err)
					}
				},
				act: &actionWithTime{mustNoErr(action.SignedTransfer(identityset.Address(ownerID).String(), identityset.PrivateKey(ownerID), test.nonceMgr.pop(identityset.Address(ownerID).String()), big.NewInt(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					successExpect,
					&functionExpect{fn: func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
						r := require.New(test.t)
						cand, err := test.getCandidateByName("cand1")
						r.NoError(err)
						r.NotEqual(uint64(math.MaxUint64), cand.DeactivatedAt)
						r.NotEqual(uint64(0), cand.DeactivatedAt)
						r.Equal(uint64(49), cand.DeactivatedAt)
						r.Equal(registerAmount.String(), cand.SelfStakingTokens)
					}},
				},
			},
			{
				// Advance past DeactivatedAt (49), then confirm exit
				name: "confirm exit after DeactivatedAt reached",
				preFunc: func(e *e2etest) {
					bc := e.cs.Blockchain()
					ap := e.cs.ActionPool()
					for bc.TipHeight() < 48 {
						_, err := createAndCommitBlock(bc, ap, time.Now())
						require.NoError(e.t, err)
					}
				},
				act: &actionWithTime{mustNoErr(action.SignedCandidateDeactivate(test.nonceMgr.pop(identityset.Address(ownerID).String()), action.CandidateDeactivateOpConfirm, gasLimit, gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					successExpect,
					&functionExpect{fn: func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
						r := require.New(test.t)
						cand, err := test.getCandidateByName("cand1")
						r.NoError(err)
						r.Equal("0", cand.SelfStakingTokens)
						r.Equal(uint64(math.MaxUint64), cand.SelfStakeBucketIdx)
					}},
				},
			},
		})
	})

	t.Run("self-stake bucket cannot be directly unstaked", func(t *testing.T) {
		cfg := initExitQueueCfg(r)
		test := newE2ETest(t, cfg)
		defer test.teardown()

		ownerID := 1
		chainID := test.cfg.Chain.ID

		test.run([]*testcase{
			{
				name: "register candidate with self-stake",
				act:  &actionWithTime{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(ownerID).String()), "cand1", identityset.Address(ownerID).String(), identityset.Address(ownerID).String(), identityset.Address(ownerID).String(), registerAmount.String(), 0, false, nil, gasLimit, gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					successExpect,
					&functionExpect{fn: func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
						r := require.New(test.t)
						cand, err := test.getCandidateByName("cand1")
						r.NoError(err)
						r.Equal(uint64(0), cand.SelfStakeBucketIdx)
					}},
				},
			},
			{
				name:   "cannot unstake self-stake bucket directly",
				act:    &actionWithTime{mustNoErr(action.SignedReclaimStake(false, test.nonceMgr.pop(identityset.Address(ownerID).String()), 0, nil, gasLimit, gasPrice, identityset.PrivateKey(ownerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrUnstakeBeforeMaturity), ""}},
			},
		})
	})
}
