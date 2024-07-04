package e2etest

import (
	"context"
	_ "embed"
	"encoding/hex"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
)

var (
	//go:embed staking_contract_v2_bytecode
	stakingContractV2Bytecode string
	stakingContractV2ABI      = staking.StakingContractABI
	stakingContractV2Address  = "io1dkqh5mu9djfas3xyrmzdv9frsmmytel4mp7a64"
)

func TestContractStakingV2(t *testing.T) {
	require := require.New(t)
	contractAddress := stakingContractV2Address
	cfg := initCfg(require)
	cfg.Genesis.UpernavikBlockHeight = 1
	cfg.Genesis.SystemStakingContractV2Address = contractAddress
	cfg.Genesis.SystemStakingContractV2Height = 1
	cfg.DardanellesUpgrade.BlockInterval = time.Second * 8640
	cfg.Plugins[config.GatewayPlugin] = nil
	test := newE2ETest(t, cfg)
	defer test.teardown()

	var (
		successExpect       = &basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_Success), ""}
		chainID             = test.cfg.Chain.ID
		contractCreator     = 1
		stakerID            = 2
		beneficiaryID       = 10
		stakeAmount         = unit.ConvertIotxToRau(10000)
		registerAmount      = unit.ConvertIotxToRau(1200000)
		stakeTime           = time.Now()
		unlockTime          = stakeTime.Add(time.Hour)
		candOwnerID         = 3
		candOwnerID2        = 4
		blocksPerDay        = 24 * time.Hour / cfg.DardanellesUpgrade.BlockInterval
		stakeDurationBlocks = big.NewInt(int64(blocksPerDay))
		minAmount           = unit.ConvertIotxToRau(1000)

		tmpVotes   = big.NewInt(0)
		tmpBalance = big.NewInt(0)
	)
	bytecode, err := hex.DecodeString(stakingContractV2Bytecode)
	require.NoError(err)
	mustCallData := func(m string, args ...any) []byte {
		data, err := abiCall(staking.StakingContractABI, m, args...)
		require.NoError(err)
		return data
	}
	genTransferActions := func(n int) []*actionWithTime {
		acts := make([]*actionWithTime, n)
		for i := 0; i < n; i++ {
			acts[i] = &actionWithTime{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()}
		}
		return acts
	}
	test.run([]*testcase{
		{
			name: "deploy staking contract",
			preActs: []*actionWithTime{
				{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(candOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(candOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID), action.WithChainID(chainID))), time.Now()},
			},
			act:    &actionWithTime{mustNoErr(action.SignedExecution("", identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, append(bytecode, mustCallData("", minAmount, common.BytesToAddress(identityset.Address(beneficiaryID).Bytes()))...), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect, &executionExpect{contractAddress}},
		},
		{
			name: "stake",
			preFunc: func(e *e2etest) {
				candidate, err := e.getCandidateByName("cand1")
				require.NoError(err)
				_, ok := tmpVotes.SetString(candidate.TotalWeightedVotes, 10)
				require.True(ok)
			},
			act: &actionWithTime{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), stakeAmount, gasLimit, gasPrice, mustCallData("stake(uint256,address)", stakeDurationBlocks, common.BytesToAddress(identityset.Address(candOwnerID).Bytes())), action.WithChainID(chainID))), stakeTime},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 1, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64(), CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 3, CreateBlockHeight: 3, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: uint64(math.MaxUint64), StakedAmount: stakeAmount.String(), AutoStake: true}},
				&candidateExpect{"cand1", &iotextypes.CandidateV2{OwnerAddress: identityset.Address(candOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), Name: "cand1", TotalWeightedVotes: "1256001586604779503009155", SelfStakingTokens: registerAmount.String(), SelfStakeBucketIdx: 0}},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					candidate, err := test.getCandidateByName("cand1")
					require.NoError(err)
					deltaVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(stakeDurationBlocks.Uint64()/uint64(blocksPerDay)*24) * (time.Hour), StakedAmount: stakeAmount}, false)
					require.Equal(tmpVotes.Add(tmpVotes, deltaVotes).String(), candidate.TotalWeightedVotes)
				}},
			},
		},
		{
			name: "unlock",
			act:  &actionWithTime{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("unlock(uint256)", big.NewInt(1)), action.WithChainID(chainID))), unlockTime},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 1, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64(), CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 4, CreateBlockHeight: 3, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: stakeAmount.String(), AutoStake: false}},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					candidate, err := test.getCandidateByName("cand1")
					require.NoError(err)
					lockedStakeVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(stakeDurationBlocks.Uint64()/uint64(blocksPerDay)*24) * (time.Hour), StakedAmount: stakeAmount}, false)
					unlockedVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: false, StakedDuration: time.Duration(stakeDurationBlocks.Uint64()/uint64(blocksPerDay)*24) * (time.Hour), StakedAmount: stakeAmount}, false)
					tmpVotes.Sub(tmpVotes, lockedStakeVotes)
					tmpVotes.Add(tmpVotes, unlockedVotes)
					require.Equal(tmpVotes.String(), candidate.TotalWeightedVotes)
				}},
			},
		},
		{
			name: "lock",
			act:  &actionWithTime{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("lock(uint256,uint256)", big.NewInt(1), big.NewInt(0).Mul(big.NewInt(2), stakeDurationBlocks)), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 1, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64()/uint64(blocksPerDay)) * 2, StakedDurationBlockNumber: stakeDurationBlocks.Uint64() * 2, CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 3, CreateBlockHeight: 3, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: stakeAmount.String(), AutoStake: true}},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					candidate, err := test.getCandidateByName("cand1")
					require.NoError(err)
					preStakeVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: false, StakedDuration: time.Duration(stakeDurationBlocks.Uint64()/uint64(blocksPerDay)*24) * (time.Hour), StakedAmount: stakeAmount}, false)
					postVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(2*stakeDurationBlocks.Uint64()/uint64(blocksPerDay)*24) * (time.Hour), StakedAmount: stakeAmount}, false)
					tmpVotes.Sub(tmpVotes, preStakeVotes)
					tmpVotes.Add(tmpVotes, postVotes)
					require.Equal(tmpVotes.String(), candidate.TotalWeightedVotes)
				}},
			},
		},
		{
			name: "unstake",
			preActs: append([]*actionWithTime{
				{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("unlock(uint256)", big.NewInt(1)), action.WithChainID(chainID))), unlockTime},
			}, genTransferActions(20)...),
			act: &actionWithTime{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("unstake(uint256)", big.NewInt(1)), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 1, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID).String(), StakedDuration: uint32(2 * stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64() * 2, CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 6, CreateBlockHeight: 3, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: 27, StakedAmount: stakeAmount.String(), AutoStake: false}},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					candidate, err := test.getCandidateByName("cand1")
					require.NoError(err)
					preStakeVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(2*stakeDurationBlocks.Uint64()/uint64(blocksPerDay)*24) * (time.Hour), StakedAmount: stakeAmount}, false)
					tmpVotes.Sub(tmpVotes, preStakeVotes)
					require.Equal(tmpVotes.String(), candidate.TotalWeightedVotes)
				}},
			},
		},
		{
			name: "withdraw",
			preFunc: func(e *e2etest) {
				acc, err := e.api.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: identityset.Address(beneficiaryID).String()})
				require.NoError(err)
				_, ok := tmpBalance.SetString(acc.AccountMeta.Balance, 10)
				require.True(ok)
			},
			preActs: genTransferActions(30),
			act:     &actionWithTime{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("withdraw(uint256,address)", big.NewInt(1), common.BytesToAddress(identityset.Address(beneficiaryID).Bytes())), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect,
				&noBucketExpect{1, contractAddress},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					acc, err := test.api.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: identityset.Address(beneficiaryID).String()})
					require.NoError(err)
					tmpBalance.Add(tmpBalance, stakeAmount)
					require.Equal(tmpBalance.String(), acc.AccountMeta.Balance)
				}},
			},
		},
		{
			name: "change candidate",
			preActs: []*actionWithTime{
				{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(candOwnerID2).String()), "cand2", identityset.Address(2).String(), identityset.Address(2).String(), identityset.Address(candOwnerID2).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID2), action.WithChainID(chainID))), time.Now()},
				{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), stakeAmount, gasLimit, gasPrice, mustCallData("stake(uint256,address)", stakeDurationBlocks, common.BytesToAddress(identityset.Address(candOwnerID).Bytes())), action.WithChainID(chainID))), stakeTime},
			},
			act: &actionWithTime{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("changeDelegate(uint256,address)", big.NewInt(2), common.BytesToAddress(identityset.Address(candOwnerID2).Bytes())), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 2, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64(), CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 60, CreateBlockHeight: 60, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: stakeAmount.String(), AutoStake: true}},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					candidate, err := test.getCandidateByName("cand1")
					require.NoError(err)
					require.Equal(tmpVotes.String(), candidate.TotalWeightedVotes)
				}},
			},
		},
		{
			name: "batch stake",
			preFunc: func(e *e2etest) {
				candidate, err := e.getCandidateByName("cand2")
				require.NoError(err)
				_, ok := tmpVotes.SetString(candidate.TotalWeightedVotes, 10)
				require.True(ok)
			},
			act: &actionWithTime{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0).Mul(big.NewInt(10), stakeAmount), gasLimit, gasPrice, mustCallData("stake(uint256,uint256,address,uint256)", stakeAmount, stakeDurationBlocks, common.BytesToAddress(identityset.Address(candOwnerID2).Bytes()), big.NewInt(10)), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 3, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64(), CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 62, CreateBlockHeight: 62, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: stakeAmount.String(), AutoStake: true}},
				&bucketExpect{&iotextypes.VoteBucket{Index: 12, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64(), CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 62, CreateBlockHeight: 62, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: stakeAmount.String(), AutoStake: true}},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					candidate, err := test.getCandidateByName("cand2")
					require.NoError(err)
					deltaVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(stakeDurationBlocks.Uint64()/uint64(blocksPerDay)*24) * (time.Hour), StakedAmount: stakeAmount}, false)
					tmpVotes.Add(tmpVotes, deltaVotes.Mul(deltaVotes, big.NewInt(10)))
					require.Equal(tmpVotes.String(), candidate.TotalWeightedVotes)
				}},
			},
		},
		{
			name: "merge",
			act:  &actionWithTime{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("merge(uint256[],uint256)", []*big.Int{big.NewInt(3), big.NewInt(4), big.NewInt(5)}, stakeDurationBlocks), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 3, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64(), CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 62, CreateBlockHeight: 62, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: big.NewInt(0).Mul(stakeAmount, big.NewInt(3)).String(), AutoStake: true}},
				&noBucketExpect{4, contractAddress}, &noBucketExpect{5, contractAddress},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					candidate, err := test.getCandidateByName("cand2")
					require.NoError(err)
					subVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(stakeDurationBlocks.Uint64()/uint64(blocksPerDay)*24) * (time.Hour), StakedAmount: stakeAmount}, false)
					addVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(stakeDurationBlocks.Uint64()/uint64(blocksPerDay)*24) * (time.Hour), StakedAmount: big.NewInt(0).Mul(stakeAmount, big.NewInt(3))}, false)
					tmpVotes.Sub(tmpVotes, subVotes.Mul(subVotes, big.NewInt(3)))
					tmpVotes.Add(tmpVotes, addVotes)
					require.Equal(tmpVotes.String(), candidate.TotalWeightedVotes)
				}},
			},
		},
		{
			name: "expand",
			act:  &actionWithTime{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), stakeAmount, gasLimit, gasPrice, mustCallData("expandBucket(uint256,uint256)", big.NewInt(3), big.NewInt(0).Mul(stakeDurationBlocks, big.NewInt(2))), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 3, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64()/uint64(blocksPerDay)) * 2, StakedDurationBlockNumber: stakeDurationBlocks.Uint64() * 2, CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 62, CreateBlockHeight: 62, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: big.NewInt(0).Mul(stakeAmount, big.NewInt(4)).String(), AutoStake: true}},
			},
		},
		{
			name: "donate",
			preFunc: func(e *e2etest) {
				resp, err := test.api.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: identityset.Address(beneficiaryID).String()})
				require.NoError(err)
				_, ok := tmpBalance.SetString(resp.AccountMeta.Balance, 10)
				require.True(ok)
			},
			act: &actionWithTime{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("donate(uint256,uint256)", big.NewInt(3), stakeAmount), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 3, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64()/uint64(blocksPerDay)) * 2, StakedDurationBlockNumber: stakeDurationBlocks.Uint64() * 2, CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 62, CreateBlockHeight: 62, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: big.NewInt(0).Mul(stakeAmount, big.NewInt(3)).String(), AutoStake: true}},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					resp, err := test.api.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: identityset.Address(beneficiaryID).String()})
					require.NoError(err)
					tmpBalance.Add(tmpBalance, stakeAmount)
					require.Equal(tmpBalance.String(), resp.AccountMeta.Balance)
				}},
			},
		},
	})
}

func methodSignToID(sign string) []byte {
	hash := crypto.Keccak256Hash([]byte(sign))
	return hash.Bytes()[:4]
}

func abiCall(_abi abi.ABI, methodSign string, args ...interface{}) ([]byte, error) {
	if methodSign == "" {
		return _abi.Pack("", args...)
	}
	m, err := _abi.MethodById(methodSignToID(methodSign))
	if err != nil {
		return nil, err
	}
	return _abi.Pack(m.Name, args...)
}
