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

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/systemcontractindex/stakingindex"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

var (
	//go:embed staking_contract_v2_bytecode
	stakingContractV2Bytecode string
	stakingContractV2ABI      = staking.StakingContractABI
	stakingContractV2Address  = "io1dkqh5mu9djfas3xyrmzdv9frsmmytel4mp7a64"

	//go:embed staking_contract_v3_bytecode
	stakingContractV3Bytecode string
	stakingContractV3ABI      = stakingindex.StakingContractABI
	stakingContractV3Address  = "io1894t0guunycg206syanwal0yqdq4kghe6yj2z8"

	gasPrice1559 = big.NewInt(unit.Qev)
)

func TestContractStakingV2(t *testing.T) {
	require := require.New(t)
	contractAddress := stakingContractV2Address
	cfg := initCfg(require)
	cfg.Genesis.UpernavikBlockHeight = 1
	cfg.Genesis.VanuatuBlockHeight = 100
	cfg.Genesis.WakeBlockHeight = 120 // mute staking v2
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
	genTransferActionsWithPrice := func(n int, price *big.Int) []*actionWithTime {
		acts := make([]*actionWithTime, n)
		for i := 0; i < n; i++ {
			acts[i] = &actionWithTime{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, price, action.WithChainID(chainID))), time.Now()}
		}
		return acts
	}
	genTransferActions := func(n int) []*actionWithTime {
		return genTransferActionsWithPrice(n, gasPrice)
	}
	test.run([]*testcase{
		{
			name: "deploy staking contract",
			preActs: []*actionWithTime{
				{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(candOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(candOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID), action.WithChainID(chainID))), time.Now()},
			},
			act:    &actionWithTime{mustNoErr(action.SignedExecution("", identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, append(bytecode, mustCallData("", minAmount)...), action.WithChainID(chainID))), time.Now()},
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
			preActs: []*actionWithTime{{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("setBeneficiary(address)", common.BytesToAddress(identityset.Address(beneficiaryID).Bytes())), action.WithChainID(chainID))), stakeTime}},
			act:     &actionWithTime{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), stakeAmount, gasLimit, gasPrice, mustCallData("stake(uint256,address)", stakeDurationBlocks, common.BytesToAddress(identityset.Address(candOwnerID).Bytes())), action.WithChainID(chainID))), stakeTime},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 1, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64(), CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 4, CreateBlockHeight: 4, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: uint64(math.MaxUint64), StakedAmount: stakeAmount.String(), AutoStake: true}},
				&candidateExpect{"cand1", &iotextypes.CandidateV2{OwnerAddress: identityset.Address(candOwnerID).String(), Id: identityset.Address(candOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), Name: "cand1", TotalWeightedVotes: "1256001586604779503009155", SelfStakingTokens: registerAmount.String(), SelfStakeBucketIdx: 0}},
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
				&bucketExpect{&iotextypes.VoteBucket{Index: 1, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64(), CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 5, CreateBlockHeight: 4, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: stakeAmount.String(), AutoStake: false}},
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
				&bucketExpect{&iotextypes.VoteBucket{Index: 1, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64()/uint64(blocksPerDay)) * 2, StakedDurationBlockNumber: stakeDurationBlocks.Uint64() * 2, CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 4, CreateBlockHeight: 4, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: stakeAmount.String(), AutoStake: true}},
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
				&bucketExpect{&iotextypes.VoteBucket{Index: 1, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID).String(), StakedDuration: uint32(2 * stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64() * 2, CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 7, CreateBlockHeight: 4, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: 28, StakedAmount: stakeAmount.String(), AutoStake: false}},
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
				&bucketExpect{&iotextypes.VoteBucket{Index: 2, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64(), CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 61, CreateBlockHeight: 61, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: stakeAmount.String(), AutoStake: true}},
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
				&bucketExpect{&iotextypes.VoteBucket{Index: 3, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64(), CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 63, CreateBlockHeight: 63, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: stakeAmount.String(), AutoStake: true}},
				&bucketExpect{&iotextypes.VoteBucket{Index: 12, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64(), CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 63, CreateBlockHeight: 63, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: stakeAmount.String(), AutoStake: true}},
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
				&bucketExpect{&iotextypes.VoteBucket{Index: 3, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64(), CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 63, CreateBlockHeight: 63, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: big.NewInt(0).Mul(stakeAmount, big.NewInt(3)).String(), AutoStake: true}},
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
				&bucketExpect{&iotextypes.VoteBucket{Index: 3, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64()/uint64(blocksPerDay)) * 2, StakedDurationBlockNumber: stakeDurationBlocks.Uint64() * 2, CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 63, CreateBlockHeight: 63, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: big.NewInt(0).Mul(stakeAmount, big.NewInt(4)).String(), AutoStake: true}},
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
				&bucketExpect{&iotextypes.VoteBucket{Index: 3, ContractAddress: contractAddress, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64()/uint64(blocksPerDay)) * 2, StakedDurationBlockNumber: stakeDurationBlocks.Uint64() * 2, CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 63, CreateBlockHeight: 63, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: math.MaxUint64, StakedAmount: big.NewInt(0).Mul(stakeAmount, big.NewInt(3)).String(), AutoStake: true}},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					resp, err := test.api.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: identityset.Address(beneficiaryID).String()})
					require.NoError(err)
					tmpBalance.Add(tmpBalance, stakeAmount)
					require.Equal(tmpBalance.String(), resp.AccountMeta.Balance)
				}},
			},
		},
		{
			name: "mute",
			preFunc: func(e *e2etest) {
				candidate, err := e.getCandidateByName("cand1")
				require.NoError(err)
				_, ok := tmpVotes.SetString(candidate.TotalWeightedVotes, 10)
				require.True(ok)
			},
			preActs: genTransferActionsWithPrice(60, gasPrice1559),
			act:     &actionWithTime{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), stakeAmount, gasLimit, gasPrice1559, mustCallData("stake(uint256,address)", stakeDurationBlocks, common.BytesToAddress(identityset.Address(candOwnerID).Bytes())), action.WithChainID(chainID))), time.Now()},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				require.NoError(err)
				require.EqualValues(uint64(127), blk.Height())
				candidate, err := test.getCandidateByName("cand1")
				require.NoError(err)
				// muted bucket should not be counted in total votes
				require.Equal(tmpVotes.String(), candidate.TotalWeightedVotes)
			},
		},
	})
}

