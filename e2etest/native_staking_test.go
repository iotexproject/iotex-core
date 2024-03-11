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
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
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
	_stakingNameSpace   = "Staking"
	_candidateNameSpace = "Candidate"

	candidate1Name = "candidate1"
	candidate2Name = "candidate2"
	candidate3Name = "candidate3"
)

var (
	selfStake, _     = new(big.Int).SetString("1200000000000000000000000", 10)
	cand1Votes, _    = new(big.Int).SetString("1635067133824581908640994", 10)
	vote, _          = new(big.Int).SetString("100000000000000000000", 10)
	autoStakeVote, _ = new(big.Int).SetString("103801784016923925869", 10)
	initBalance, _   = new(big.Int).SetString("100000000000000000000000000", 10)
)

var (
	gasPrice = big.NewInt(0)
	gasLimit = uint64(1000000)
)

func TestNativeStaking(t *testing.T) {
	require := require.New(t)

	testInitCands := []genesis.BootstrapCandidate{
		{
			OwnerAddress:      identityset.Address(22).String(),
			OperatorAddress:   identityset.Address(23).String(),
			RewardAddress:     identityset.Address(23).String(),
			Name:              "test1",
			SelfStakingTokens: selfStake.String(),
		},
		{
			OwnerAddress:      identityset.Address(24).String(),
			OperatorAddress:   identityset.Address(25).String(),
			RewardAddress:     identityset.Address(25).String(),
			Name:              "test2",
			SelfStakingTokens: selfStake.String(),
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
		ap := svr.ChainService(chainID).ActionPool()
		require.NotNil(bc)
		prtcl, ok := svr.ChainService(chainID).Registry().Find("staking")
		require.True(ok)
		stkPrtcl := prtcl.(*staking.Protocol)

		require.True(cfg.Genesis.IsFbkMigration(1))

		// Create two candidates
		cand1Addr := identityset.Address(0)
		cand1PriKey := identityset.PrivateKey(0)

		cand2Addr := identityset.Address(1)
		cand2PriKey := identityset.PrivateKey(1)

		// create non-stake candidate
		cand3Addr := identityset.Address(4)
		cand3PriKey := identityset.PrivateKey(4)

		fixedTime := time.Unix(cfg.Genesis.Timestamp, 0)
		addOneTx := func(tx *action.SealedEnvelope, err error) (*action.SealedEnvelope, *action.Receipt, error) {
			if err != nil {
				return tx, nil, err
			}
			if err := ap.Add(ctx, tx); err != nil {
				return tx, nil, err
			}
			blk, err := createAndCommitBlock(bc, ap, fixedTime)
			if err != nil {
				return tx, nil, err
			}
			h, err := tx.Hash()
			if err != nil {
				return tx, nil, err
			}
			for _, r := range blk.Receipts {
				if r.ActionHash == h {
					return tx, r, nil
				}
			}
			return tx, nil, errors.Errorf("failed to find receipt for %x", h)
		}

		register1, r1, err := addOneTx(action.SignedCandidateRegister(1, candidate1Name, cand1Addr.String(), cand1Addr.String(),
			cand1Addr.String(), selfStake.String(), 91, true, nil, gasLimit, gasPrice, cand1PriKey))
		require.NoError(err)
		register2, _, err := addOneTx(action.SignedCandidateRegister(1, candidate2Name, cand2Addr.String(), cand2Addr.String(),
			cand2Addr.String(), selfStake.String(), 1, false, nil, gasLimit, gasPrice, cand2PriKey))
		require.NoError(err)
		// check candidate state
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, cand1Votes, cand1Addr))
		require.NoError(checkCandidateState(sf, candidate2Name, cand2Addr.String(), selfStake, selfStake, cand2Addr))

		// check candidate account state
		require.NoError(checkAccountState(cfg, sf, register1, true, initBalance, cand1Addr))
		require.NoError(checkAccountState(cfg, sf, register2, true, initBalance, cand2Addr))

		// get self-stake index from receipts
		require.EqualValues(iotextypes.ReceiptStatus_Success, r1.Status)
		logs := r1.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleCandidateRegister)), logs[0].Topics[0])
		selfstakeIndex1 := byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:])
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])

		// create two stakes from two voters
		voter1Addr := identityset.Address(2)
		voter1PriKey := identityset.PrivateKey(2)

		voter2Addr := identityset.Address(3)
		voter2PriKey := identityset.PrivateKey(3)

		cs1, r1, err := addOneTx(action.SignedCreateStake(1, candidate1Name, vote.String(), 1, false,
			nil, gasLimit, gasPrice, voter1PriKey))
		require.NoError(err)
		cs2, r2, err := addOneTx(action.SignedCreateStake(1, candidate1Name, vote.String(), 1, false,
			nil, gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		// check candidate state
		expectedVotes := big.NewInt(0).Add(cand1Votes, big.NewInt(0).Mul(vote, big.NewInt(2)))
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, expectedVotes, cand1Addr))

		// check voter account state
		require.NoError(checkAccountState(cfg, sf, cs1, false, initBalance, voter1Addr))
		require.NoError(checkAccountState(cfg, sf, cs2, false, initBalance, voter2Addr))

		// get bucket index from receipts
		require.EqualValues(iotextypes.ReceiptStatus_Success, r1.Status)
		logs = r1.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleCreateStake)), logs[0].Topics[0])
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])
		voter1BucketIndex := byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:])

		require.EqualValues(iotextypes.ReceiptStatus_Success, r2.Status)
		logs = r2.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleCreateStake)), logs[0].Topics[0])
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])
		voter2BucketIndex := byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:])

		// change candidate
		_, rc, err := addOneTx(action.SignedChangeCandidate(2, candidate2Name, voter2BucketIndex, nil,
			gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		require.EqualValues(iotextypes.ReceiptStatus_Success, rc.Status)
		logs = rc.Logs()
		require.Equal(4, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleChangeCandidate)), logs[0].Topics[0])
		require.Equal(voter2BucketIndex, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])
		require.Equal(hash.BytesToHash256(cand2Addr.Bytes()), logs[0].Topics[3])

		// check candidate state
		expectedVotes = big.NewInt(0).Add(cand1Votes, vote)
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, expectedVotes, cand1Addr))
		expectedVotes = big.NewInt(0).Add(selfStake, vote)
		require.NoError(checkCandidateState(sf, candidate2Name, cand2Addr.String(), selfStake, expectedVotes, cand2Addr))

		// transfer stake
		_, rt, err := addOneTx(action.SignedTransferStake(2, voter2Addr.String(), voter1BucketIndex, nil, gasLimit, gasPrice, voter1PriKey))
		require.NoError(err)

		require.EqualValues(iotextypes.ReceiptStatus_Success, rt.Status)
		logs = rt.Logs()
		require.Equal(4, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleTransferStake)), logs[0].Topics[0])
		require.Equal(voter1BucketIndex, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(voter2Addr.Bytes()), logs[0].Topics[2])
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[3])

		// check buckets
		var bis staking.BucketIndices
		_, err = sf.State(&bis, protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(staking.AddrKeyWithPrefix(voter1Addr, _voterIndex)))
		require.Error(err)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))

		_, err = sf.State(&bis, protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(staking.AddrKeyWithPrefix(voter2Addr, _voterIndex)))
		require.NoError(err)
		require.Equal(2, len(bis))
		require.Equal(voter2BucketIndex, bis[0])
		require.Equal(voter1BucketIndex, bis[1])

		// deposit to stake
		ds, rd, err := addOneTx(action.SignedDepositToStake(3, voter2BucketIndex, vote.String(), nil, gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		require.EqualValues(iotextypes.ReceiptStatus_ErrInvalidBucketType, rd.Status)
		logs = rd.Logs()
		require.Equal(4, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleDepositToStake)), logs[0].Topics[0])
		require.Equal(voter2BucketIndex, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(voter2Addr.Bytes()), logs[0].Topics[2])
		require.Equal(hash.BytesToHash256(cand2Addr.Bytes()), logs[0].Topics[3])

		// restake
		_, rr, err := addOneTx(action.SignedRestake(4, voter2BucketIndex, 1, true, nil,
			gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		require.EqualValues(iotextypes.ReceiptStatus_Success, rr.Status)
		logs = rr.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleRestake)), logs[0].Topics[0])
		require.Equal(voter2BucketIndex, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(cand2Addr.Bytes()), logs[0].Topics[2])

		// check candidate state
		expectedVotes = big.NewInt(0).Add(selfStake, autoStakeVote)
		require.NoError(checkCandidateState(sf, candidate2Name, cand2Addr.String(), selfStake, expectedVotes, cand2Addr))

		// deposit to stake again
		ds, rd, err = addOneTx(action.SignedDepositToStake(5, voter2BucketIndex, vote.String(), nil, gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		// check voter account state
		require.NoError(checkAccountState(cfg, sf, ds, false, big.NewInt(0).Sub(initBalance, vote), voter2Addr))

		// unstake voter stake
		_, ru, err := addOneTx(action.SignedReclaimStake(false, 6, voter1BucketIndex, nil, gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		require.Equal(uint64(iotextypes.ReceiptStatus_ErrUnstakeBeforeMaturity), ru.Status)
		logs = ru.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleUnstake)), logs[0].Topics[0])
		require.Equal(voter1BucketIndex, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])

		unstakeTime := fixedTime.Add(time.Duration(1) * 24 * time.Hour)
		addOneTx = func(tx *action.SealedEnvelope, err error) (*action.SealedEnvelope, *action.Receipt, error) {
			if err != nil {
				return tx, nil, err
			}
			if err := ap.Add(ctx, tx); err != nil {
				return tx, nil, err
			}
			blk, err := createAndCommitBlock(bc, ap, unstakeTime)
			if err != nil {
				return tx, nil, err
			}
			h, err := tx.Hash()
			if err != nil {
				return tx, nil, err
			}
			for _, r := range blk.Receipts {
				if r.ActionHash == h {
					return tx, r, nil
				}
			}
			return tx, nil, errors.Errorf("failed to find receipt for %x", h)
		}

		// unstake with correct timestamp
		_, ru, err = addOneTx(action.SignedReclaimStake(false, 7, voter1BucketIndex, nil, gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		require.Equal(uint64(iotextypes.ReceiptStatus_Success), ru.Status)
		logs = ru.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleUnstake)), logs[0].Topics[0])
		require.Equal(voter1BucketIndex, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])

		// check candidate state
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, cand1Votes, cand1Addr))

		// unstake self stake
		_, ru, err = addOneTx(action.SignedReclaimStake(false, 2, selfstakeIndex1, nil, gasLimit, gasPrice, cand1PriKey))
		require.NoError(err)

		require.EqualValues(iotextypes.ReceiptStatus_ErrInvalidBucketType, ru.Status)
		logs = ru.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleUnstake)), logs[0].Topics[0])
		require.Equal(selfstakeIndex1, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])

		// check candidate state
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, cand1Votes, cand1Addr))

		// withdraw stake
		ws, rw, err := addOneTx(action.SignedReclaimStake(true, 3, selfstakeIndex1, nil, gasLimit, gasPrice, cand1PriKey))
		require.NoError(err)

		require.EqualValues(iotextypes.ReceiptStatus_ErrWithdrawBeforeUnstake, rw.Status)
		logs = rw.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleWithdrawStake)), logs[0].Topics[0])
		require.Equal(selfstakeIndex1, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])

		// withdraw	with correct timestamp
		unstakeTime = unstakeTime.Add(cfg.Genesis.WithdrawWaitingPeriod)
		ws, rw, err = addOneTx(action.SignedReclaimStake(true, 4, selfstakeIndex1, nil, gasLimit, gasPrice, cand1PriKey))
		require.NoError(err)

		require.EqualValues(iotextypes.ReceiptStatus_ErrWithdrawBeforeUnstake, rw.Status)
		logs = rw.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleWithdrawStake)), logs[0].Topics[0])
		require.Equal(selfstakeIndex1, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])

		// check buckets
		_, err = sf.State(&bis, protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(staking.AddrKeyWithPrefix(cand1Addr, _voterIndex)))
		require.NoError(err)
		require.Equal(1, len(bis))

		_, err = sf.State(&bis, protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(staking.AddrKeyWithPrefix(cand1Addr, _candIndex)))
		require.NoError(err)
		require.Equal(2, len(bis))

		// check candidate account state
		require.NoError(checkAccountState(cfg, sf, ws, true, big.NewInt(0).Sub(initBalance, selfStake), cand1Addr))

		// register without stake
		register3, r3, err := addOneTx(action.SignedCandidateRegister(1, candidate3Name, cand3Addr.String(), cand3Addr.String(),
			cand3Addr.String(), "0", 1, false, nil, gasLimit, gasPrice, cand3PriKey))
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_Success, r3.Status)
		require.NoError(checkCandidateState(sf, candidate3Name, cand3Addr.String(), big.NewInt(0), big.NewInt(0), cand3Addr))
		require.NoError(checkAccountState(cfg, sf, register3, true, initBalance, cand3Addr))

		ctx, err = bc.Context(ctx)
		require.NoError(err)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight: bc.TipHeight() + 1,
		})
		ctx = protocol.WithFeatureCtx(ctx)
		cands, err := stkPrtcl.ActiveCandidates(ctx, sf, 0)
		require.NoError(err)
		require.Equal(4, len(cands))
		for _, cand := range cands {
			t.Logf("\ncandidate=%+v, %+v\n", string(cand.CanName), cand.Votes.String())
		}
		// stake bucket
		_, cr3, err := addOneTx(action.SignedCreateStake(3, candidate3Name, selfStake.String(), 1, false,
			nil, gasLimit, gasPrice, voter1PriKey))
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_Success, cr3.Status)
		logs = cr3.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleCreateStake)), logs[0].Topics[0])
		endorseBucketIndex := byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:])
		t.Logf("endorseBucketIndex=%+v", endorseBucketIndex)
		// endorse bucket
		_, esr, err := addOneTx(action.SignedCandidateEndorsement(4, endorseBucketIndex, true, gasLimit, gasPrice, voter1PriKey))
		require.NoError(err)
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_Success, esr.Status)
		// candidate self stake
		_, cssr, err := addOneTx(action.SignedCandidateActivate(2, endorseBucketIndex, gasLimit, gasPrice, cand3PriKey))
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_Success, cssr.Status)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight: bc.TipHeight() + 1,
		})
		ctx = protocol.WithFeatureCtx(ctx)
		cands, err = stkPrtcl.ActiveCandidates(ctx, sf, 0)
		require.NoError(err)
		require.Equal(5, len(cands))
		for _, cand := range cands {
			t.Logf("\ncandidate=%+v, %+v\n", string(cand.CanName), cand.Votes.String())
		}
		// unendorse bucket
		_, esr, err = addOneTx(action.SignedCandidateEndorsement(5, endorseBucketIndex, false, gasLimit, gasPrice, voter1PriKey))
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_Success, esr.Status)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight: bc.TipHeight() + 1,
		})
		cands, err = stkPrtcl.ActiveCandidates(ctx, sf, 0)
		require.NoError(err)
		require.Equal(5, len(cands))
		t.Run("endorsement is withdrawing, candidate can also be chosen as delegate", func(t *testing.T) {
			ctx, err = bc.Context(ctx)
			require.NoError(err)
			ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
				BlockHeight: bc.TipHeight(),
			})
			ctx = protocol.WithFeatureCtx(ctx)
			cands, err = stkPrtcl.ActiveCandidates(ctx, sf, 0)
			require.NoError(err)
			require.Equal(5, len(cands))
		})
		t.Run("endorsement is expired, candidate can not be chosen as delegate any more", func(t *testing.T) {
			ctx, err = bc.Context(ctx)
			require.NoError(err)
			jumpBlocks(bc, int(cfg.Genesis.EndorsementWithdrawWaitingBlocks), require)
			ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
				BlockHeight: bc.TipHeight(),
			})
			ctx = protocol.WithFeatureCtx(ctx)
			cands, err = stkPrtcl.ActiveCandidates(ctx, sf, 0)
			require.NoError(err)
			require.Equal(4, len(cands))
		})
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
	testSGDIndexPath, err := testutil.PathOfTempFile("sgdindex")
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
		testutil.CleanupPath(testSystemLogPath)
		testutil.CleanupPath(testSGDIndexPath)
		// clear the gateway
		delete(cfg.Plugins, config.GatewayPlugin)
	}()

	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.ContractStakingIndexDBPath = testIndexPath
	cfg.Chain.SGDIndexDBPath = testSGDIndexPath
	cfg.System.SystemLogDBPath = testSystemLogPath
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.BootstrapCandidates = testInitCands
	cfg.Genesis.FbkMigrationBlockHeight = 1
	cfg.Genesis.ToBeEnabledBlockHeight = 0
	cfg.Genesis.EndorsementWithdrawWaitingBlocks = 10

	t.Run("test native staking", func(t *testing.T) {
		testNativeStaking(cfg, t)
	})
}

