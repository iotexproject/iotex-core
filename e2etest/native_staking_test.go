package e2etest

import (
	"context"
	"encoding/hex"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/server/itx"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
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
	cand1NewVotes, _ = big.NewInt(0).SetString("1542516163985454635820816", 10)
	vote, _          = new(big.Int).SetString("100000000000000000000", 10)
	autoStakeVote, _ = new(big.Int).SetString("103801784016923925869", 10)
	initBalance, _   = new(big.Int).SetString("100000000000000000000000000", 10)
)

var (
	gasPrice = big.NewInt(0)
	gasLimit = uint64(10000000)
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
		_, esr, err := addOneTx(action.SignedCandidateEndorsementLegacy(4, endorseBucketIndex, true, gasLimit, gasPrice, voter1PriKey, action.WithChainID(chainID)))
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
		_, esr, err = addOneTx(action.SignedCandidateEndorsementLegacy(5, endorseBucketIndex, false, gasLimit, gasPrice, voter1PriKey, action.WithChainID(chainID)))
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
				BlockHeight: 2,
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
				BlockHeight: 2,
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
			require.NoError(checkCandidateState(sf, candidate1Name, newOwner1.String(), big.NewInt(0), cand1NewVotes, cand1Addr))
		})
		t.Run("candidate transfer ownership to a exist candidate", func(t *testing.T) {
			_, ccto, err := addOneTx(action.SignedCandidateTransferOwnership(7, cand2Addr.String(), nil, gasLimit, gasPrice, cand1PriKey, action.WithChainID(chainID)))
			require.NoError(err)
			require.EqualValues(iotextypes.ReceiptStatus_ErrCandidateAlreadyExist, ccto.Status)
			require.NoError(checkCandidateState(sf, candidate1Name, identityset.Address(33).String(), big.NewInt(0), cand1NewVotes, cand1Addr))
		})
		t.Run("candidate transfer ownership to a normal new address again", func(t *testing.T) {
			newOwner2 := identityset.Address(34)
			_, ccto, err := addOneTx(action.SignedCandidateTransferOwnership(8, newOwner2.String(), nil, gasLimit, gasPrice, cand1PriKey, action.WithChainID(chainID)))
			require.NoError(err)
			require.EqualValues(iotextypes.ReceiptStatus_Success, ccto.Status)
			require.NoError(checkCandidateState(sf, candidate1Name, newOwner2.String(), big.NewInt(0), cand1NewVotes, cand1Addr))
		})
		t.Run("candidate transfer ownership to a transfered candidate", func(t *testing.T) {
			newOwner2 := identityset.Address(34)
			_, ccto, err := addOneTx(action.SignedCandidateTransferOwnership(9, newOwner2.String(), nil, gasLimit, gasPrice, cand1PriKey, action.WithChainID(chainID)))
			require.NoError(err)
			require.EqualValues(iotextypes.ReceiptStatus_ErrCandidateAlreadyExist, ccto.Status)
			require.NoError(checkCandidateState(sf, candidate1Name, newOwner2.String(), big.NewInt(0), cand1NewVotes, cand1Addr))
		})
		t.Run("candidate transfer ownership to a contract address", func(t *testing.T) {
			data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
			_, se, err := addOneTx(action.SignedExecution(action.EmptyAddress, cand1PriKey, 10, big.NewInt(0), uint64(100000), big.NewInt(0), data))
			require.NoError(err)
			require.EqualValues(iotextypes.ReceiptStatus_Success, se.Status)
			_, ccto, err := addOneTx(action.SignedCandidateTransferOwnership(11, se.ContractAddress, nil, gasLimit, gasPrice, cand1PriKey, action.WithChainID(chainID)))
			require.NoError(err)
			require.EqualValues(iotextypes.ReceiptStatus_ErrUnknown, ccto.Status)
			require.NoError(checkCandidateState(sf, candidate1Name, identityset.Address(34).String(), big.NewInt(0), cand1NewVotes, cand1Addr))
		})
		t.Run("candidate transfer ownership to an invalid address", func(t *testing.T) {
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
	cfg.Genesis = genesis.TestDefault()
	initDBPaths(require, &cfg)
	defer func() {
		clearDBPaths(&cfg)
		// clear the gateway
		delete(cfg.Plugins, config.GatewayPlugin)
	}()

	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.BootstrapCandidates = testInitCands
	cfg.Genesis.FbkMigrationBlockHeight = 1
	cfg.Genesis.TsunamiBlockHeight = 2
	cfg.Genesis.EndorsementWithdrawWaitingBlocks = 10
	cfg.Genesis.UpernavikBlockHeight = 3 // enable CandidateIdentifiedByOwner feature

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
		return errors.Errorf("votes does not match, expected: %s, actual: %s", expectedVotes.String(), cand.Votes.String())
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

	registerAmount, _ := big.NewInt(0).SetString("1200000000000000000000000", 10)
	gasLimit := uint64(10000000)
	gasPrice := big.NewInt(1)

	t.Run("transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg(require))
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
				expect: []actionExpect{successExpect, &candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", Id: identityset.Address(oldOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1245621408203087110422302", SelfStakingTokens: "0", OwnerAddress: identityset.Address(newOwnerID).String(), SelfStakeBucketIdx: math.MaxUint64}}},
			},
			{
				name: "cannot transfer from non-owner",
				act: &actionWithTime{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(4).String()), identityset.Address(oldOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(4), action.WithChainID(chainID))),
					time.Now(),
				},
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrCandidateNotExist), ""}},
			},
			{
				name: "cannot transfer to self",
				act: &actionWithTime{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(newOwnerID).String()), identityset.Address(newOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(newOwnerID), action.WithChainID(chainID))),
					time.Now(),
				},
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrUnauthorizedOperator), ""}},
			},
			{
				name:   "transfer back to old owner",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(newOwnerID).String()), identityset.Address(oldOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(newOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", Id: identityset.Address(oldOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1245621408203087110422302", SelfStakingTokens: "0", OwnerAddress: identityset.Address(oldOwnerID).String(), SelfStakeBucketIdx: math.MaxUint64}}},
			},
		})
	})
	t.Run("transfer endorsed candidate", func(t *testing.T) {
		test := newE2ETest(t, initCfg(require))
		defer test.teardown()
		oldOwnerID := 1
		newOwnerID := 2
		stakerID := 3
		chainID := test.cfg.Chain.ID
		test.run([]*testcase{
			{
				name: "success to transfer candidate ownership",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(oldOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand1", registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, action.CandidateEndorsementOpEndorse, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCandidateActivate(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), 1, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				},
				act:    &actionWithTime{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), identityset.Address(newOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", Id: identityset.Address(oldOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "2491242816406174220844604", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(newOwnerID).String(), SelfStakeBucketIdx: 1}}},
			},
		})
	})
	t.Run("candidate activate after transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg(require))
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
				expect: []actionExpect{successExpect, &candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", Id: identityset.Address(oldOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "3736864224609261331266906", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(newOwnerID).String(), SelfStakeBucketIdx: 2}}},
			},
		})
	})
	t.Run("candidate endorsement after transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg(require))
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
				act:    &actionWithTime{mustNoErr(action.SignedCandidateEndorsementLegacy(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, true, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &bucketExpect{&iotextypes.VoteBucket{Index: 1, CandidateAddress: identityset.Address(oldOwnerID).String(), StakedAmount: registerAmount.String(), AutoStake: true, StakedDuration: 1, CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}, Owner: identityset.Address(stakerID).String(), ContractAddress: "", EndorsementExpireBlockHeight: math.MaxUint64}}},
			},
		})
	})
	t.Run("candidate register after transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg(require))
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
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrCandidateAlreadyExist), ""}},
			},
			{
				name:   "new owner cannot register again",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(newOwnerID).String()), "cand2", identityset.Address(2).String(), identityset.Address(1).String(), identityset.Address(newOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(newOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrCandidateAlreadyExist), ""}},
			},
			{
				name:   "transfer ownership to another new owner",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(newOwnerID).String()), identityset.Address(newOwnerID2).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(newOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", Id: identityset.Address(oldOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1245621408203087110422302", SelfStakingTokens: "0", OwnerAddress: identityset.Address(newOwnerID2).String(), SelfStakeBucketIdx: math.MaxUint64}}},
			},
			{
				name:   "new owner can register again",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(newOwnerID).String()), "cand3", identityset.Address(3).String(), identityset.Address(1).String(), identityset.Address(newOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(newOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &candidateExpect{"cand3", &iotextypes.CandidateV2{Name: "cand3", Id: identityset.Address(newOwnerID).String(), OperatorAddress: identityset.Address(3).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1245621408203087110422302", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(newOwnerID).String(), SelfStakeBucketIdx: 1}}},
			},
		})
	})
	t.Run("candidate update after transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg(require))
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
				expect: []actionExpect{successExpect, &candidateExpect{"cand2", &iotextypes.CandidateV2{Name: "cand2", Id: identityset.Address(oldOwnerID).String(), OperatorAddress: identityset.Address(2).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1245621408203087110422302", SelfStakingTokens: "0", OwnerAddress: identityset.Address(newOwnerID).String(), SelfStakeBucketIdx: math.MaxUint64}}},
			},
		})
	})
	t.Run("stake change candidate after transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg(require))
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
					&candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", Id: identityset.Address(oldOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "2491242816406174220844604", SelfStakingTokens: "0", OwnerAddress: identityset.Address(newOwnerID).String(), SelfStakeBucketIdx: math.MaxUint64}},
					&candidateExpect{"cand2", &iotextypes.CandidateV2{Name: "cand2", Id: identityset.Address(stakerID).String(), OperatorAddress: identityset.Address(2).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1245621408203087110422302", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(stakerID).String(), SelfStakeBucketIdx: 1}},
				},
			},
		})
	})
	t.Run("stake create after transfer candidate ownership", func(t *testing.T) {
		test := newE2ETest(t, initCfg(require))
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
	t.Run("migrate stake", func(t *testing.T) {
		contractAddress := "io1dkqh5mu9djfas3xyrmzdv9frsmmytel4mp7a64"
		cfg := initCfg(require)
		cfg.Genesis.SystemStakingContractV2Address = contractAddress
		cfg.Genesis.SystemStakingContractV2Height = 1
		cfg.Genesis.VanuatuBlockHeight = 100
		testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
		cfg.DardanellesUpgrade.BlockInterval = time.Second * 8640
		cfg.Plugins[config.GatewayPlugin] = nil
		test := newE2ETest(t, cfg)
		defer test.teardown()

		chainID := test.cfg.Chain.ID
		stakerID := 1
		stakeAmount, _ := big.NewInt(0).SetString("10000000000000000000000", 10)
		stakeDurationDays := uint32(1) // 1day
		stakeTime := time.Now()
		candOwnerID := 2
		blocksPerDay := 24 * time.Hour / cfg.DardanellesUpgrade.BlockInterval
		balance := big.NewInt(0)
		h := identityset.Address(1).String()
		t.Logf("address 1: %v\n", h)
		minAmount, _ := big.NewInt(0).SetString("1000000000000000000000", 10) // 1000 IOTX
		bytecode, err := hex.DecodeString(stakingContractV2Bytecode)
		require.NoError(err)
		mustCallData := func(m string, args ...any) []byte {
			data, err := abiCall(staking.StakingContractABI, m, args...)
			require.NoError(err)
			return data
		}
		deployCode := append(bytecode, mustCallData("", minAmount)...)
		poorID := 30
		test.run([]*testcase{
			{
				name: "deploy staking contract",
				act:  &actionWithTime{mustNoErr(action.SignedExecution("", identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, deployCode, action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					successExpect, &executionExpect{contractAddress},
				},
			},
			{
				name: "non-owner cannot migrate stake",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("setBeneficiary(address)", common.BytesToAddress(identityset.Address(stakerID).Bytes())), action.WithChainID(chainID))), stakeTime},
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(candOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(candOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand1", stakeAmount.String(), stakeDurationDays, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), stakeTime},
				},
				act: &actionWithTime{mustNoErr(action.SignedMigrateStake(test.nonceMgr.pop(identityset.Address(2).String()), 1, gasLimit, gasPrice, identityset.PrivateKey(2), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrUnauthorizedOperator), ""},
					&bucketExpect{&iotextypes.VoteBucket{Index: 1, CandidateAddress: identityset.Address(candOwnerID).String(), StakedAmount: stakeAmount.String(), AutoStake: true, StakedDuration: stakeDurationDays, Owner: identityset.Address(stakerID).String(), CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}}},
					&candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", Id: identityset.Address(candOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1256001586604779503009155", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(candOwnerID).String(), SelfStakeBucketIdx: 0}},
				},
			},
			{
				name: "success to migrate stake",
				preFunc: func(e *e2etest) {
					// get balance before migration
					resp, err := e.api.GetAccount(context.Background(), &iotexapi.GetAccountRequest{
						Address: identityset.Address(stakerID).String(),
					})
					require.NoError(err)
					b, ok := big.NewInt(0).SetString(resp.GetAccountMeta().GetBalance(), 10)
					require.True(ok)
					balance = b
				},
				act: &actionWithTime{mustNoErr(action.SignedMigrateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					successExpect,
					&fullActionExpect{
						address.StakingProtocolAddr, 212012,
						[]*action.TransactionLog{
							{
								Type:      iotextypes.TransactionLogType_GAS_FEE,
								Amount:    big.NewInt(int64(action.MigrateStakeBaseIntrinsicGas)),
								Sender:    identityset.Address(stakerID).String(),
								Recipient: address.RewardingPoolAddr,
							},
							{
								Type:      iotextypes.TransactionLogType_WITHDRAW_BUCKET,
								Amount:    stakeAmount,
								Sender:    address.StakingBucketPoolAddr,
								Recipient: identityset.Address(stakerID).String(),
							},
							{
								Type:      iotextypes.TransactionLogType_GAS_FEE,
								Amount:    big.NewInt(202012),
								Sender:    identityset.Address(stakerID).String(),
								Recipient: address.RewardingPoolAddr,
							},
							{
								Type:      iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER,
								Amount:    stakeAmount,
								Sender:    identityset.Address(stakerID).String(),
								Recipient: contractAddress,
							},
						},
					},
					&bucketExpect{&iotextypes.VoteBucket{Index: 1, CandidateAddress: identityset.Address(candOwnerID).String(), StakedAmount: stakeAmount.String(), AutoStake: true, StakedDuration: stakeDurationDays, StakedDurationBlockNumber: uint64(stakeDurationDays) * uint64(blocksPerDay), CreateBlockHeight: 6, StakeStartBlockHeight: 6, UnstakeStartBlockHeight: math.MaxUint64, Owner: identityset.Address(stakerID).String(), ContractAddress: contractAddress, CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), UnstakeStartTime: timestamppb.New(time.Time{})}},
					&noBucketExpect{1, ""},
					&candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", Id: identityset.Address(candOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "1256001586604779503009155", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(candOwnerID).String(), SelfStakeBucketIdx: 0}},
					&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
						resp, err := test.api.GetAccount(context.Background(), &iotexapi.GetAccountRequest{
							Address: identityset.Address(stakerID).String(),
						})
						require.NoError(err)
						postBalance, ok := big.NewInt(0).SetString(resp.GetAccountMeta().GetBalance(), 10)
						require.True(ok)
						gasInLog := big.NewInt(0)
						for _, l := range receipt.TransactionLogs() {
							if l.Type == iotextypes.TransactionLogType_GAS_FEE {
								gasInLog.Add(gasInLog, l.Amount)
							}
						}
						gasFee := big.NewInt(0).Mul(big.NewInt(int64(receipt.GasConsumed)), gasPrice)
						// sum of gas in logs = gas consumed of receipt
						require.Equal(gasInLog, gasFee)
						// balance = preBalance - gasFee
						require.Equal(balance.Sub(balance, gasFee).String(), postBalance.String())
					}},
				},
			},
			{
				name: "stake",
				act:  &actionWithTime{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand1", unit.ConvertIotxToRau(100).String(), stakeDurationDays, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), stakeTime},
				expect: []actionExpect{
					successExpect,
					&bucketExpect{&iotextypes.VoteBucket{Index: 2, CandidateAddress: identityset.Address(candOwnerID).String(), StakedAmount: unit.ConvertIotxToRau(100).String(), AutoStake: true, StakedDuration: stakeDurationDays, Owner: identityset.Address(stakerID).String(), CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}}},
				},
			},
			{
				name:    "contract call failure",
				preActs: []*actionWithTime{},
				act:     &actionWithTime{mustNoErr(action.SignedMigrateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), 2, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrExecutionReverted), ""},
					&fullActionExpect{
						address.StakingProtocolAddr, 29447, []*action.TransactionLog{
							{
								Type:      iotextypes.TransactionLogType_GAS_FEE,
								Amount:    big.NewInt(29447),
								Sender:    identityset.Address(stakerID).String(),
								Recipient: address.RewardingPoolAddr,
							},
						},
					},
					&bucketExpect{&iotextypes.VoteBucket{Index: 2, CandidateAddress: identityset.Address(candOwnerID).String(), StakedAmount: unit.ConvertIotxToRau(100).String(), AutoStake: true, StakedDuration: stakeDurationDays, Owner: identityset.Address(stakerID).String(), CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}}},
				},
			},
			{
				name: "self-stake bucket cannot be migrated",
				act:  &actionWithTime{mustNoErr(action.SignedMigrateStake(test.nonceMgr.pop(identityset.Address(candOwnerID).String()), 0, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), ""},
				},
			},
			{
				name: "unstaked bucket cannot be migrated",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand1", unit.ConvertIotxToRau(100).String(), 0, false, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), stakeTime},
					{mustNoErr(action.SignedReclaimStake(false, test.nonceMgr.pop(identityset.Address(stakerID).String()), 3, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				},
				act: &actionWithTime{mustNoErr(action.SignedMigrateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), 3, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), ""},
				},
			},
			{
				name: "non auto-stake bucket cannot be migrated",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand1", unit.ConvertIotxToRau(100).String(), stakeDurationDays, false, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), stakeTime},
				},
				act: &actionWithTime{mustNoErr(action.SignedMigrateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), 4, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), ""},
				},
			},
			{
				name: "endorsement bucket cannot be migrated",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand1", registerAmount.String(), 91, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), stakeTime},
					{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(stakerID).String()), 5, action.CandidateEndorsementOpEndorse, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				},
				act: &actionWithTime{mustNoErr(action.SignedMigrateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), 5, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), ""},
				},
			},
			{
				name: "estimateGas",
				act:  &actionWithTime{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand1", stakeAmount.String(), stakeDurationDays, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), stakeTime},
				expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					ms := action.NewMigrateStake(6)
					resp, err := test.api.EstimateActionGasConsumption(context.Background(), &iotexapi.EstimateActionGasConsumptionRequest{
						Action:        &iotexapi.EstimateActionGasConsumptionRequest_StakeMigrate{StakeMigrate: ms.Proto()},
						CallerAddress: identityset.Address(3).String(),
						GasPrice:      gasPrice.String(),
					})
					require.NoError(err)
					require.Equal(uint64(194912), resp.Gas)
					require.Len(receipt.Logs(), 1)
					topic := receipt.Logs()[0].Topics[1][:]
					bktIdx := byteutil.BytesToUint64BigEndian(topic[len(topic)-8:])
					require.Equal(uint64(6), bktIdx)
				}}},
			},
			{
				name: "estimateGasPoorAcc",
				act:  &actionWithTime{mustNoErr(action.SignedTransferStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), identityset.Address(poorID).String(), 6, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					resp1, err := test.api.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: identityset.Address(poorID).String()})
					require.NoError(err)
					require.Equal("0", resp1.GetAccountMeta().Balance)
					ms := action.NewMigrateStake(6)
					resp, err := test.api.EstimateActionGasConsumption(context.Background(), &iotexapi.EstimateActionGasConsumptionRequest{
						Action:        &iotexapi.EstimateActionGasConsumptionRequest_StakeMigrate{StakeMigrate: ms.Proto()},
						CallerAddress: identityset.Address(poorID).String(),
						GasPrice:      gasPrice.String(),
					})
					require.NoError(err)
					require.Equal(uint64(194912), resp.Gas)
				}}},
			},
		})
	})
	t.Run("new endorsement", func(t *testing.T) {
		cfg := initCfg(require)
		cfg.Genesis.UpernavikBlockHeight = 6
		cfg.Genesis.EndorsementWithdrawWaitingBlocks = 5
		test := newE2ETest(t, cfg)
		defer test.teardown()

		var (
			candOwnerID  = 1
			stakerID     = 2
			candOwnerID2 = 3
			chainID      = test.cfg.Chain.ID
			stakeTime    = time.Now()
		)
		test.run([]*testcase{
			{
				name: "endorse action disabled before UpernavikBlockHeight",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(candOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(candOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand1", registerAmount.String(), 91, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), stakeTime},
				},
				act:    &actionWithTime{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr[(identityset.Address(stakerID).String())], 1, action.CandidateEndorsementOpEndorse, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{errReceiptNotFound, 0, ""}},
			},
			{
				name:   "intentToRevokeEndorsement action disabled before UpernavikBlockHeight",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr[(identityset.Address(stakerID).String())], 1, action.CandidateEndorsementOpIntentToRevoke, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{errReceiptNotFound, 0, ""}},
			},
			{
				name:   "revokeEndorsement action disabled before UpernavikBlockHeight",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr[(identityset.Address(stakerID).String())], 1, action.CandidateEndorsementOpRevoke, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{errReceiptNotFound, 0, ""}},
			},
			{
				name:   "endorse action disabled after UpernavikBlockHeight",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, action.CandidateEndorsementOpEndorse, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect},
			},
			{
				name: "cannot change candidate after bucket is endorsed",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(candOwnerID2).String()), "cand2", identityset.Address(3).String(), identityset.Address(3).String(), identityset.Address(candOwnerID2).String(), "0", 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID2), action.WithChainID(chainID))), time.Now()},
				},
				act:    &actionWithTime{mustNoErr(action.SignedChangeCandidate(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand2", 1, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), ""}},
			},
			{
				name:   "cannot migrate after bucket is endorsed",
				act:    &actionWithTime{mustNoErr(action.SignedMigrateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), ""}},
			},
			{
				name:   "cannot unstake after bucket is endorsed",
				act:    &actionWithTime{mustNoErr(action.SignedReclaimStake(false, test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), ""}},
			},
			{
				name:   "cannot revoke before endorsement expire",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, action.CandidateEndorsementOpRevoke, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), ""}},
			},
			{
				name: "intentToRevoke now if endorsement is not used",
				act:  &actionWithTime{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, action.CandidateEndorsementOpIntentToRevoke, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					successExpect,
					&bucketExpect{&iotextypes.VoteBucket{Index: 1, EndorsementExpireBlockHeight: 12, CandidateAddress: identityset.Address(candOwnerID).String(), StakedAmount: registerAmount.String(), AutoStake: true, StakedDuration: 91, Owner: identityset.Address(stakerID).String(), CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}}},
				},
			},
			{
				name: "revoke success",
				act:  &actionWithTime{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, action.CandidateEndorsementOpRevoke, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					successExpect,
					&bucketExpect{&iotextypes.VoteBucket{Index: 1, EndorsementExpireBlockHeight: 0, CandidateAddress: identityset.Address(candOwnerID).String(), StakedAmount: registerAmount.String(), AutoStake: true, StakedDuration: 91, Owner: identityset.Address(stakerID).String(), CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}}},
				},
			},
			{
				name: "intentToRevoke if endorsement is used",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedChangeCandidate(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand2", 1, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, action.CandidateEndorsementOpEndorse, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCandidateActivate(test.nonceMgr.pop(identityset.Address(candOwnerID2).String()), 1, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID2), action.WithChainID(chainID))), time.Now()},
				},
				act: &actionWithTime{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, action.CandidateEndorsementOpIntentToRevoke, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					successExpect,
					&bucketExpect{&iotextypes.VoteBucket{Index: 1, EndorsementExpireBlockHeight: 22, CandidateAddress: identityset.Address(candOwnerID2).String(), StakedAmount: registerAmount.String(), AutoStake: true, StakedDuration: 91, Owner: identityset.Address(stakerID).String(), CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}}},
					&candidateExpect{"cand2", &iotextypes.CandidateV2{Name: "cand2", Id: identityset.Address(candOwnerID2).String(), OperatorAddress: identityset.Address(3).String(), RewardAddress: identityset.Address(3).String(), TotalWeightedVotes: "1635067133824581908640994", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(candOwnerID2).String(), SelfStakeBucketIdx: 1}},
				},
			},
			{
				name:   "cannot revoke before expired",
				act:    &actionWithTime{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, action.CandidateEndorsementOpRevoke, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrInvalidBucketType), ""}},
			},
			{
				name: "eligible as proposer after expired",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
				},
				act: &actionWithTime{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect,
					&candidateExpect{"cand2", &iotextypes.CandidateV2{Name: "cand2", Id: identityset.Address(candOwnerID2).String(), OperatorAddress: identityset.Address(3).String(), RewardAddress: identityset.Address(3).String(), TotalWeightedVotes: "1635067133824581908640994", SelfStakingTokens: registerAmount.String(), OwnerAddress: identityset.Address(candOwnerID2).String(), SelfStakeBucketIdx: 1}},
				},
			},
			{
				name: "revoke success after expired",
				act:  &actionWithTime{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, action.CandidateEndorsementOpRevoke, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{
					successExpect,
					&bucketExpect{&iotextypes.VoteBucket{Index: 1, EndorsementExpireBlockHeight: 0, CandidateAddress: identityset.Address(candOwnerID2).String(), StakedAmount: registerAmount.String(), AutoStake: true, StakedDuration: 91, Owner: identityset.Address(stakerID).String(), CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}}},
					&candidateExpect{"cand2", &iotextypes.CandidateV2{Name: "cand2", Id: identityset.Address(candOwnerID2).String(), OperatorAddress: identityset.Address(3).String(), RewardAddress: identityset.Address(3).String(), TotalWeightedVotes: "1542516163985454635820816", SelfStakingTokens: "0", OwnerAddress: identityset.Address(candOwnerID2).String(), SelfStakeBucketIdx: math.MaxUint64}},
				},
			},
			{
				name: "revoke if endorse bucket is self-owned",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedTransferStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), identityset.Address(candOwnerID2).String(), 1, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(candOwnerID2).String()), 1, action.CandidateEndorsementOpEndorse, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID2), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCandidateActivate(test.nonceMgr.pop(identityset.Address(candOwnerID2).String()), 1, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID2), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(candOwnerID2).String()), 1, action.CandidateEndorsementOpIntentToRevoke, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID2), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
				},
				act: &actionWithTime{mustNoErr(action.SignedCandidateEndorsement(test.nonceMgr.pop(identityset.Address(candOwnerID2).String()), 1, action.CandidateEndorsementOpRevoke, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID2), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect,
					&candidateExpect{"cand2", &iotextypes.CandidateV2{Name: "cand2", Id: identityset.Address(candOwnerID2).String(), OperatorAddress: identityset.Address(3).String(), RewardAddress: identityset.Address(3).String(), TotalWeightedVotes: "1542516163985454635820816", SelfStakingTokens: "0", OwnerAddress: identityset.Address(candOwnerID2).String(), SelfStakeBucketIdx: math.MaxUint64}},
				},
			},
		})
	})
	t.Run("revise_endorsement", func(t *testing.T) {
		cfg := initCfg(require)
		cfg.Genesis.UpernavikBlockHeight = 25
		cfg.Genesis.EndorsementWithdrawWaitingBlocks = 5
		test := newE2ETest(t, cfg)
		defer test.teardown()

		var (
			candOwnerID  = 1
			stakerID     = 2
			candOwnerID2 = 3
			candOwnerID3 = 4
			chainID      = test.cfg.Chain.ID
			stakeTime    = time.Now()
		)
		test.run([]*testcase{
			{
				name: "deposit after endorsement expired",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(candOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(candOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID), action.WithChainID(test.cfg.Chain.ID))), time.Now()},
					{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand1", registerAmount.String(), 91, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(test.cfg.Chain.ID))), stakeTime},
					{mustNoErr(action.SignedCandidateEndorsementLegacy(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, true, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(test.cfg.Chain.ID))), time.Now()},
					{mustNoErr(action.SignedCandidateActivate(test.nonceMgr.pop(identityset.Address(candOwnerID).String()), 1, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID), action.WithChainID(test.cfg.Chain.ID))), time.Now()},
					{mustNoErr(action.SignedCandidateEndorsementLegacy(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, false, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(test.cfg.Chain.ID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
				},
				act: &actionWithTime{mustNoErr(action.SignedDepositToStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, unit.ConvertIotxToRau(10000000).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect,
					&candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", Id: identityset.Address(candOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "15734989908573124317570090", SelfStakingTokens: "0", OwnerAddress: identityset.Address(candOwnerID).String(), SelfStakeBucketIdx: math.MaxUint64}},
					&bucketExpect{&iotextypes.VoteBucket{Index: 1, EndorsementExpireBlockHeight: 10, CandidateAddress: identityset.Address(candOwnerID).String(), StakedAmount: "11200000000000000000000000", AutoStake: true, StakedDuration: 91, Owner: identityset.Address(stakerID).String(), CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}}},
				},
			},
			{
				name: "wrong votes after change delegate",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(candOwnerID2).String()), "cand2", identityset.Address(3).String(), identityset.Address(3).String(), identityset.Address(candOwnerID2).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID2), action.WithChainID(test.cfg.Chain.ID))), time.Now()},
					{mustNoErr(action.SignedChangeCandidate(test.nonceMgr.pop(identityset.Address(candOwnerID).String()), "cand2", 0, nil, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID), action.WithChainID(chainID))), time.Now()},
				},
				act: &actionWithTime{mustNoErr(action.SignedChangeCandidate(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand2", 1, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect,
					&candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", Id: identityset.Address(candOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "92550969839127272820178", SelfStakingTokens: "0", OwnerAddress: identityset.Address(candOwnerID).String(), SelfStakeBucketIdx: math.MaxUint64}},
				},
			},
			{
				name: "endorsement expired",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(candOwnerID3).String()), "cand3", identityset.Address(4).String(), identityset.Address(4).String(), identityset.Address(candOwnerID3).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID3), action.WithChainID(test.cfg.Chain.ID))), time.Now()},
					{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand3", registerAmount.String(), 91, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(test.cfg.Chain.ID))), stakeTime},
					{mustNoErr(action.SignedCandidateEndorsementLegacy(test.nonceMgr.pop(identityset.Address(stakerID).String()), 4, true, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(test.cfg.Chain.ID))), time.Now()},
					{mustNoErr(action.SignedCandidateActivate(test.nonceMgr.pop(identityset.Address(candOwnerID3).String()), 4, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID3), action.WithChainID(test.cfg.Chain.ID))), time.Now()},
					{mustNoErr(action.SignedCandidateEndorsementLegacy(test.nonceMgr.pop(identityset.Address(stakerID).String()), 4, false, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(test.cfg.Chain.ID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
				},
				act: &actionWithTime{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect,
					&candidateExpect{"cand3", &iotextypes.CandidateV2{Name: "cand3", Id: identityset.Address(candOwnerID3).String(), OperatorAddress: identityset.Address(4).String(), RewardAddress: identityset.Address(4).String(), TotalWeightedVotes: "2880688542027669019063296", SelfStakingTokens: "0", OwnerAddress: identityset.Address(candOwnerID3).String(), SelfStakeBucketIdx: math.MaxUint64}},
					&bucketExpect{&iotextypes.VoteBucket{Index: 4, EndorsementExpireBlockHeight: 24, CandidateAddress: identityset.Address(candOwnerID3).String(), StakedAmount: "1200000000000000000000000", AutoStake: true, StakedDuration: 91, Owner: identityset.Address(stakerID).String(), CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}}},
				},
			},
			{
				name: "revise at upernavik block height",
				act:  &actionWithTime{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect,
					&bucketExpect{&iotextypes.VoteBucket{Index: 1, EndorsementExpireBlockHeight: 10, CandidateAddress: identityset.Address(candOwnerID2).String(), StakedAmount: "11200000000000000000000000", AutoStake: true, StakedDuration: 91, Owner: identityset.Address(stakerID).String(), CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}}},
					&bucketExpect{&iotextypes.VoteBucket{Index: 4, EndorsementExpireBlockHeight: 0, CandidateAddress: identityset.Address(candOwnerID3).String(), StakedAmount: "1200000000000000000000000", AutoStake: true, StakedDuration: 91, Owner: identityset.Address(stakerID).String(), CreateTime: timestamppb.New(stakeTime), StakeStartTime: timestamppb.New(stakeTime), UnstakeStartTime: &timestamppb.Timestamp{}}},
					&candidateExpect{"cand1", &iotextypes.CandidateV2{Name: "cand1", Id: identityset.Address(candOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), TotalWeightedVotes: "0", SelfStakingTokens: "0", OwnerAddress: identityset.Address(candOwnerID).String(), SelfStakeBucketIdx: math.MaxUint64}},
				},
			},
		})
	})
	t.Run("votesCounting", func(t *testing.T) {
		contractAddr := "io16gnlvx6zk3tev9g6vaupngkpcrwe8hdsknxerw"
		cfg := initCfg(require)
		cfg.Genesis.UpernavikBlockHeight = 1
		cfg.Genesis.VanuatuBlockHeight = 100
		testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
		cfg.Genesis.EndorsementWithdrawWaitingBlocks = 5
		cfg.DardanellesUpgrade.BlockInterval = time.Second * 8640
		cfg.Genesis.SystemStakingContractV2Address = contractAddr
		cfg.Genesis.SystemStakingContractV2Height = 0
		test := newE2ETest(t, cfg)
		defer test.teardown()

		var (
			oldOwnerID          = 1
			stakerID            = 2
			contractCreator     = 3
			newOwnerID          = 4
			beneficiaryID       = 5
			chainID             = test.cfg.Chain.ID
			stakeTime           = time.Now()
			minAmount           = unit.ConvertIotxToRau(1000)
			stakeAmount         = unit.ConvertIotxToRau(10000)
			blocksPerDay        = 24 * time.Hour / cfg.DardanellesUpgrade.BlockInterval
			stakeDurationBlocks = big.NewInt(int64(blocksPerDay))
			candidate           *iotextypes.CandidateV2
		)
		bytecode, err := hex.DecodeString(stakingContractV2Bytecode)
		require.NoError(err)
		mustCallData := func(m string, args ...any) []byte {
			data, err := abiCall(staking.StakingContractABI, m, args...)
			require.NoError(err)
			return data
		}
		test.run([]*testcase{
			{
				name: "prepare",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(oldOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				},
				act:    &actionWithTime{mustNoErr(action.SignedExecution("", identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, append(bytecode, mustCallData("", minAmount)...), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &executionExpect{contractAddr}},
			},
			{
				name: "stakeBuckets",
				preActs: []*actionWithTime{
					{mustNoErr(action.SignedExecution(contractAddr, identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("setBeneficiary(address)", common.BytesToAddress(identityset.Address(beneficiaryID).Bytes())), action.WithChainID(chainID))), stakeTime},
					{mustNoErr(action.SignedCreateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), "cand1", stakeAmount.String(), 91, true, nil, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(test.cfg.Chain.ID))), stakeTime},
				},
				act: &actionWithTime{mustNoErr(action.SignedExecution(contractAddr, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), stakeAmount, gasLimit, gasPrice, mustCallData("stake(uint256,address)", stakeDurationBlocks, common.BytesToAddress(identityset.Address(oldOwnerID).Bytes())), action.WithChainID(chainID))), stakeTime},
				expect: []actionExpect{successExpect, &functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					cand, err := test.getCandidateByName("cand1")
					require.NoError(err)
					candidate = cand
					selfStake, err := test.getBucket(cand.SelfStakeBucketIdx, "")
					require.NoError(err)
					require.NotNil(selfStake)
					nb, err := test.getBucket(1, "")
					require.NoError(err)
					require.NotNil(nb)
					nft, err := test.getBucket(1, contractAddr)
					require.NoError(err)
					require.NotNil(nft)
					amtSS, ok := big.NewInt(0).SetString(selfStake.StakedAmount, 10)
					require.True(ok)
					amtNB, ok := big.NewInt(0).SetString(nb.StakedAmount, 10)
					require.True(ok)
					amtNFT, ok := big.NewInt(0).SetString(nft.StakedAmount, 10)
					require.True(ok)
					voteSS := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{StakedAmount: amtSS, StakedDuration: time.Duration(selfStake.StakedDuration*24) * time.Hour, AutoStake: selfStake.AutoStake}, true)
					voteNB := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{StakedAmount: amtNB, StakedDuration: time.Duration(nb.StakedDuration*24) * time.Hour, AutoStake: nb.AutoStake}, false)
					voteNFT := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{StakedAmount: amtNFT, StakedDuration: time.Duration(nft.StakedDuration*24) * time.Hour, AutoStake: nft.AutoStake}, false)
					require.Equal(voteSS.Add(voteSS, voteNB.Add(voteNB, voteNFT)).String(), cand.TotalWeightedVotes)
				}}},
			},
			{
				name: "transferOwnership",
				act:  &actionWithTime{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), identityset.Address(newOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
				expect: []actionExpect{successExpect, &functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					cand, err := test.getCandidateByName(candidate.Name)
					require.NoError(err)
					selfStakeBucket, err := test.getBucket(candidate.SelfStakeBucketIdx, "")
					require.NoError(err)
					require.NotNil(selfStakeBucket)
					amtSS, ok := big.NewInt(0).SetString(selfStakeBucket.StakedAmount, 10)
					require.True(ok)
					selfStakeVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{StakedAmount: amtSS, StakedDuration: time.Duration(selfStakeBucket.StakedDuration*24) * time.Hour, AutoStake: selfStakeBucket.AutoStake}, true)
					nonSelfStakeVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{StakedAmount: amtSS, StakedDuration: time.Duration(selfStakeBucket.StakedDuration*24) * time.Hour, AutoStake: selfStakeBucket.AutoStake}, false)
					deltaVotes := big.NewInt(0).Sub(selfStakeVotes, nonSelfStakeVotes)
					votes, ok := big.NewInt(0).SetString(cand.TotalWeightedVotes, 10)
					require.True(ok)
					require.Equal(votes.Sub(votes, deltaVotes).String(), candidate.TotalWeightedVotes)
				}}},
			},
		})
	})
}