func TestContractStakingV3(t *testing.T) {
	require := require.New(t)
	contractV2Address := stakingContractV2Address
	contractV2AddressEth := common.BytesToAddress(assertions.MustNoErrorV(address.FromString(contractV2Address)).Bytes())
	contractV3Address := stakingContractV3Address
	contractV3AddressEth := common.BytesToAddress(assertions.MustNoErrorV(address.FromString(contractV3Address)).Bytes())
	cfg := initCfg(require)
	cfg.Genesis.UpernavikBlockHeight = 1
	cfg.Genesis.VanuatuBlockHeight = 100
	cfg.Genesis.WakeBlockHeight = 120 // mute staking v2 & enable staking v3
	cfg.Genesis.SystemStakingContractV2Address = contractV2Address
	cfg.Genesis.SystemStakingContractV2Height = 1
	cfg.Genesis.SystemStakingContractV3Address = contractV3Address
	cfg.Genesis.SystemStakingContractV3Height = 1
	cfg.DardanellesUpgrade.BlockInterval = time.Second * 8640
	cfg.Plugins[config.GatewayPlugin] = nil
	test := newE2ETest(t, cfg)
	defer test.teardown()

	var (
		successExpect        = &basicActionExpect{nil, uint64(iotextypes.ReceiptStatus_Success), ""}
		chainID              = test.cfg.Chain.ID
		contractCreator      = 1
		stakerID             = 2
		beneficiaryID        = 10
		stakeAmount          = unit.ConvertIotxToRau(10000)
		registerAmount       = unit.ConvertIotxToRau(1200000)
		stakeTime            = time.Now()
		unlockTime           = stakeTime.Add(time.Hour)
		unstakeTime          = unlockTime.Add(2 * 24 * time.Hour)
		withdrawTime         = unstakeTime.Add(3 * 24 * time.Hour)
		candOwnerID          = 3
		candOwnerID2         = 4
		blocksPerDay         = 24 * time.Hour / cfg.DardanellesUpgrade.BlockInterval
		stakeDurationBlocks  = big.NewInt(int64(blocksPerDay))
		secondsPerDay        = 24 * 3600
		stakeDurationSeconds = big.NewInt(int64(secondsPerDay)) // 1 day
		minAmount            = unit.ConvertIotxToRau(1000)

		tmpVotes   = big.NewInt(0)
		tmpBalance = big.NewInt(0)
	)
	bytecodeV2, err := hex.DecodeString(stakingContractV2Bytecode)
	require.NoError(err)
	mustCallData := func(m string, args ...any) []byte {
		data, err := abiCall(staking.StakingContractABI, m, args...)
		require.NoError(err)
		return data
	}
	bytecodeV3, err := hex.DecodeString(stakingContractV3Bytecode)
	require.NoError(err)
	mustCallDataV3 := func(m string, args ...any) []byte {
		data, err := abiCall(stakingContractV3ABI, m, args...)
		require.NoError(err)
		return data
	}
	test.run([]*testcase{
		{
			name: "deploy_contract_v2",
			preActs: []*actionWithTime{
				{mustNoErr(action.SignedCandidateRegister(test.nonceMgr.pop(identityset.Address(candOwnerID).String()), "cand1", identityset.Address(1).String(), identityset.Address(1).String(), identityset.Address(candOwnerID).String(), registerAmount.String(), 1, true, nil, gasLimit, gasPrice, identityset.PrivateKey(candOwnerID), action.WithChainID(chainID))), time.Now()},
			},
			acts: []*actionWithTime{
				{mustNoErr(action.SignedExecution("", identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, append(bytecodeV2, mustCallData("", minAmount)...), action.WithChainID(chainID))), time.Now()},
				{mustNoErr(action.SignedExecution(contractV2Address, identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("setBeneficiary(address)", common.BytesToAddress(identityset.Address(beneficiaryID).Bytes())), action.WithChainID(chainID))), stakeTime},
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				require.NoError(err)
				require.EqualValues(3, len(blk.Receipts))
				for _, receipt := range blk.Receipts {
					require.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
				}
				require.Equal(contractV2Address, blk.Receipts[0].ContractAddress)
			},
		},
		{
			name: "deploy_contract_v3",
			acts: []*actionWithTime{
				{mustNoErr(action.SignedExecution("", identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, append(bytecodeV3, mustCallDataV3("", minAmount, contractV2AddressEth)...), action.WithChainID(chainID))), time.Now()},
				{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, mustCallDataV3("setBeneficiary(address)", common.BytesToAddress(identityset.Address(beneficiaryID).Bytes())), action.WithChainID(chainID))), time.Now()},
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				require.NoError(err)
				require.EqualValues(3, len(blk.Receipts))
				for _, receipt := range blk.Receipts {
					require.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
				}
				require.Equal(contractV3Address, blk.Receipts[0].ContractAddress)
			},
		},
		{
			name: "stake",
			preFunc: func(e *e2etest) {
				candidate, err := e.getCandidateByName("cand1")
				require.NoError(err)
				_, ok := tmpVotes.SetString(candidate.TotalWeightedVotes, 10)
				require.True(ok)
			},
			act: &actionWithTime{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), stakeAmount, gasLimit, gasPrice, mustCallData("stake(uint256,address)", stakeDurationSeconds, common.BytesToAddress(identityset.Address(candOwnerID).Bytes())), action.WithChainID(chainID))), stakeTime},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 1, ContractAddress: contractV3Address, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID).String(), StakedDuration: uint32(stakeDurationSeconds.Uint64() / uint64(secondsPerDay)), CreateTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), StakeStartTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), UnstakeStartTime: timestamppb.New(time.Unix(0, 0)), StakedAmount: stakeAmount.String(), AutoStake: true}},
				&candidateExpect{"cand1", &iotextypes.CandidateV2{OwnerAddress: identityset.Address(candOwnerID).String(), Id: identityset.Address(candOwnerID).String(), OperatorAddress: identityset.Address(1).String(), RewardAddress: identityset.Address(1).String(), Name: "cand1", TotalWeightedVotes: "1256001586604779503009155", SelfStakingTokens: registerAmount.String(), SelfStakeBucketIdx: 0}},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					candidate, err := test.getCandidateByName("cand1")
					require.NoError(err)
					deltaVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(stakeDurationSeconds.Int64()) * (time.Second), StakedAmount: stakeAmount, Timestamped: true}, false)
					require.Equal(tmpVotes.Add(tmpVotes, deltaVotes).String(), candidate.TotalWeightedVotes)
				}},
			},
		},
		{
			name: "unlock",
			act:  &actionWithTime{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallDataV3("unlock(uint256)", big.NewInt(1)), action.WithChainID(chainID))), unlockTime},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 1, ContractAddress: contractV3Address, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID).String(), StakedDuration: uint32(stakeDurationSeconds.Uint64() / uint64(secondsPerDay)), CreateTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), StakeStartTime: timestamppb.New(time.Unix(unlockTime.Unix(), 0)), UnstakeStartTime: timestamppb.New(time.Unix(0, 0)), StakedAmount: stakeAmount.String(), AutoStake: false}},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					candidate, err := test.getCandidateByName("cand1")
					require.NoError(err)
					lockedStakeVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(stakeDurationSeconds.Int64()) * (time.Second), StakedAmount: stakeAmount}, false)
					unlockedVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: false, StakedDuration: time.Duration(stakeDurationSeconds.Int64()) * (time.Second), StakedAmount: stakeAmount}, false)
					tmpVotes.Sub(tmpVotes, lockedStakeVotes)
					tmpVotes.Add(tmpVotes, unlockedVotes)
					require.Equal(tmpVotes.String(), candidate.TotalWeightedVotes)
				}},
			},
		},
		{
			name: "lock",
			act:  &actionWithTime{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallDataV3("lock(uint256,uint256)", big.NewInt(1), big.NewInt(0).Mul(big.NewInt(2), stakeDurationSeconds)), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 1, ContractAddress: contractV3Address, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID).String(), StakedDuration: uint32(2 * stakeDurationSeconds.Uint64() / uint64(secondsPerDay)), CreateTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), StakeStartTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), UnstakeStartTime: timestamppb.New(time.Unix(0, 0)), StakedAmount: stakeAmount.String(), AutoStake: true}},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					candidate, err := test.getCandidateByName("cand1")
					require.NoError(err)
					preStakeVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: false, StakedDuration: time.Duration(stakeDurationSeconds.Uint64()) * (time.Second), StakedAmount: stakeAmount}, false)
					postVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(2*stakeDurationSeconds.Uint64()) * (time.Second), StakedAmount: stakeAmount}, false)
					tmpVotes.Sub(tmpVotes, preStakeVotes)
					tmpVotes.Add(tmpVotes, postVotes)
					require.Equal(tmpVotes.String(), candidate.TotalWeightedVotes)
				}},
			},
		},
		{
			name: "unstake",
			preActs: []*actionWithTime{
				{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("unlock(uint256)", big.NewInt(1)), action.WithChainID(chainID))), unlockTime},
			},
			act: &actionWithTime{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("unstake(uint256)", big.NewInt(1)), action.WithChainID(chainID))), unstakeTime},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 1, ContractAddress: contractV3Address, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID).String(), StakedDuration: uint32(2 * stakeDurationSeconds.Uint64() / uint64(secondsPerDay)), CreateTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), StakeStartTime: timestamppb.New(time.Unix(unlockTime.Unix(), 0)), UnstakeStartTime: timestamppb.New(time.Unix(unstakeTime.Unix(), 0)), StakedAmount: stakeAmount.String(), AutoStake: false}},
			},
			blockExpect: func(test *e2etest, blk *block.Block, err error) {
				require.NoError(err)
				candidate, err := test.getCandidateByName("cand1")
				require.NoError(err)
				preStakeVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(2*stakeDurationSeconds.Uint64()) * (time.Second), StakedAmount: stakeAmount}, false)
				tmpVotes.Sub(tmpVotes, preStakeVotes)
				require.Equal(tmpVotes.String(), candidate.TotalWeightedVotes)
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
			act: &actionWithTime{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallDataV3("withdraw(uint256,address)", big.NewInt(1), common.BytesToAddress(identityset.Address(beneficiaryID).Bytes())), action.WithChainID(chainID))), withdrawTime},
			expect: []actionExpect{successExpect,
				&noBucketExpect{1, contractV3Address},
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
				{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), stakeAmount, gasLimit, gasPrice, mustCallDataV3("stake(uint256,address)", stakeDurationSeconds, common.BytesToAddress(identityset.Address(candOwnerID).Bytes())), action.WithChainID(chainID))), stakeTime},
			},
			act: &actionWithTime{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallDataV3("changeDelegate(uint256,address)", big.NewInt(2), common.BytesToAddress(identityset.Address(candOwnerID2).Bytes())), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 2, ContractAddress: contractV3Address, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationSeconds.Uint64() / uint64(secondsPerDay)), CreateTime: timestamppb.New(time.Unix(int64(stakeTime.Unix()), 0)), StakeStartTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), UnstakeStartTime: timestamppb.New(time.Unix(0, 0)), StakedAmount: stakeAmount.String(), AutoStake: true}},
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
			act: &actionWithTime{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0).Mul(big.NewInt(10), stakeAmount), gasLimit, gasPrice, mustCallDataV3("stake(uint256,uint256,address,uint256)", stakeAmount, stakeDurationSeconds, common.BytesToAddress(identityset.Address(candOwnerID2).Bytes()), big.NewInt(10)), action.WithChainID(chainID))), stakeTime},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 3, ContractAddress: contractV3Address, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationSeconds.Uint64() / uint64(secondsPerDay)), CreateTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), StakeStartTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), UnstakeStartTime: timestamppb.New(time.Unix(0, 0)), StakedAmount: stakeAmount.String(), AutoStake: true}},
				&bucketExpect{&iotextypes.VoteBucket{Index: 12, ContractAddress: contractV3Address, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationSeconds.Uint64() / uint64(secondsPerDay)), CreateTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), StakeStartTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), UnstakeStartTime: timestamppb.New(time.Unix(0, 0)), StakedAmount: stakeAmount.String(), AutoStake: true}},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					candidate, err := test.getCandidateByName("cand2")
					require.NoError(err)
					deltaVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(stakeDurationSeconds.Uint64()) * (time.Second), StakedAmount: stakeAmount}, false)
					tmpVotes.Add(tmpVotes, deltaVotes.Mul(deltaVotes, big.NewInt(10)))
					require.Equal(tmpVotes.String(), candidate.TotalWeightedVotes)
				}},
			},
		},
		{
			name: "merge",
			act:  &actionWithTime{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("merge(uint256[],uint256)", []*big.Int{big.NewInt(3), big.NewInt(4), big.NewInt(5)}, stakeDurationSeconds), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 3, ContractAddress: contractV3Address, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationSeconds.Uint64() / uint64(secondsPerDay)), CreateTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), StakeStartTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), UnstakeStartTime: timestamppb.New(time.Unix(0, 0)), StakedAmount: big.NewInt(0).Mul(stakeAmount, big.NewInt(3)).String(), AutoStake: true}},
				&noBucketExpect{4, contractV3Address}, &noBucketExpect{5, contractV3Address},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					candidate, err := test.getCandidateByName("cand2")
					require.NoError(err)
					subVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(stakeDurationSeconds.Uint64()) * (time.Second), StakedAmount: stakeAmount}, false)
					addVotes := staking.CalculateVoteWeight(test.cfg.Genesis.VoteWeightCalConsts, &staking.VoteBucket{AutoStake: true, StakedDuration: time.Duration(stakeDurationSeconds.Uint64()) * (time.Second), StakedAmount: big.NewInt(0).Mul(stakeAmount, big.NewInt(3))}, false)
					tmpVotes.Sub(tmpVotes, subVotes.Mul(subVotes, big.NewInt(3)))
					tmpVotes.Add(tmpVotes, addVotes)
					require.Equal(tmpVotes.String(), candidate.TotalWeightedVotes)
				}},
			},
		},
		{
			name: "expand",
			act:  &actionWithTime{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), stakeAmount, gasLimit, gasPrice, mustCallData("expandBucket(uint256,uint256)", big.NewInt(3), big.NewInt(0).Mul(stakeDurationSeconds, big.NewInt(2))), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 3, ContractAddress: contractV3Address, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationSeconds.Uint64()/uint64(secondsPerDay)) * 2, CreateTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), StakeStartTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), UnstakeStartTime: timestamppb.New(time.Unix(0, 0)), StakedAmount: big.NewInt(0).Mul(stakeAmount, big.NewInt(4)).String(), AutoStake: true}},
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
			act: &actionWithTime{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("donate(uint256,uint256)", big.NewInt(3), stakeAmount), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 3, ContractAddress: contractV3Address, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID2).String(), StakedDuration: uint32(stakeDurationSeconds.Uint64()/uint64(secondsPerDay)) * 2, CreateTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), StakeStartTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), UnstakeStartTime: timestamppb.New(time.Unix(0, 0)), StakedAmount: big.NewInt(0).Mul(stakeAmount, big.NewInt(3)).String(), AutoStake: true}},
				&functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
					resp, err := test.api.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: identityset.Address(beneficiaryID).String()})
					require.NoError(err)
					tmpBalance.Add(tmpBalance, stakeAmount)
					require.Equal(tmpBalance.String(), resp.AccountMeta.Balance)
				}},
			},
		},
		{
			name: "migrate",
			preActs: []*actionWithTime{
				{mustNoErr(action.SignedExecution(contractV2Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), stakeAmount, gasLimit, gasPrice, mustCallData("stake(uint256,address)", stakeDurationBlocks, common.BytesToAddress(identityset.Address(candOwnerID).Bytes())), action.WithChainID(chainID))), stakeTime},
				{mustNoErr(action.SignedExecution(contractV2Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("approve(address,uint256)", contractV3AddressEth, big.NewInt(1)), action.WithChainID(chainID))), stakeTime},
			},
			act: &actionWithTime{mustNoErr(action.SignedExecution(contractV3Address, identityset.PrivateKey(stakerID), test.nonceMgr.pop(identityset.Address(stakerID).String()), big.NewInt(0), gasLimit, gasPrice, mustCallDataV3("migrateLegacyBucket(uint256)", big.NewInt(1)), action.WithChainID(chainID))), stakeTime},
			expect: []actionExpect{successExpect,
				&bucketExpect{&iotextypes.VoteBucket{Index: 13, ContractAddress: contractV3Address, Owner: identityset.Address(stakerID).String(), CandidateAddress: identityset.Address(candOwnerID).String(), StakedDuration: uint32(stakeDurationBlocks.Uint64() * 5 / uint64(secondsPerDay)), CreateTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), StakeStartTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), UnstakeStartTime: timestamppb.New(time.Unix(0, 0)), StakedAmount: stakeAmount.String(), AutoStake: true}},
				&bucketExpect{&iotextypes.VoteBucket{Index: 1, ContractAddress: contractV2Address, Owner: contractV3Address, CandidateAddress: address.ZeroAddress, StakedDuration: uint32(stakeDurationBlocks.Uint64() / uint64(blocksPerDay)), StakedDurationBlockNumber: stakeDurationBlocks.Uint64(), CreateTime: timestamppb.New(time.Time{}), StakeStartTime: timestamppb.New(time.Time{}), StakeStartBlockHeight: 17, CreateBlockHeight: 17, UnstakeStartTime: timestamppb.New(time.Time{}), UnstakeStartBlockHeight: uint64(math.MaxUint64), StakedAmount: stakeAmount.String(), AutoStake: true}},
			},
		},
	})
}

