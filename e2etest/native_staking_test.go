package e2etest

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	// TODO: to be removed
	_const = byte(iota)
	_bucket
	_voterIndex
	_candIndex

	candidate1Name = "candidate1"
	candidate2Name = "candidate2"
	selfStake      = "1200000000000000000000000"
	initVotes      = "1260000000000000053290706"
	vote           = "100000000000000000000"
	autoStakeVote  = "103801784016923925869"
	initBalance    = "100000000000000000000000000"
)

var (
	gasPrice = big.NewInt(0)
	gasLimit = uint64(1000000)
)

func TestNativeStaking(t *testing.T) {
	require := require.New(t)

	testInitCands := []genesis.BootstrapCandidate{
		{
			identityset.Address(22).String(),
			identityset.Address(23).String(),
			identityset.Address(23).String(),
			"test1",
			selfStake,
		},
		{
			identityset.Address(24).String(),
			identityset.Address(25).String(),
			identityset.Address(25).String(),
			"test2",
			selfStake,
		},
	}

	testNativeStaking := func(cfg config.Config, t *testing.T) {
		ctx := context.Background()

		// Create a new blockchain
		svr, err := itx.NewServer(cfg)
		require.NoError(err)
		require.NoError(svr.Start(ctx))
		defer func() {
			require.NoError(svr.Stop(ctx))
		}()

		chainID := cfg.Chain.ID
		bc := svr.ChainService(chainID).Blockchain()
		sf := svr.ChainService(chainID).StateFactory()
		dao := svr.ChainService(chainID).BlockDAO()
		require.NotNil(bc)

		// Create two candidates
		cand1Addr := identityset.Address(0)
		cand1PriKey := identityset.PrivateKey(0)

		cand2Addr := identityset.Address(1)
		cand2PriKey := identityset.PrivateKey(1)

		register1, err := testutil.SignedCandidateRegister(1, candidate1Name, cand1Addr.String(), cand1Addr.String(),
			cand1Addr.String(), selfStake, 1, false, nil, gasLimit, gasPrice, cand1PriKey)
		require.NoError(err)
		register2, err := testutil.SignedCandidateRegister(1, candidate2Name, cand2Addr.String(), cand2Addr.String(),
			cand2Addr.String(), selfStake, 1, false, nil, gasLimit, gasPrice, cand2PriKey)
		require.NoError(err)

		fixedTime := time.Unix(cfg.Genesis.Timestamp, 0)
		require.NoError(createAndCommitBlock(bc, []address.Address{cand1Addr},
			[]action.SealedEnvelope{register1}, fixedTime))
		require.NoError(createAndCommitBlock(bc, []address.Address{cand2Addr},
			[]action.SealedEnvelope{register2}, fixedTime))

		// check candidate state
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, initVotes, cand1Addr))
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, initVotes, cand1Addr))

		// check candidate account state
		require.NoError(checkAccountState(cfg, sf, register1, true, initBalance, cand1Addr))
		require.NoError(checkAccountState(cfg, sf, register2, true, initBalance, cand2Addr))

		// get self-stake index from receipts
		r1, err := dao.GetReceiptByActionHash(register1.Hash(), 1)
		require.NoError(err)
		require.Equal(1, len(r1.Logs))
		selfstakeIndex1 := byteutil.BytesToUint64BigEndian(r1.Logs[0].Data)

		// create two stakes from two voters
		voter1Addr := identityset.Address(2)
		voter1PriKey := identityset.PrivateKey(2)

		voter2Addr := identityset.Address(3)
		voter2PriKey := identityset.PrivateKey(3)

		cs1, err := testutil.SignedCreateStake(1, candidate1Name, vote, 1, false,
			nil, gasLimit, gasPrice, voter1PriKey)
		require.NoError(err)
		cs2, err := testutil.SignedCreateStake(1, candidate1Name, vote, 1, false,
			nil, gasLimit, gasPrice, voter2PriKey)
		require.NoError(err)

		require.NoError(createAndCommitBlock(bc, []address.Address{voter1Addr, voter2Addr},
			[]action.SealedEnvelope{cs1, cs2}, fixedTime))

		// check candidate state
		wv, _ := big.NewInt(0).SetString(vote, 10)
		expectedVotes, _ := big.NewInt(0).SetString(initVotes, 10)
		expectedVotes.Add(expectedVotes, big.NewInt(0).Mul(wv, big.NewInt(2)))
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, expectedVotes.String(), cand1Addr))

		// check voter account state
		require.NoError(checkAccountState(cfg, sf, cs1, false, initBalance, voter1Addr))
		require.NoError(checkAccountState(cfg, sf, cs2, false, initBalance, voter2Addr))

		// get bucket index from receipts
		r1, err = dao.GetReceiptByActionHash(cs1.Hash(), 3)
		require.NoError(err)
		require.Equal(1, len(r1.Logs))

		r2, err := dao.GetReceiptByActionHash(cs2.Hash(), 3)
		require.NoError(err)
		require.Equal(1, len(r2.Logs))

		voter1BucketIndex := byteutil.BytesToUint64BigEndian(r1.Logs[0].Data)
		voter2BucketIndex := byteutil.BytesToUint64BigEndian(r2.Logs[0].Data)

		// change candidate
		cc, err := testutil.SignedChangeCandidate(2, candidate2Name, voter2BucketIndex, nil,
			gasLimit, gasPrice, voter2PriKey)
		require.NoError(err)

		require.NoError(createAndCommitBlock(bc, []address.Address{voter2Addr}, []action.SealedEnvelope{cc}, fixedTime))

		// check candidate state
		expectedVotes, _ = big.NewInt(0).SetString(initVotes, 10)
		expectedVotes.Add(expectedVotes, wv)
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, expectedVotes.String(), cand1Addr))
		require.NoError(checkCandidateState(sf, candidate2Name, cand2Addr.String(), selfStake, expectedVotes.String(), cand2Addr))

		// transfer stake
		ts, err := testutil.SignedTransferStake(2, voter2Addr.String(), voter1BucketIndex, nil, gasLimit, gasPrice, voter1PriKey)
		require.NoError(err)
		require.NoError(createAndCommitBlock(bc, []address.Address{voter1Addr}, []action.SealedEnvelope{ts}, fixedTime))

		// check buckets
		var bis staking.BucketIndices
		_, err = sf.State(&bis, protocol.NamespaceOption(staking.StakingNameSpace),
			protocol.KeyOption(addrKeyWithPrefix(voter1Addr, _voterIndex)))
		require.Error(err)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))

		_, err = sf.State(&bis, protocol.NamespaceOption(staking.StakingNameSpace),
			protocol.KeyOption(addrKeyWithPrefix(voter2Addr, _voterIndex)))
		require.NoError(err)
		require.Equal(2, len(bis))
		require.Equal(voter2BucketIndex, bis[0])
		require.Equal(voter1BucketIndex, bis[1])

		// deposit to stake
		ds, err := testutil.SignedDepositToStake(3, voter2BucketIndex, vote, nil, gasLimit, gasPrice, voter2PriKey)
		require.NoError(err)
		require.NoError(createAndCommitBlock(bc, []address.Address{voter2Addr}, []action.SealedEnvelope{ds}, fixedTime))
		r, err := dao.GetReceiptByActionHash(ds.Hash(), 6)
		require.NoError(err)
		require.Equal(uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), r.Status)

		// restake
		rs, err := testutil.SignedRestake(3, voter2BucketIndex, 1, true, nil,
			gasLimit, gasPrice, voter2PriKey)
		require.NoError(err)
		require.NoError(createAndCommitBlock(bc, []address.Address{voter2Addr}, []action.SealedEnvelope{rs}, fixedTime))

		// check candidate state
		expectedVotes, _ = big.NewInt(0).SetString(initVotes, 10)
		wv, _ = big.NewInt(0).SetString(autoStakeVote, 10)
		expectedVotes.Add(expectedVotes, wv)
		require.NoError(checkCandidateState(sf, candidate2Name, cand2Addr.String(), selfStake, expectedVotes.String(), cand2Addr))

		// deposit to stake again
		ds, err = testutil.SignedDepositToStake(3, voter2BucketIndex, vote, nil, gasLimit, gasPrice, voter2PriKey)
		require.NoError(err)
		require.NoError(createAndCommitBlock(bc, []address.Address{voter2Addr}, []action.SealedEnvelope{ds}, fixedTime))

		// check voter account state
		voterBalance, _ := big.NewInt(0).SetString(initBalance, 10)
		vote, _ := big.NewInt(0).SetString(vote, 10)
		voterBalance.Sub(voterBalance, vote)
		require.NoError(checkAccountState(cfg, sf, ds, false, voterBalance.String(), voter2Addr))

		// unstake voter stake
		us, err := testutil.SignedReclaimStake(false, 4, voter1BucketIndex, nil, gasLimit, gasPrice, voter2PriKey)
		require.NoError(err)
		require.NoError(createAndCommitBlock(bc, []address.Address{voter2Addr}, []action.SealedEnvelope{us}, fixedTime))

		// check candidate state
		expectedVotes, _ = big.NewInt(0).SetString(initVotes, 10)
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, expectedVotes.String(), cand1Addr))

		// unstake self stake
		us, err = testutil.SignedReclaimStake(false, 2, selfstakeIndex1, nil, gasLimit, gasPrice, cand1PriKey)
		require.NoError(err)
		require.NoError(createAndCommitBlock(bc, []address.Address{cand1Addr}, []action.SealedEnvelope{us}, fixedTime))

		// check candidate state
		expectedVotes = big.NewInt(0)
		expectedSelfStake := big.NewInt(0)
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), expectedSelfStake.String(),
			expectedVotes.String(), cand1Addr))

		// withdraw stake
		ws, err := testutil.SignedReclaimStake(true, 3, selfstakeIndex1, nil, gasLimit, gasPrice, cand1PriKey)
		require.NoError(err)
		require.NoError(createAndCommitBlock(bc, []address.Address{cand1Addr}, []action.SealedEnvelope{ws}, fixedTime))
		r, err = dao.GetReceiptByActionHash(ws.Hash(), 11)
		require.NoError(err)
		require.Equal(uint64(iotextypes.ReceiptStatus_ErrWithdrawBeforeMaturity), r.Status)

		require.NoError(createAndCommitBlock(bc, []address.Address{cand1Addr}, []action.SealedEnvelope{ws}, fixedTime.Add(cfg.Genesis.WithdrawWaitingPeriod)))

		// check buckets
		_, err = sf.State(&bis, protocol.NamespaceOption(staking.StakingNameSpace),
			protocol.KeyOption(addrKeyWithPrefix(cand1Addr, _voterIndex)))
		require.Error(err)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))

		_, err = sf.State(&bis, protocol.NamespaceOption(staking.StakingNameSpace),
			protocol.KeyOption(addrKeyWithPrefix(cand1Addr, _candIndex)))
		require.NoError(err)
		require.Equal(1, len(bis))

		// check candidate account state
		candidateBalance, _ := big.NewInt(0).SetString(initBalance, 10)
		registrationFee, ok := new(big.Int).SetString(cfg.Genesis.RegistrationConsts.Fee, 10)
		require.True(ok)
		candidateBalance.Sub(candidateBalance, registrationFee)
		require.NoError(checkAccountState(cfg, sf, ws, false, candidateBalance.String(), cand1Addr))
	}

	cfg := config.Default
	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)
	testSystemLogPath, err := testutil.PathOfTempFile("systemlog")
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
		testutil.CleanupPath(t, testIndexPath)
		testutil.CleanupPath(t, testSystemLogPath)
		// clear the gateway
		delete(cfg.Plugins, config.GatewayPlugin)
	}()

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.System.SystemLogDBPath = testSystemLogPath
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.BootstrapCandidates = testInitCands

	t.Run("test native staking", func(t *testing.T) {
		testNativeStaking(cfg, t)
	})
}