func checkCandidateState(
	sr protocol.StateReader,
	expectedName,
	expectedOwnerAddr string,
	expectedSelfStake,
	expectedVotes *big.Int,
	candidateAddr address.Address,
) error {
	var cand staking.Candidate
	if _, err := sr.State(&cand, protocol.NamespaceOption(_candidateNameSpace), protocol.KeyOption(candidateAddr.Bytes())); err != nil {
		return err
	}
	if expectedName != cand.Name {
		return errors.New("name does not match")
	}
	if expectedOwnerAddr != cand.Owner.String() {
		return errors.New("Owner address does not match")
	}
	if expectedSelfStake.Cmp(cand.SelfStake) != 0 {
		return errors.New("self stake does not match")
	}
	if expectedVotes.Cmp(cand.Votes) != 0 {
		return errors.New("votes does not match")
	}
	return nil
}

func checkAccountState(
	cfg config.Config,
	sr protocol.StateReader,
	act *action.SealedEnvelope,
	registrationFee bool,
	expectedBalance *big.Int,
	accountAddr address.Address,
) error {
	cost, err := act.Cost()
	if err != nil {
		return err
	}
	if registrationFee {
		regFee, ok := new(big.Int).SetString(cfg.Genesis.RegistrationConsts.Fee, 10)
		if !ok {
			return errors.New("failed to set genesis registration fee")
		}
		cost.Add(cost, regFee)
	}
	acct1, err := accountutil.LoadAccount(sr, accountAddr)
	if err != nil {
		return err
	}
	if expectedBalance.Cmp(cost.Add(cost, acct1.Balance)) != 0 {
		return errors.New("balance does not match")
	}
	return nil
}

func createAndCommitBlock(bc blockchain.Blockchain, ap actpool.ActPool, blkTime time.Time) (*block.Block, error) {
	blk, err := bc.MintNewBlock(blkTime)
	if err != nil {
		return nil, err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return nil, err
	}
	ap.Reset()
	return blk, nil
}