func initCfg(r *require.Assertions) config.Config {
	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg = deepcopy.Copy(cfg).(config.Config)
	initDBPaths(r, &cfg)

	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.InitBalanceMap[identityset.Address(1).String()] = "100000000000000000000000000"
	cfg.Genesis.InitBalanceMap[identityset.Address(2).String()] = "100000000000000000000000000"
	cfg.Genesis.EndorsementWithdrawWaitingBlocks = 10
	cfg.Genesis.TsunamiBlockHeight = 1
	cfg.Genesis.UpernavikBlockHeight = 2 // enable CandidateIdentifiedByOwner feature
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	return cfg
}

func TestCandidateOwnerCollision(t *testing.T) {
	require := require.New(t)
	initCfg := func() config.Config {
		cfg := config.Default
		cfg.Genesis = genesis.TestDefault()
		cfg = deepcopy.Copy(cfg).(config.Config)
		initDBPaths(require, &cfg)

		cfg.ActPool.MinGasPriceStr = "0"
		cfg.Chain.TrieDBPatchFile = ""
		cfg.Consensus.Scheme = config.NOOPScheme
		cfg.Chain.EnableAsyncIndexWrite = false
		cfg.Genesis.InitBalanceMap[identityset.Address(1).String()] = "100000000000000000000000000"
		cfg.Genesis.InitBalanceMap[identityset.Address(2).String()] = "100000000000000000000000000"
		cfg.Genesis.EndorsementWithdrawWaitingBlocks = 10
		cfg.Genesis.TsunamiBlockHeight = 1
		cfg.Genesis.UpernavikBlockHeight = 2 // enable CandidateIdentifiedByOwner feature
		testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
		return cfg
	}
	registerAmount, _ := big.NewInt(0).SetString("1200000000000000000000000", 10)
	gasLimit = uint64(10000000)
	gasPrice = big.NewInt(1)
	oldOwnerID := 1
	newOwnerID := 2
	newOwnerID2 := 3
	test := newE2ETest(t, initCfg())
	defer test.teardown()
	chainID := test.cfg.Chain.ID
	test.run([]*testcase{
		{
			name: "owner cannot register again",
			preActs: []*actionWithTime{
				{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(oldOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
			},
			act:    &actionWithTime{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand2", identityset.Address(2).String(), identityset.Address(2).String(), identityset.Address(oldOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrCandidateAlreadyExist), ""}},
		},
		{
			name:    "original owner cannot register again",
			preActs: []*actionWithTime{{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), identityset.Address(newOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()}},
			act:     &actionWithTime{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(oldOwnerID).String()), "cand2", identityset.Address(2).String(), identityset.Address(2).String(), identityset.Address(oldOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(oldOwnerID), action.WithChainID(chainID))), time.Now()},
			expect:  []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrCandidateAlreadyExist), ""}},
		},
		{
			name:    "cannot transfer to a new owner that same as an existed candidate identifier",
			preActs: []*actionWithTime{{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(newOwnerID2).String()), "cand2", identityset.Address(3).String(), identityset.Address(3).String(), identityset.Address(newOwnerID2).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(newOwnerID2), action.WithChainID(chainID))), time.Now()}},
			act:     &actionWithTime{mustNoErr(action.SignedCandidateTransferOwnership(test.nonceMgr.pop(identityset.Address(newOwnerID2).String()), identityset.Address(oldOwnerID).String(), nil, gasLimit, gasPrice, identityset.PrivateKey(newOwnerID2), action.WithChainID(chainID))), time.Now()},
			expect:  []actionExpect{&basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_ErrCandidateAlreadyExist), ""}},
		},
	})
}