// TODO: to be removed
func addrKeyWithPrefix(voterAddr address.Address, prefix byte) []byte {
	k := voterAddr.Bytes()
	key := make([]byte, len(k)+1)
	key[0] = prefix
	copy(key[1:], k)
	return key
}

func checkCandidateState(
	sr protocol.StateReader,
	expectedName,
	expectedOwnerAddr,
	expectedSelfStake,
	expectedVotes string,
	candidateAddr address.Address,
) error {
	var cand staking.Candidate
	if _, err := sr.State(&cand, protocol.NamespaceOption(staking.CandidateNameSpace), protocol.KeyOption(candidateAddr.Bytes())); err != nil {
		return err
	}
	if expectedName != cand.Name {
		return errors.New("name does not match")
	}
	if expectedOwnerAddr != cand.Owner.String() {
		return errors.New("Owner address does not match")
	}
	if expectedSelfStake != cand.SelfStake.String() {
		return errors.New("self stake does not match")
	}
	if expectedVotes != cand.Votes.String() {
		return errors.New("votes does not match")
	}
	return nil
}

func checkAccountState(
	cfg config.Config,
	sr protocol.StateReader,
	act action.SealedEnvelope,
	registrationFee bool,
	expectedBalance string,
	accountAddr address.Address,
) error {
	cost, err := act.Cost()
	if err != nil {
		return err
	}
	if registrationFee {
		regFee, _ := new(big.Int).SetString(cfg.Genesis.RegistrationConsts.Fee, 10)
		cost.Add(cost, regFee)
	}
	acct1, err := accountutil.LoadAccount(sr, hash.BytesToHash160(accountAddr.Bytes()))
	if err != nil {
		return err
	}
	if expectedBalance != big.NewInt(0).Add(acct1.Balance, cost).String() {
		return errors.New("balance does not match")
	}
	return nil
}

func createAndCommitBlock(bc blockchain.Blockchain, callers []address.Address, selps []action.SealedEnvelope, blkTime time.Time) error {
	if len(callers) != len(selps) {
		return errors.New("incorrect input")
	}
	accMap := make(map[string][]action.SealedEnvelope)
	for i, caller := range callers {
		accMap[caller.String()] = []action.SealedEnvelope{selps[i]}
	}
	blk, err := bc.MintNewBlock(accMap, blkTime)
	if err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}