func TestMigrateStake(t *testing.T) {
	require := require.New(t)
	registerAmount, _ := big.NewInt(0).SetString("1200000000000000000000000", 10)
	gasLimit := uint64(10000000)
	gasPrice := gasPrice1559
	t.Run("migrate_to_v3", func(t *testing.T) {
		contractAddress := "io1dkqh5mu9djfas3xyrmzdv9frsmmytel4mp7a64"
		cfg := initCfg(require)
		cfg.Genesis.SystemStakingContractV2Address = address.ZeroAddress
		cfg.Genesis.SystemStakingContractV2Height = 1
		cfg.Genesis.SystemStakingContractV3Address = contractAddress
		cfg.Genesis.SystemStakingContractV3Height = 1
		cfg.Genesis.VanuatuBlockHeight = 1
		cfg.Genesis.WakeBlockHeight = 1
		testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
		cfg.DardanellesUpgrade.BlockInterval = time.Second * 8640
		cfg.Plugins[config.GatewayPlugin] = nil
		test := newE2ETest(t, cfg)
		defer test.teardown()

		chainID := test.cfg.Chain.ID
		stakerID := 1
		contractCreator := 1
		stakeAmount, _ := big.NewInt(0).SetString("10000000000000000000000", 10)
		stakeDurationDays := uint32(1) // 1day
		stakeTime := time.Now()
		candOwnerID := 2
		balance := big.NewInt(0)
		h := identityset.Address(1).String()
		t.Logf("address 1: %v\n", h)
		minAmount, _ := big.NewInt(0).SetString("1000000000000000000000", 10) // 1000 IOTX
		bytecode, err := hex.DecodeString(stakingContractV3Bytecode)
		require.NoError(err)
		mustCallData := func(m string, args ...any) []byte {
			data, err := abiCall(stakingContractV3ABI, m, args...)
			require.NoError(err)
			return data
		}
		poorID := 30
		contractV2AddressEth := common.BytesToAddress(mustNoErr(address.FromString(address.ZeroAddress)).Bytes())
		beneficiaryID := 10
		test.run([]*testcase{
			{
				name: "deploy_contract_v3",
				acts: []*actionWithTime{
					{mustNoErr(action.SignedExecution("", identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, append(bytecode, mustCallData("", minAmount, contractV2AddressEth)...), action.WithChainID(chainID))), time.Now()},
					{mustNoErr(action.SignedExecution(contractAddress, identityset.PrivateKey(contractCreator), test.nonceMgr.pop(identityset.Address(contractCreator).String()), big.NewInt(0), gasLimit, gasPrice, mustCallData("setBeneficiary(address)", common.BytesToAddress(identityset.Address(beneficiaryID).Bytes())), action.WithChainID(chainID))), time.Now()},
				},
				blockExpect: func(test *e2etest, blk *block.Block, err error) {
					require.NoError(err)
					t.Log("contract address:", blk.Receipts[0].ContractAddress)
					require.EqualValues(3, len(blk.Receipts))
					for _, receipt := range blk.Receipts {
						require.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
					}
					require.Equal(contractAddress, blk.Receipts[0].ContractAddress)
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
				act: &actionWithTime{mustNoErr(action.SignedMigrateStake(test.nonceMgr.pop(identityset.Address(stakerID).String()), 1, gasLimit, gasPrice, identityset.PrivateKey(stakerID), action.WithChainID(chainID))), stakeTime},
				expect: []actionExpect{
					successExpect,
					&fullActionExpect{
						address.StakingProtocolAddr, 212012,
						[]*action.TransactionLog{
							{
								Type:      iotextypes.TransactionLogType_GAS_FEE,
								Amount:    new(big.Int).Mul(big.NewInt(int64(action.MigrateStakeBaseIntrinsicGas)), gasPrice),
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
								Amount:    new(big.Int).Mul(big.NewInt(202012), gasPrice),
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
					&bucketExpect{&iotextypes.VoteBucket{Index: 1, CandidateAddress: identityset.Address(candOwnerID).String(), StakedAmount: stakeAmount.String(), AutoStake: true, StakedDuration: stakeDurationDays, StakedDurationBlockNumber: 0, CreateTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), StakeStartTime: timestamppb.New(time.Unix(stakeTime.Unix(), 0)), UnstakeStartTime: timestamppb.New(time.Unix(0, 0)), Owner: identityset.Address(stakerID).String(), ContractAddress: contractAddress}},
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
								Amount:    new(big.Int).Mul(big.NewInt(29447), gasPrice),
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
