package e2etest

import (
	"context"
	"encoding/hex"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
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
			cand1Addr.String(), selfStake.String(), 91, true, nil, gasLimit, gasPrice, cand1PriKey, action.WithChainID(chainID)))
		require.NoError(err)
		register2, _, err := addOneTx(action.SignedCandidateRegister(1, candidate2Name, cand2Addr.String(), cand2Addr.String(),
			cand2Addr.String(), selfStake.String(), 1, false, nil, gasLimit, gasPrice, cand2PriKey, action.WithChainID(chainID)))
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
			nil, gasLimit, gasPrice, voter1PriKey, action.WithChainID(chainID)))
		require.NoError(err)
		cs2, r2, err := addOneTx(action.SignedCreateStake(1, candidate1Name, vote.String(), 1, false,
			nil, gasLimit, gasPrice, voter2PriKey, action.WithChainID(chainID)))
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
			cand3Addr.String(), "0", 1, false, nil, gasLimit, gasPrice, cand3PriKey, action.WithChainID(chainID)))
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
			nil, gasLimit, gasPrice, voter1PriKey, action.WithChainID(chainID)))
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_Success, cr3.Status)
		logs = cr3.Logs()
		require.Equal(3, len(logs[0].Topics))
		require.Equal(hash.BytesToHash256([]byte(staking.HandleCreateStake)), logs[0].Topics[0])
		endorseBucketIndex := byteutil.BytesToUint64BigEndian(logs[0].Topics[1][24:])
		t.Logf("endorseBucketIndex=%+v", endorseBucketIndex)
		// endorse bucket
		_, esr, err := addOneTx(action.SignedCandidateEndorsement(4, endorseBucketIndex, true, gasLimit, gasPrice, voter1PriKey, action.WithChainID(chainID)))
		require.NoError(err)
		require.NoError(err)
		require.EqualValues(iotextypes.ReceiptStatus_Success, esr.Status)
		// candidate self stake
		_, cssr, err := addOneTx(action.SignedCandidateActivate(2, endorseBucketIndex, gasLimit, gasPrice, cand3PriKey, action.WithChainID(chainID)))
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
		_, esr, err = addOneTx(action.SignedCandidateEndorsement(5, endorseBucketIndex, false, gasLimit, gasPrice, voter1PriKey, action.WithChainID(chainID)))
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
		t.Run("candidate transfer ownership to self", func(t *testing.T) {
			_, ccto, err := addOneTx(action.SignedCandidateTransferOwnership(5, cand1Addr.String(), nil, gasLimit, gasPrice, cand1PriKey, action.WithChainID(chainID)))
			require.NoError(err)
			require.EqualValues(iotextypes.ReceiptStatus_ErrUnauthorizedOperator, ccto.Status)
			require.NoError(checkCandidateState(sf, candidate1Name, cand1Addr.String(), selfStake, cand1Votes, cand1Addr))
		})

		t.Run("candidate transfer ownership to a normal new address", func(t *testing.T) {
			newOwner1 := identityset.Address(33)
			_, ccto, err := addOneTx(action.SignedCandidateTransferOwnership(6, newOwner1.String(), nil, gasLimit, gasPrice, cand1PriKey, action.WithChainID(chainID)))
			require.NoError(err)
			require.EqualValues(iotextypes.ReceiptStatus_Success, ccto.Status)
			require.NoError(checkCandidateState(sf, candidate1Name, newOwner1.String(), selfStake, cand1Votes, cand1Addr))
		})
		t.Run("candidate transfer ownership to a exist candidate", func(t *testing.T) {
			_, ccto, err := addOneTx(action.SignedCandidateTransferOwnership(7, cand2Addr.String(), nil, gasLimit, gasPrice, cand1PriKey, action.WithChainID(chainID)))
			require.NoError(err)
			require.EqualValues(iotextypes.ReceiptStatus_ErrUnauthorizedOperator, ccto.Status)
			require.NoError(checkCandidateState(sf, candidate1Name, identityset.Address(33).String(), selfStake, cand1Votes, cand1Addr))
		})
		t.Run("candidate transfer ownership to a normal new address again", func(t *testing.T) {
			newOwner2 := identityset.Address(34)
			_, ccto, err := addOneTx(action.SignedCandidateTransferOwnership(8, newOwner2.String(), nil, gasLimit, gasPrice, cand1PriKey, action.WithChainID(chainID)))
			require.NoError(err)
			require.EqualValues(iotextypes.ReceiptStatus_Success, ccto.Status)
			require.NoError(checkCandidateState(sf, candidate1Name, newOwner2.String(), selfStake, cand1Votes, cand1Addr))
		})
		t.Run("candidate transfer ownership to a transfered candidate", func(t *testing.T) {
			newOwner2 := identityset.Address(34)
			_, ccto, err := addOneTx(action.SignedCandidateTransferOwnership(9, newOwner2.String(), nil, gasLimit, gasPrice, cand1PriKey, action.WithChainID(chainID)))
			require.NoError(err)
			require.EqualValues(iotextypes.ReceiptStatus_ErrUnauthorizedOperator, ccto.Status)
			require.NoError(checkCandidateState(sf, candidate1Name, newOwner2.String(), selfStake, cand1Votes, cand1Addr))
		})
		t.Run("candidate transfer ownership to a contract address", func(t *testing.T) {
			data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
			_, se, err := addOneTx(action.SignedExecution(action.EmptyAddress, cand1PriKey, 10, big.NewInt(0), uint64(100000), big.NewInt(0), data))
			require.NoError(err)
			require.EqualValues(iotextypes.ReceiptStatus_Success, se.Status)
			_, ccto, err := addOneTx(action.SignedCandidateTransferOwnership(11, se.ContractAddress, nil, gasLimit, gasPrice, cand1PriKey, action.WithChainID(chainID)))
			require.NoError(err)
			require.EqualValues(iotextypes.ReceiptStatus_ErrUnauthorizedOperator, ccto.Status)
			require.NoError(checkCandidateState(sf, candidate1Name, identityset.Address(34).String(), selfStake, cand1Votes, cand1Addr))
		})
		t.Run("candidate transfer ownership to a invalid address", func(t *testing.T) {
			_, _, err := addOneTx(action.SignedCandidateTransferOwnership(12, "123", nil, gasLimit, gasPrice, cand1PriKey, action.WithChainID(chainID)))
			require.ErrorContains(err, action.ErrAddress.Error())
		})
		t.Run("candidate transfer ownership with none candidate", func(t *testing.T) {
			newOwner := identityset.Address(34)
			_, _, err := addOneTx(action.SignedCandidateTransferOwnership(12, newOwner.String(), nil, gasLimit, gasPrice, identityset.PrivateKey(12), action.WithChainID(chainID)))
			require.ErrorContains(err, "failed to find receipt")
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
	cfg.Genesis.TsunamiBlockHeight = 2
	cfg.Genesis.EndorsementWithdrawWaitingBlocks = 10
	cfg.Genesis.ToBeEnabledBlockHeight = 3 // enable CandidateIdentifiedByOwner feature

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

func TestCandidateTransferOwnership(t *testing.T) {
	require := require.New(t)
	initCfg := func() config.Config {
		cfg := deepcopy.Copy(config.Default).(config.Config)
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
		cfg.Genesis.InitBalanceMap[identityset.Address(1).String()] = "100000000000000000000000000"
		cfg.Genesis.InitBalanceMap[identityset.Address(2).String()] = "100000000000000000000000000"
		cfg.Genesis.EndorsementWithdrawWaitingBlocks = 10
		cfg.Genesis.PacificBlockHeight = 1
		cfg.Genesis.AleutianBlockHeight = 1
		cfg.Genesis.BeringBlockHeight = 1
		cfg.Genesis.CookBlockHeight = 1
		cfg.Genesis.DardanellesBlockHeight = 1
		cfg.Genesis.DaytonaBlockHeight = 1
		cfg.Genesis.EasterBlockHeight = 1
		cfg.Genesis.FbkMigrationBlockHeight = 1
		cfg.Genesis.FairbankBlockHeight = 1
		cfg.Genesis.GreenlandBlockHeight = 1
		cfg.Genesis.HawaiiBlockHeight = 1
		cfg.Genesis.IcelandBlockHeight = 1
		cfg.Genesis.JutlandBlockHeight = 1
		cfg.Genesis.KamchatkaBlockHeight = 1
		cfg.Genesis.LordHoweBlockHeight = 1
		cfg.Genesis.MidwayBlockHeight = 1
		cfg.Genesis.NewfoundlandBlockHeight = 1
		cfg.Genesis.OkhotskBlockHeight = 1
		cfg.Genesis.PalauBlockHeight = 1
		cfg.Genesis.QuebecBlockHeight = 1
		cfg.Genesis.RedseaBlockHeight = 1
		cfg.Genesis.SumatraBlockHeight = 1
		cfg.Genesis.TsunamiBlockHeight = 1
		cfg.Genesis.UpernavikBlockHeight = 1
		cfg.Genesis.ToBeEnabledBlockHeight = 1 // enable CandidateIdentifiedByOwner feature
		return cfg
	}
	registerAmount, _ := big.NewInt(0).SetString("1200000000000000000000000", 10)
	gasLimit = uint64(1000000)
	gasPrice = big.NewInt(10)
	successExpect := &basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_Success), ""}

	t.Run("transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg())
		defer test.teardown()
		oldOwnerID := 1
		newOwnerID := 2
		chainID := test.cfg.Chain.ID
		test.run([]*testcase{
			{
				name: "success to transfer candidate ownership",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(oldOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				},
				act:    &actionWithTime{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), identityset.Address(newOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1245621408203087110422302", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(newOwnerID).String(), SelfStakeBucketIdx: 0}}},
			},
			{
				name:   "cannot transfer to old owner",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(newOwnerID).String()), identityset.Address(oldOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(newOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrUnauthorizedOperator), ""}},
			},
		})
	})
	t.Run("candidate activate after transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg())
		defer test.teardown()

		oldOwnerID := 1
		newOwnerID := 2
		chainID := test.cfg.Chain.ID

		test.run([]*testcase{
			{
				name: "old owner cannot invoke candidate activate",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(oldOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), identityset.Address(newOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand1", registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				},
				act:    &actionWithTime{mustNoErr(action.SignedCandidateActivate(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), 1, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrCandidateNotExist), ""}},
			},
			{
				name: "new owner can invoke candidate activate",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(newOwnerID).String()), "cand1", registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(newOwnerID), action.WithChainID(chainID))), time.Now()},
				},
				act:    &actionWithTime{mustNoErr(action.SignedCandidateActivate(test.nonceMgr.pop(identityset.Address(newOwnerID).String()), 2, gasLimit, gasPrice, identityset.PrivateKey(newOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "3736864224609261331266906", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(newOwnerID).String(), SelfStakeBucketIdx: 2}}},
			},
		})
	})
	t.Run("candidate endorsement after transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg())
		defer test.teardown()

		oldOwnerID := 1
		newOwnerID := 2
		stakerID := 3
		chainID := test.cfg.Chain.ID
		stakeTime := time.Now()

		test.run([]*testcase{
			{
				name: "endorse same candidate after transfer ownership",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(oldOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand1", registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), stakeTime},
					{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), identityset.Address(newOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				},
				act:    &actionWithTime{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, true, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &bucketExpect{&iotextypes.VoteBucket{Index: 1, CandidateAddress: identityset.Address(oldOwnerID).String(), StakedAmount: registerAmount.String(), AutoStake: true, StakedDuration: 1, CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}, Owner: identityset.Address(stakerID).String(), ContractAddress: "", EndorsementExpireBlockHeight: math.MaxUint64}}},
			},
		})
	})
	t.Run("candidate register after transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg())
		defer test.teardown()

		oldOwnerID := 1
		newOwnerID := 2
		newOwnerID2 := 3
		chainID := test.cfg.Chain.ID

		test.run([]*testcase{
			{
				name: "old owner can register again after ownership transfer",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(oldOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), identityset.Address(newOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				},
				act:    &actionWithTime{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand2", identityset.Address(2).String(), identityset.Address(1).String(), identityset.Address(oldOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &candidateExpect{"cand2", &iotextypes.CandidateV2{Id: "", Name: "cand2", OperatorAddress: identityset.Address(2).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1245621408203087110422302", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(oldOwnerID).String(), SelfStakeBucketIdx: 1}}},
			},
			{
				name:   "new owner cannot register again",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(newOwnerID).String()), "cand2", identityset.Address(2).String(), identityset.Address(1).String(), identityset.Address(newOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(newOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrCandidateAlreadyExist), ""}},
			},
			{
				name:   "transfer ownership to another new owner",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(newOwnerID).String()), identityset.Address(newOwnerID2).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(newOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1245621408203087110422302", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(newOwnerID2).String(), SelfStakeBucketIdx: 0}}},
			},
			{
				name:   "new owner can register again",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(newOwnerID).String()), "cand3", identityset.Address(3).String(), identityset.Address(1).String(), identityset.Address(newOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(newOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &candidateExpect{"cand3", &iotextypes.CandidateV2{Name: "cand3", OperatorAddress: identityset.Address(3).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1245621408203087110422302", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(newOwnerID).String(), SelfStakeBucketIdx: 2}}},
			},
		})
	})
	t.Run("candidate update after transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg())
		defer test.teardown()

		oldOwnerID := 1
		newOwnerID := 2
		chainID := test.cfg.Chain.ID

		test.run([]*testcase{
			{
				name: "old owner cannot update",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(oldOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), identityset.Address(newOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				},
				act:    &actionWithTime{mustNoErr(action.SignedCandidateUpdate(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand2", identityset.Address(2).String(), identityset.Address(1).String(), gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrCandidateNotExist), ""}},
			},
			{
				name:   "new owner can update",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateUpdate(test.nonceMgr.pop(identityset.Address(newOwnerID).String()), "cand2", identityset.Address(2).String(), identityset.Address(1).String(), gasLimit, gasPrice, identityset.PrivateKey(newOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &candidateExpect{"cand2", &iotextypes.CandidateV2{Name: "cand2", OperatorAddress: identityset.Address(2).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1245621408203087110422302", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(newOwnerID).String(), SelfStakeBucketIdx: 0}}},
			},
		})
	})
	t.Run("stake change candidate after transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg())
		defer test.teardown()

		oldOwnerID := 1
		newOwnerID := 2
		stakerID := 3
		chainID := test.cfg.Chain.ID
		stakeTime := time.Now()

		test.run([]*testcase{
			{
				name: "change candidate after transfer ownership",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(oldOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), identityset.Address(newOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand2", identityset.Address(2).String(), identityset.Address(1).String(), identityset.Address(stakerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand2", registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), stakeTime},
				},
				act: &actionWithTime{mustNoErr(action.SignedChangeCandidate(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand1", 2, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					successExpect,
					&bucketExpect{&iotextypes.VoteBucket{Index: 2, CandidateAddress: identityset.Address(oldOwnerID).String(), StakedAmount: registerAmount.String(), AutoStake: true, StakedDuration: 1, CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}, Owner: identityset.Address(stakerID).String(), ContractAddress: "", EndorsementExpireBlockHeight: 0}},
					&candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "2491242816406174220844604", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(newOwnerID).String(), SelfStakeBucketIdx: 0}},
					&candidateExpect{"cand2", &iotextypes.CandidateV2{Name: "cand2", OperatorAddress: identityset.Address(2).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1245621408203087110422302", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(stakerID).String(), SelfStakeBucketIdx: 1}},
				},
			},
		})
	})
	t.Run("stake create after transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg())
		defer test.teardown()

		oldOwnerID := 1
		newOwnerID := 2
		stakerID := 3
		chainID := test.cfg.Chain.ID
		stakeTime := time.Now()

		test.run([]*testcase{
			{
				name: "create stake after transfer ownership",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(oldOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), identityset.Address(newOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				},
				act: &actionWithTime{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand1", registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), stakeTime},
				expect: []actionExpect{
					successExpect,
					&bucketExpect{&iotextypes.VoteBucket{Index: 1, CandidateAddress: identityset.Address(oldOwnerID).String(), StakedAmount: registerAmount.String(), AutoStake: true, StakedDuration: 1, CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}, Owner: identityset.Address(stakerID).String(), ContractAddress: "", EndorsementExpireBlockHeight: 0}},
				},
			},
		})
	})
}
