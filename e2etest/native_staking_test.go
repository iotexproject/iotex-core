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
)

var (
	selfStake, _     = big.NewInt(0).SetString("1200000000000000000000000", 10)
	cand1Votes, _    = big.NewInt(0).SetString("1635067133824581908640994", 10)
	vote, _          = big.NewInt(0).SetString("100000000000000000000", 10)
	autoStakeVote, _ = big.NewInt(0).SetString("103801784016923925869", 10)
	initBalance, _   = big.NewInt(0).SetString("100000000000000000000000000", 10)
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
			selfStake.String(),
		},
		{
			identityset.Address(24).String(),
			identityset.Address(25).String(),
			identityset.Address(25).String(),
			"test2",
			selfStake.String(),
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
		dao := svr.ChainService(chainID).BlockDAO()
		require.NotNil(bc)

		require.True(cfg.Genesis.IsFbkMigration(1))

		// Create two candidates
		cand1Addr := identityset.Address(0)
		cand1PriKey := identityset.PrivateKey(0)

		cand2Addr := identityset.Address(1)
		cand2PriKey := identityset.PrivateKey(1)

		fixedTime := time.Unix(cfg.Genesis.Timestamp, 0)
		addOneTx := func(tx action.SealedEnvelope, err error) (action.SealedEnvelope, error) {
			if err != nil {
				return tx, err
			}
			if err := ap.Add(ctx, tx); err != nil {
				return tx, err
			}
			if err := createAndCommitBlock(bc, ap, fixedTime); err != nil {
				return tx, err
			}
			return tx, nil
		}

		register1, err := addOneTx(action.SignedCandidateRegister(1, candidate1Name, cand1Addr.String(), cand1Addr.String(),
			cand1Addr.String(), selfStake.String(), 91, true, nil, gasLimit, gasPrice, cand1PriKey))
		require.NoError(err)
		register2, err := addOneTx(action.SignedCandidateRegister(1, candidate2Name, cand2Addr.String(), cand2Addr.String(),
			cand2Addr.String(), selfStake.String(), 1, false, nil, gasLimit, gasPrice, cand2PriKey))
		require.NoError(err)
		// check candidate state
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, cand1Votes, cand1Addr))
		require.NoError(checkCandidateState(sf, candidate2Name, cand2Addr.String(), selfStake, selfStake, cand2Addr))

		// check candidate account state
		require.NoError(checkAccountState(cfg, sf, register1, true, initBalance, cand1Addr))
		require.NoError(checkAccountState(cfg, sf, register2, true, initBalance, cand2Addr))

		// get self-stake index from receipts
		register1Hash, err := register1.Hash()
		require.NoError(err)
		r1, err := dao.GetReceiptByActionHash(register1Hash, 1)
		require.NoError(err)
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

		cs1, err := action.SignedCreateStake(1, candidate1Name, vote.String(), 1, false,
			nil, gasLimit, gasPrice, voter1PriKey)
		require.NoError(err)
		require.NoError(ap.Add(context.Background(), cs1))
		cs2, err := addOneTx(action.SignedCreateStake(1, candidate1Name, vote.String(), 1, false,
			nil, gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		// check candidate state
		expectedVotes := big.NewInt(0).Add(cand1Votes, big.NewInt(0).Mul(vote, big.NewInt(2)))
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, expectedVotes, cand1Addr))

		// check voter account state
		require.NoError(checkAccountState(cfg, sf, cs1, false, initBalance, voter1Addr))
		require.NoError(checkAccountState(cfg, sf, cs2, false, initBalance, voter2Addr))

		// get bucket index from receipts
		cs1Hash, err := cs1.Hash()
		require.NoError(err)
		r1, err = dao.GetReceiptByActionHash(cs1Hash, 3)
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_Success, r1.Status)
		logs = r1.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleCreateStake)), logs[0].Topics[0])
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])
		voter1BucketIndex := byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:])

		cs2Hash, err := cs2.Hash()
		require.NoError(err)
		r1, err = dao.GetReceiptByActionHash(cs2Hash, 3)
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_Success, r1.Status)
		logs = r1.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleCreateStake)), logs[0].Topics[0])
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])
		voter2BucketIndex := byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:])

		// change candidate
		cc, err := addOneTx(action.SignedChangeCandidate(2, candidate2Name, voter2BucketIndex, nil,
			gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		ccHash, err := cc.Hash()
		require.NoError(err)
		r1, err = dao.GetReceiptByActionHash(ccHash, 4)
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_Success, r1.Status)
		logs = r1.Logs()
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
		ts, err := addOneTx(action.SignedTransferStake(2, voter2Addr.String(), voter1BucketIndex, nil, gasLimit, gasPrice, voter1PriKey))
		require.NoError(err)

		tsHash, err := ts.Hash()
		require.NoError(err)
		r1, err = dao.GetReceiptByActionHash(tsHash, 5)
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_Success, r1.Status)
		logs = r1.Logs()
		require.Equal(4, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleTransferStake)), logs[0].Topics[0])
		require.Equal(voter1BucketIndex, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(voter2Addr.Bytes()), logs[0].Topics[2])
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[3])

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
		ds, err := addOneTx(action.SignedDepositToStake(3, voter2BucketIndex, vote.String(), nil, gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		dsHash, err := ds.Hash()
		require.NoError(err)
		r1, err = dao.GetReceiptByActionHash(dsHash, 6)
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_ErrInvalidBucketType, r1.Status)
		logs = r1.Logs()
		require.Equal(4, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleDepositToStake)), logs[0].Topics[0])
		require.Equal(voter2BucketIndex, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(voter2Addr.Bytes()), logs[0].Topics[2])
		require.Equal(hash.BytesToHash256(cand2Addr.Bytes()), logs[0].Topics[3])

		// restake
		rs, err := addOneTx(action.SignedRestake(4, voter2BucketIndex, 1, true, nil,
			gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		rsHash, err := rs.Hash()
		require.NoError(err)
		r1, err = dao.GetReceiptByActionHash(rsHash, 7)
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_Success, r1.Status)
		logs = r1.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleRestake)), logs[0].Topics[0])
		require.Equal(voter2BucketIndex, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(cand2Addr.Bytes()), logs[0].Topics[2])

		// check candidate state
		expectedVotes = big.NewInt(0).Add(selfStake, autoStakeVote)
		require.NoError(checkCandidateState(sf, candidate2Name, cand2Addr.String(), selfStake, expectedVotes, cand2Addr))

		// deposit to stake again
		ds, err = addOneTx(action.SignedDepositToStake(5, voter2BucketIndex, vote.String(), nil, gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		// check voter account state
		require.NoError(checkAccountState(cfg, sf, ds, false, big.NewInt(0).Sub(initBalance, vote), voter2Addr))

		// unstake voter stake
		us, err := addOneTx(action.SignedReclaimStake(false, 6, voter1BucketIndex, nil, gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		usHash, err := us.Hash()
		require.NoError(err)
		r1, err = dao.GetReceiptByActionHash(usHash, 9)
		require.NoError(err)
		require.Equal(uint64(iotextypes.ReceiptStatus_ErrUnstakeBeforeMaturity), r1.Status)
		logs = r1.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleUnstake)), logs[0].Topics[0])
		require.Equal(voter1BucketIndex, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])

		addOneTx = func(tx action.SealedEnvelope, err error) (action.SealedEnvelope, error) {
			unstakeTime := fixedTime.Add(time.Duration(1) * 24 * time.Hour)
			if err != nil {
				return tx, err
			}
			if err := ap.Add(ctx, tx); err != nil {
				return tx, err
			}
			if err := createAndCommitBlock(bc, ap, unstakeTime); err != nil {
				return tx, err
			}
			return tx, nil
		}

		// unstake with correct timestamp
		us, err = addOneTx(action.SignedReclaimStake(false, 7, voter1BucketIndex, nil, gasLimit, gasPrice, voter2PriKey))
		require.NoError(err)

		usHash, err = us.Hash()
		require.NoError(err)
		r1, err = dao.GetReceiptByActionHash(usHash, 10)
		require.NoError(err)
		require.Equal(uint64(iotextypes.ReceiptStatus_Success), r1.Status)
		logs = r1.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleUnstake)), logs[0].Topics[0])
		require.Equal(voter1BucketIndex, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])

		// check candidate state
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, cand1Votes, cand1Addr))

		// unstake self stake
		us, err = addOneTx(action.SignedReclaimStake(false, 2, selfstakeIndex1, nil, gasLimit, gasPrice, cand1PriKey))
		require.NoError(err)

		usHash, err = us.Hash()
		require.NoError(err)
		r1, err = dao.GetReceiptByActionHash(usHash, 11)
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_ErrInvalidBucketType, r1.Status)
		logs = r1.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleUnstake)), logs[0].Topics[0])
		require.Equal(selfstakeIndex1, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])

		// check candidate state
		require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, cand1Votes, cand1Addr))

		// withdraw stake
		ws, err := addOneTx(action.SignedReclaimStake(true, 3, selfstakeIndex1, nil, gasLimit, gasPrice, cand1PriKey))
		require.NoError(err)

		wsHash, err := ws.Hash()
		require.NoError(err)
		r1, err = dao.GetReceiptByActionHash(wsHash, 12)
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_ErrWithdrawBeforeUnstake, r1.Status)
		logs = r1.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleWithdrawStake)), logs[0].Topics[0])
		require.Equal(selfstakeIndex1, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])

		// withdraw	with correct timestamp
		ws, err = addOneTx(action.SignedReclaimStake(true, 4, selfstakeIndex1, nil, gasLimit, gasPrice, cand1PriKey))
		require.NoError(err)

		wsHash, err = ws.Hash()
		require.NoError(err)
		r1, err = dao.GetReceiptByActionHash(wsHash, 13)
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_ErrWithdrawBeforeUnstake, r1.Status)
		logs = r1.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleWithdrawStake)), logs[0].Topics[0])
		require.Equal(selfstakeIndex1, byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:]))
		require.Equal(hash.BytesToHash256(cand1Addr.Bytes()), logs[0].Topics[2])

		// check buckets
		_, err = sf.State(&bis, protocol.NamespaceOption(staking.StakingNameSpace),
			protocol.KeyOption(addrKeyWithPrefix(cand1Addr, _voterIndex)))
		require.NoError(err)
		require.Equal(1, len(bis))

		_, err = sf.State(&bis, protocol.NamespaceOption(staking.StakingNameSpace),
			protocol.KeyOption(addrKeyWithPrefix(cand1Addr, _candIndex)))
		require.NoError(err)
		require.Equal(2, len(bis))

		// check candidate account state
		require.NoError(checkAccountState(cfg, sf, ws, true, big.NewInt(0).Sub(initBalance, selfStake), cand1Addr))
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

	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.System.SystemLogDBPath = testSystemLogPath
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.BootstrapCandidates = testInitCands
	cfg.Genesis.FbkMigrationBlockHeight = 1

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
	expectedOwnerAddr string,
	expectedSelfStake,
	expectedVotes *big.Int,
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
	act action.SealedEnvelope,
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

func createAndCommitBlock(bc blockchain.Blockchain, ap actpool.ActPool, blkTime time.Time) error {
	blk, err := bc.MintNewBlock(blkTime)
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	ap.Reset()
	return nil
}
