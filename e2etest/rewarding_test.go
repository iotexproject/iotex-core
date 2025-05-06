package e2etest

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/api"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/probe"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/v2/server/itx"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
	"github.com/iotexproject/iotex-core/v2/tools/util"
)

type claimTestCaseID int

const (
	//To claim amount 0
	caseClaimZero claimTestCaseID = iota
	//To claim all unclaimed balance
	caseClaimAll
	//To claim more than unclaimed balance
	caseClaimMoreThanBalance
	//To claim only part of available balance
	caseClaimPartOfBalance
	//To claim a negative amount
	caseClaimNegative
	//To claim with an operator address other than the rewarding address
	caseClaimToNonRewardingAddr
	//Total number of claim test cases, keep this at the bottom the enum
	totalClaimCasesNum
)

func TestBlockReward(t *testing.T) {
	r := require.New(t)
	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	initDBPaths(r, &cfg)
	defer func() { clearDBPaths(&cfg) }()
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Genesis.NumDelegates = 1
	cfg.Genesis.NumSubEpochs = 10
	cfg.Genesis.Delegates = []genesis.Delegate{
		{
			OperatorAddrStr: identityset.Address(0).String(),
			RewardAddrStr:   identityset.Address(0).String(),
			VotesStr:        "10",
		},
	}
	cfg.Genesis.BlockInterval = time.Second
	cfg.Consensus.RollDPoS.FSM.AcceptBlockTTL = 300 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.AcceptProposalEndorsementTTL = 300 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.AcceptLockEndorsementTTL = 300 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.CommitTTL = 100 * time.Millisecond
	cfg.DardanellesUpgrade.AcceptBlockTTL = 300 * time.Millisecond
	cfg.DardanellesUpgrade.AcceptProposalEndorsementTTL = 300 * time.Millisecond
	cfg.DardanellesUpgrade.AcceptLockEndorsementTTL = 300 * time.Millisecond
	cfg.DardanellesUpgrade.CommitTTL = 100 * time.Millisecond
	cfg.DardanellesUpgrade.BlockInterval = time.Second
	cfg.Chain.MintTimeout = 100 * time.Millisecond
	cfg.Chain.ProducerPrivKey = identityset.PrivateKey(0).HexString()
	cfg.Network.Port = testutil.RandomPort()
	cfg.Genesis.PollMode = "lifeLong"
	cfg.Genesis.AleutianBlockHeight = 0
	cfg.Genesis.DardanellesBlockHeight = 1 // enable block reward
	cfg.Genesis.GreenlandBlockHeight = 6
	cfg.Genesis.KamchatkaBlockHeight = 7
	cfg.Genesis.VanuatuBlockHeight = 8      // enable dynamic fee
	cfg.Genesis.ToBeEnabledBlockHeight = 10 // enable wake block reward
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	block.LoadGenesisHash(&cfg.Genesis)

	svr, err := itx.NewServer(cfg)
	require.NoError(t, err)
	require.NoError(t, svr.Start(context.Background()))
	defer func() {
		require.NoError(t, svr.Stop(context.Background()))
	}()

	inject := func(ctx context.Context, tps int) {
		actpool := svr.ChainService(cfg.Chain.ID).ActionPool()
		senderSK := identityset.PrivateKey(1)
		amount := big.NewInt(100)
		gasLimit := uint64(100000)
		gasPrice := big.NewInt(unit.Qev * 2)
		nonce, err := actpool.GetPendingNonce(senderSK.PublicKey().Address().String())
		require.NoError(t, err)
		ticker := time.NewTicker(time.Second / time.Duration(tps))
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				tx, err := action.SignedTransfer(identityset.Address(10).String(), senderSK, nonce, amount, nil, gasLimit, gasPrice, action.WithChainID(cfg.Chain.ID))
				require.NoError(t, err)
				if err = actpool.Add(ctx, tx); err != nil {
					t.Log("failed to add action to actpool", zap.Error(err))
				} else {
					nonce++
				}
			case <-ctx.Done():
				return
			}
		}
	}
	ctxInject, cancel := context.WithCancel(context.Background())
	defer cancel()
	go inject(ctxInject, 10)

	require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 20*time.Second, func() (b bool, e error) {
		return svr.ChainService(1).Blockchain().TipHeight() >= 5, nil
	}))

	ctx := protocol.WithBlockCtx(
		context.Background(),
		protocol.BlockCtx{
			BlockHeight: 0,
		},
	)
	ctx = genesis.WithGenesisContext(ctx, cfg.Genesis)
	ctx = protocol.WithFeatureCtx(ctx)

	rp := rewarding.FindProtocol(svr.ChainService(1).Registry())
	require.NotNil(t, rp)
	sf := svr.ChainService(1).StateFactory()

	sk, err := crypto.HexStringToPrivateKey(cfg.Chain.ProducerPrivKey)
	require.NoError(t, err)
	addr := sk.PublicKey().Address()
	require.NotNil(t, addr)

	blockReward, err := rp.BlockReward(ctx, sf)
	require.NoError(t, err)
	balance, _, err := rp.UnclaimedBalance(ctx, sf, addr)
	require.NoError(t, err)
	assert.True(t, balance.Cmp(big.NewInt(0).Mul(blockReward, big.NewInt(5))) <= 0)

	checkBlockReward := func(blockHeight uint64) {
		blk, err := svr.ChainService(1).BlockDAO().GetBlockByHeight(uint64(blockHeight))
		require.NoError(t, err)
		ok := false
		var (
			gr  *action.GrantReward
			idx int
		)
		for k, act := range blk.Body.Actions {
			gr, ok = act.Action().(*action.GrantReward)
			if ok {
				idx = k
				require.NoError(t, err)
				assert.Equal(t, uint64(blockHeight), gr.Height())
				break
			}
		}
		assert.True(t, ok)
		receipts, err := svr.ChainService(1).BlockDAO().GetReceipts(uint64(blockHeight))
		require.NoError(t, err)
		require.Len(t, receipts[idx].Logs(), 1)
		rewardLogs, err := rewarding.UnmarshalRewardLog(receipts[idx].Logs()[0].Data)
		require.NoError(t, err)

		rewards := make(map[rewardingpb.RewardLog_RewardType]*big.Int)
		rewards[rewardingpb.RewardLog_BLOCK_REWARD] = big.NewInt(0)
		rewards[rewardingpb.RewardLog_PRIORITY_BONUS] = big.NewInt(0)
		for _, txLog := range rewardLogs.Logs {
			amount, ok := big.NewInt(0).SetString(txLog.Amount, 10)
			require.True(t, ok)
			rewards[txLog.Type] = amount
		}

		switch {
		case blockHeight < cfg.Genesis.VanuatuBlockHeight:
			// fixed block reward
			assert.Equal(t, cfg.Genesis.DardanellesBlockReward().String(), rewards[rewardingpb.RewardLog_BLOCK_REWARD].String())
			assert.Equal(t, big.NewInt(0).String(), rewards[rewardingpb.RewardLog_PRIORITY_BONUS].String())
		case blockHeight < cfg.Genesis.ToBeEnabledBlockHeight:
			// fixed block reward + priority bonus
			require.True(t, rewards[rewardingpb.RewardLog_PRIORITY_BONUS].Sign() > 0, "blockHeight %d", blockHeight)
			assert.Equal(t, cfg.Genesis.DardanellesBlockReward().String(), rewards[rewardingpb.RewardLog_BLOCK_REWARD].String())
			assert.Equal(t, slices.ContainsFunc(blk.Actions, func(e *action.SealedEnvelope) bool { return !action.IsSystemAction(e) }), rewards[rewardingpb.RewardLog_PRIORITY_BONUS].Sign() > 0)
		default:
			// dynamic block reward + priority bonus
			require.True(t, rewards[rewardingpb.RewardLog_PRIORITY_BONUS].Sign() > 0)
			assert.Equal(t, slices.ContainsFunc(blk.Actions, func(e *action.SealedEnvelope) bool { return !action.IsSystemAction(e) }), rewards[rewardingpb.RewardLog_PRIORITY_BONUS].Sign() > 0)
			total := new(big.Int).Add(rewards[rewardingpb.RewardLog_BLOCK_REWARD], rewards[rewardingpb.RewardLog_PRIORITY_BONUS])
			assert.Equal(t, true, total.Cmp(cfg.Genesis.WakeBlockReward()) >= 0)
		}
	}

	require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 20*time.Second, func() (b bool, e error) {
		return svr.ChainService(1).Blockchain().TipHeight() >= 11, nil
	}))
	for i := 1; i <= 11; i++ {
		checkBlockReward(uint64(i))
	}
}

func TestBlockEpochReward(t *testing.T) {
	// TODO: fix the test
	t.Skip()

	dbFilePaths := make([]string, 0)

	//Test will stop after reaching this height
	runToHeight := uint64(60)

	//Number of nodes
	numNodes := 4

	// Set mini-cluster configurations
	rand.Seed(time.Now().UnixNano())
	configs := make([]config.Config, numNodes)
	for i := 0; i < numNodes; i++ {
		chainDBPath := fmt.Sprintf("./chain%d.db", i+1)
		dbFilePaths = append(dbFilePaths, chainDBPath)
		trieDBPath := fmt.Sprintf("./trie%d.db", i+1)
		dbFilePaths = append(dbFilePaths, trieDBPath)
		indexDBPath := fmt.Sprintf("./index%d.db", i+1)
		dbFilePaths = append(dbFilePaths, indexDBPath)
		contractIndexDBPath := fmt.Sprintf("./contractindex%d.db", i+1)
		dbFilePaths = append(dbFilePaths, contractIndexDBPath)
		consensusDBPath := fmt.Sprintf("./consensus%d.db", i+1)
		dbFilePaths = append(dbFilePaths, consensusDBPath)
		networkPort := 4689 + i
		apiPort := 14014 + i
		HTTPStatsPort := 8080 + i
		HTTPAdminPort := 9009 + i
		cfg := newConfig(chainDBPath, trieDBPath, indexDBPath, contractIndexDBPath, identityset.PrivateKey(i),
			networkPort, apiPort, uint64(numNodes))
		cfg.Consensus.RollDPoS.ConsensusDBPath = consensusDBPath
		if i == 0 {
			cfg.Network.BootstrapNodes = []string{}
			cfg.Network.MasterKey = "bootnode"
		}

		//Set Operator and Reward address
		cfg.Genesis.Delegates[i].RewardAddrStr = identityset.Address(i + numNodes).String()
		cfg.Genesis.Delegates[i].OperatorAddrStr = identityset.Address(i).String()
		//Generate random votes  from [1000,2000]
		cfg.Genesis.Delegates[i].VotesStr = strconv.Itoa(1000 + rand.Intn(1000))
		cfg.System.HTTPStatsPort = HTTPStatsPort
		cfg.System.HTTPAdminPort = HTTPAdminPort
		configs[i] = cfg
	}

	for _, dbFilePath := range dbFilePaths {
		if fileutil.FileExists(dbFilePath) && os.RemoveAll(dbFilePath) != nil {
			log.L().Error("Failed to delete db file")
		}
	}

	defer func() {
		for _, dbFilePath := range dbFilePaths {
			if fileutil.FileExists(dbFilePath) && os.RemoveAll(dbFilePath) != nil {
				log.L().Error("Failed to delete db file")
			}
		}

	}()
	// Create mini-cluster
	svrs := make([]*itx.Server, numNodes)
	for i := 0; i < numNodes; i++ {
		svr, err := itx.NewServer(configs[i])
		if err != nil {
			log.L().Fatal("Failed to create server.", zap.Error(err))
		}
		svrs[i] = svr
	}

	// Start mini-cluster
	for i := 0; i < numNodes; i++ {
		go itx.StartServer(context.Background(), svrs[i], probe.New(7788+i), configs[i])
	}

	// target address for grpc connection. Default is "127.0.0.1:14014"
	grpcAddr := "127.0.0.1:14014"

	grpcctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(grpcctx, grpcAddr, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		log.L().Error("Failed to connect to API server.")
	}
	defer conn.Close()

	client := iotexapi.NewAPIServiceClient(conn)

	// Get each server's parameters: rewarding protocol, working set, block chain etc.
	rps := make([]*rewarding.Protocol, numNodes)
	sfs := make([]factory.Factory, numNodes)
	chains := make([]blockchain.Blockchain, numNodes)
	apis := make([]*api.ServerV2, numNodes)
	//Map of expected unclaimed balance for each reward address
	exptUnclaimed := make(map[string]*big.Int, numNodes)
	//Map of real unclaimed balance for each reward address
	unClaimedBalances := make(map[string]*big.Int, numNodes)
	//Map of initial balance of both reward and operator address
	initBalances := make(map[string]*big.Int, numNodes)
	//Map of claimed amount for each reward address
	claimedAmount := make(map[string]*big.Int, numNodes)
	//Map to translate from operator address to reward address
	getRewardAddStr := make(map[string]string)

	for i := 0; i < numNodes; i++ {
		rp := rewarding.FindProtocol(svrs[i].ChainService(configs[i].Chain.ID).Registry())
		require.NotNil(t, rp)
		rps[i] = rp

		sfs[i] = svrs[i].ChainService(configs[i].Chain.ID).StateFactory()

		chains[i] = svrs[i].ChainService(configs[i].Chain.ID).Blockchain()
		apis[i] = svrs[i].APIServer(configs[i].Chain.ID)

		rewardAddr := identityset.Address(i + numNodes)
		rewardAddrStr := identityset.Address(i + numNodes).String()
		exptUnclaimed[rewardAddrStr] = big.NewInt(0)
		ctx := genesis.WithGenesisContext(context.Background(), configs[i].Genesis)
		initState, err := accountutil.AccountState(ctx, sfs[i], rewardAddr)
		require.NoError(t, err)
		initBalances[rewardAddrStr] = initState.Balance

		operatorAddr := identityset.Address(i)
		operatorAddrStr := identityset.Address(i).String()
		initState, err = accountutil.AccountState(ctx, sfs[i], operatorAddr)
		require.NoError(t, err)
		initBalances[operatorAddrStr] = initState.Balance

		claimedAmount[rewardAddrStr] = big.NewInt(0)

		getRewardAddStr[identityset.Address(i).String()] = rewardAddrStr

	}

	blocksPerEpoch := configs[0].Genesis.Blockchain.NumDelegates * configs[0].Genesis.Blockchain.NumSubEpochs

	blockReward, err := rps[0].BlockReward(context.Background(), sfs[0])
	require.NoError(t, err)

	//Calculate epoch reward shares for each delegate based on their weight (votes number)
	epochReward, err := rps[0].EpochReward(context.Background(), sfs[0])
	require.NoError(t, err)
	epRwdShares := make(map[string]*big.Int, numNodes)

	totalVotes := big.NewInt(0)
	for i := 0; i < numNodes; i++ {
		totalVotes = totalVotes.Add(configs[0].Genesis.Delegates[i].Votes(), totalVotes)
	}

	for i := 0; i < numNodes; i++ {
		tempShare := big.NewInt(0).Mul(epochReward, configs[0].Genesis.Delegates[i].Votes())
		rewardAddrStr := identityset.Address(i + numNodes).String()
		epRwdShares[rewardAddrStr] = big.NewInt(0).Div(tempShare, totalVotes)
	}

	//Map from action hash to expected result(Fail-false or Success-true), storing pending injected claim actions,
	pendingClaimActions := make(map[hash.Hash256]bool)
	//Start testing
	preHeight := uint64(0)
	preEpochNum := uint64(0)
	preExpectHigh := uint64(0)

	fmt.Println("Starting test")

	if err := testutil.WaitUntil(100*time.Millisecond, 120*time.Second, func() (bool, error) {
		height := chains[0].TipHeight()

		//New height is reached, need to update block reward
		if height > preHeight {

			err = testutil.WaitUntil(100*time.Millisecond, 15*time.Second, func() (bool, error) {
				//This Waituntil block guarantees that we can get a consistent snapshot of the followings at some height:
				// 1) all unclaimed balance live
				// 2) expected unclaimed balance
				// The test keeps comparing these values (after Waituntil block) to make sure everything is correct
				curHigh := chains[0].TipHeight()

				//check pending Claim actions, if a claim is executed, then adjust the expectation accordingly
				//Wait until all the pending actions are settled
				updateExpectationWithPendingClaimList(t, apis[0].CoreService(), exptUnclaimed, claimedAmount, pendingClaimActions)
				if len(pendingClaimActions) > 0 {
					// if there is pending action, retry
					return false, nil
				}

				for i := 0; i < numNodes; i++ {
					rewardAddr := identityset.Address(i + numNodes)
					unClaimedBalances[rewardAddr.String()], _, err =
						rps[0].UnclaimedBalance(context.Background(), sfs[0], rewardAddr)
				}

				if curHigh != chains[0].TipHeight() {
					return false, nil
				}

				//add expected block/epoch reward
				for h := preExpectHigh + 1; h <= curHigh; h++ {
					//Add block reward to current block producer
					header, err := chains[0].BlockHeaderByHeight(h)
					require.NoError(t, err)
					exptUnclaimed[getRewardAddStr[header.ProducerAddress()]] =
						big.NewInt(0).Add(exptUnclaimed[getRewardAddStr[header.ProducerAddress()]], blockReward)

					//update Epoch rewards
					epochNum := h / blocksPerEpoch
					if epochNum > preEpochNum {
						require.Equal(t, epochNum, preEpochNum+1)
						preEpochNum = epochNum

						//Add normal epoch reward
						for i := 0; i < numNodes; i++ {
							rewardAddrStr := identityset.Address(i + numNodes).String()
							expectAfterEpoch := big.NewInt(0).Add(exptUnclaimed[rewardAddrStr], epRwdShares[rewardAddrStr])
							exptUnclaimed[rewardAddrStr] = expectAfterEpoch
						}
						//Add foundation bonus
						foundationBonusLastEpoch, err := rps[0].FoundationBonusLastEpoch(context.Background(), sfs[0])
						require.NoError(t, err)
						foundationBonus, err := rps[0].FoundationBonus(context.Background(), sfs[0])
						require.NoError(t, err)
						if epochNum <= foundationBonusLastEpoch {
							for i := 0; i < numNodes; i++ {
								rewardAddrStr := identityset.Address(i + numNodes).String()
								expectAfterEpochBonus := big.NewInt(0).Add(exptUnclaimed[rewardAddrStr], foundationBonus)
								exptUnclaimed[rewardAddrStr] = expectAfterEpochBonus
							}
						}

					}
					preExpectHigh = h
				}

				//check pending Claim actions, if a claim is executed, then adjust the expectation accordingly
				updateExpectationWithPendingClaimList(t, apis[0].CoreService(), exptUnclaimed, claimedAmount, pendingClaimActions)

				curHighCheck := chains[0].TipHeight()
				preHeight = curHighCheck
				//If chain height changes, we need to take snapshot again.
				return curHigh == curHighCheck, nil

			})
			require.NoError(t, err)

			//Comparing the expected and real unclaimed balance
			for i := 0; i < numNodes; i++ {
				rewardAddrStr := identityset.Address(i + numNodes).String()

				fmt.Println("Server ", i, " ", rewardAddrStr,
					" unclaimed ", unClaimedBalances[rewardAddrStr].String(), " height ", preHeight)
				fmt.Println("Server ", i, " ", rewardAddrStr,
					"  expected ", exptUnclaimed[rewardAddrStr].String())

				require.Equal(t, exptUnclaimed[rewardAddrStr].String(), unClaimedBalances[rewardAddrStr].String())
			}

			// perform a random claim and record the amount
			// chose a random node to claim
			d := rand.Intn(numNodes)
			var amount *big.Int
			rewardAddrStr := identityset.Address(d + numNodes).String()
			rewardPriKey := identityset.PrivateKey(d + numNodes)
			expectedSuccess := true

			rand.Seed(time.Now().UnixNano())
			switch r := rand.Intn(int(totalClaimCasesNum)); claimTestCaseID(r) {
			case caseClaimZero:
				//Claim 0
				amount = big.NewInt(0)
			case caseClaimAll:
				//Claim all
				amount = exptUnclaimed[rewardAddrStr]
			case caseClaimMoreThanBalance:
				//Claim more than available unclaimed balance
				amount = big.NewInt(0).Mul(exptUnclaimed[rewardAddrStr], big.NewInt(2))
			case caseClaimPartOfBalance:
				//Claim random part of available
				amount = big.NewInt(0).Div(exptUnclaimed[rewardAddrStr], big.NewInt(int64(rand.Intn(100000))))
			case caseClaimNegative:
				//Claim negative
				amount = big.NewInt(-100000)
				expectedSuccess = false
			case caseClaimToNonRewardingAddr:
				//Claim to operator address instead of reward address
				rewardPriKey = identityset.PrivateKey(d)
				amount = big.NewInt(12345)
				expectedSuccess = false
			}

			injectClaim(t, nil, client, rewardPriKey, amount,
				expectedSuccess, 3, 1, pendingClaimActions)

		}

		return height > runToHeight, nil
	}); err != nil {

		log.L().Error(err.Error())
	}

	//Wait until all the pending actions are settled
	err = testutil.WaitUntil(100*time.Millisecond, 40*time.Second, func() (bool, error) {
		updateExpectationWithPendingClaimList(t, apis[0].CoreService(), exptUnclaimed, claimedAmount, pendingClaimActions)
		return len(pendingClaimActions) == 0, nil
	})
	require.NoError(t, err)

	for i := 0; i < numNodes; i++ {
		//Check Reward address balance
		rewardAddr := identityset.Address(i + numNodes)
		rewardAddrStr := rewardAddr.String()
		endState, err := accountutil.AccountState(
			genesis.WithGenesisContext(context.Background(), configs[0].Genesis),
			sfs[0],
			rewardAddr,
		)
		require.NoError(t, err)
		fmt.Println("Server ", i, " ", rewardAddrStr, " Closing Balance ", endState.Balance.String())
		expectBalance := big.NewInt(0).Add(initBalances[rewardAddrStr], claimedAmount[rewardAddrStr])
		fmt.Println("Server ", i, " ", rewardAddrStr, "Expected Balance ", expectBalance.String())
		require.Equal(t, expectBalance.String(), endState.Balance.String())

		//Make sure the non-reward addresses have not received money
		operatorAddr := identityset.Address(i)
		operatorAddrStr := identityset.Address(i).String()
		operatorState, err := accountutil.AccountState(
			genesis.WithGenesisContext(context.Background(), configs[i].Genesis),
			sfs[i],
			operatorAddr,
		)
		require.NoError(t, err)
		require.Equal(t, initBalances[operatorAddrStr], operatorState.Balance)
	}
}

func TestClaimReward(t *testing.T) {
	t.Skip("")
	require := require.New(t)
	// set config
	cfg := initCfg(require)
	producerSK, err := crypto.HexStringToPrivateKey(cfg.Chain.ProducerPrivKey)
	cfg.Genesis.TsunamiBlockHeight = 1
	cfg.Genesis.UpernavikBlockHeight = 10
	cfg.Genesis.InitBalanceMap[producerSK.PublicKey().Address().String()] = "100000000000000000000000000"
	cfg.Plugins[config.GatewayPlugin] = struct{}{}
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	// new e2e test
	test := newE2ETest(t, cfg)
	defer test.teardown()
	chainID := cfg.Chain.ID
	gasPrice := big.NewInt(1)
	require.NoError(err)
	genTransferActions := func(n int) []*actionWithTime {
		acts := make([]*actionWithTime, n)
		for i := 0; i < n; i++ {
			acts[i] = &actionWithTime{mustNoErr(action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(2), test.nonceMgr.pop(identityset.Address(2).String()), unit.ConvertIotxToRau(1), nil, gasLimit, gasPrice, action.WithChainID(chainID))), time.Now()}
		}
		return acts
	}
	callerBalance := big.NewInt(0)
	producerBalance := big.NewInt(0)
	// run test
	test.run([]*testcase{
		{
			name:    "v1 claimreward before UpernavikBlockHeight",
			preActs: genTransferActions(5),
			act:     &actionWithTime{mustNoErr(action.SignedClaimRewardLegacy(test.nonceMgr.pop(producerSK.PublicKey().Address().String()), gasLimit, gasPrice, producerSK, unit.ConvertIotxToRau(1), nil, action.WithChainID(chainID))), time.Now()},
			expect:  []actionExpect{successExpect},
		},
		{
			name:   "v2 claimreward before UpernavikBlockHeight",
			act:    &actionWithTime{mustNoErr(action.SignedClaimReward(test.nonceMgr[(identityset.Address(1).String())], gasLimit, gasPrice, identityset.PrivateKey(1), unit.ConvertIotxToRau(1), nil, producerSK.PublicKey().Address(), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{&basicActionExpect{err: errReceiptNotFound}},
		},
		{
			name:    "v1 claimreward after UpernavikBlockHeight",
			preActs: genTransferActions(5),
			act:     &actionWithTime{mustNoErr(action.SignedClaimRewardLegacy(test.nonceMgr.pop(producerSK.PublicKey().Address().String()), gasLimit, gasPrice, producerSK, unit.ConvertIotxToRau(1), nil, action.WithChainID(chainID))), time.Now()},
			expect:  []actionExpect{successExpect},
		},
		{
			name: "v2 claimreward after UpernavikBlockHeight",
			preFunc: func(e *e2etest) {
				resp, err := e.api.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: identityset.Address(1).String()})
				require.NoError(err)
				callerBalance, _ = big.NewInt(0).SetString(resp.GetAccountMeta().Balance, 10)
				resp, err = e.api.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: producerSK.PublicKey().Address().String()})
				require.NoError(err)
				producerBalance, _ = big.NewInt(0).SetString(resp.GetAccountMeta().Balance, 10)
			},
			act: &actionWithTime{mustNoErr(action.SignedClaimReward(test.nonceMgr.pop(identityset.Address(1).String()), gasLimit, gasPrice, identityset.PrivateKey(1), unit.ConvertIotxToRau(1), nil, producerSK.PublicKey().Address(), action.WithChainID(chainID))), time.Now()},
			expect: []actionExpect{successExpect, &functionExpect{func(test *e2etest, act *action.SealedEnvelope, receipt *action.Receipt, err error) {
				// caller balance sub action gas fee
				resp, err := test.api.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: identityset.Address(1).String()})
				require.NoError(err)
				callerBalance.Sub(callerBalance, big.NewInt(0).Mul(big.NewInt(int64(receipt.GasConsumed)), gasPrice))
				require.Equal(callerBalance.String(), resp.GetAccountMeta().Balance)
				// producer balance received 1 IOTX
				resp, err = test.api.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: producerSK.PublicKey().Address().String()})
				require.NoError(err)
				producerBalance.Add(producerBalance, unit.ConvertIotxToRau(1))
				require.Equal(producerBalance.String(), resp.GetAccountMeta().Balance)
			}}},
		},
	})
}

func injectClaim(
	t *testing.T,
	wg *sync.WaitGroup,
	c iotexapi.APIServiceClient,
	beneficiaryPri crypto.PrivateKey,
	amount *big.Int,
	expectedSuccess bool,
	retryNum int,
	retryInterval int,
	pendingClaimActions map[hash.Hash256]bool,
) {
	if wg != nil {
		wg.Add(1)
	}
	payload := []byte{}
	beneficiaryAddr := beneficiaryPri.PublicKey().Address()
	require.NotNil(t, beneficiaryAddr)
	ctx := context.Background()
	request := iotexapi.GetAccountRequest{Address: beneficiaryAddr.String()}
	response, err := c.GetAccount(ctx, &request)
	require.NoError(t, err)
	nonce := response.AccountMeta.PendingNonce

	act := action.NewClaimFromRewardingFund(amount, nil, payload)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(big.NewInt(0)).
		SetGasLimit(100000).
		SetAction(act).Build()

	selp, err := action.Sign(elp, beneficiaryPri)
	require.NoError(t, err)

	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Duration(retryInterval)*time.Second), uint64(retryNum))
	if err := backoff.Retry(func() error {
		_, err := c.SendAction(context.Background(), &iotexapi.SendActionRequest{Action: selp.Proto()})
		return err
	}, bo); err != nil {
		log.L().Error("Failed to inject claim", zap.Error(err))
	}

	if err == nil {
		selpHash, err1 := selp.Hash()
		if err1 != nil {
			log.L().Error("Failed to get hash", zap.Error(err1))
		}
		pendingClaimActions[selpHash] = expectedSuccess
	}

	if wg != nil {
		wg.Done()
	}
}

func updateExpectationWithPendingClaimList(
	t *testing.T,
	api api.CoreService,
	exptUnclaimed map[string]*big.Int,
	claimedAmount map[string]*big.Int,
	pendingClaimActions map[hash.Hash256]bool,
) bool {
	updated := false
	for selpHash, expectedSuccess := range pendingClaimActions {
		receipt, err := util.GetReceiptByAction(api, selpHash)
		if err == nil {
			actInfo, err := util.GetActionByActionHash(api, selpHash)
			require.NoError(t, err)
			addr := actInfo.GetSender()
			require.NotNil(t, addr)

			act := &action.ClaimFromRewardingFund{}
			err = act.LoadProto(actInfo.GetAction().Core.GetClaimFromRewardingFund())
			require.NoError(t, err)
			amount := act.ClaimAmount()

			if receipt.Status == uint64(iotextypes.ReceiptStatus_Success) {
				newExpectUnclaimed := big.NewInt(0).Sub(exptUnclaimed[addr], amount)
				exptUnclaimed[addr] = newExpectUnclaimed

				newClaimedAmount := big.NewInt(0).Add(claimedAmount[addr], amount)
				claimedAmount[addr] = newClaimedAmount
				updated = true

				//An test case expected to fail should never success
				require.NotEqual(t, expectedSuccess, false)
			}

			delete(pendingClaimActions, selpHash)
		}
	}

	return updated
}

func newConfig(
	chainDBPath,
	trieDBPath,
	indexDBPath,
	contractIndexDBPath string,
	producerPriKey crypto.PrivateKey,
	networkPort,
	apiPort int,
	numNodes uint64,
) config.Config {
	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()

	cfg.Network.Port = networkPort
	cfg.Network.BootstrapNodes = []string{"/ip4/127.0.0.1/tcp/4689/ipfs/12D3KooWJwW6pUpTkxPTMv84RPLPMQVEAjZ6fvJuX4oZrvW5DAGQ"}

	cfg.Chain.ID = 1
	cfg.Chain.ChainDBPath = chainDBPath
	cfg.Chain.TrieDBPath = trieDBPath
	cfg.Chain.IndexDBPath = indexDBPath
	cfg.Chain.ContractStakingIndexDBPath = contractIndexDBPath
	cfg.Chain.ProducerPrivKey = producerPriKey.HexString()
	cfg.Chain.EnableAsyncIndexWrite = false

	cfg.ActPool.MinGasPriceStr = big.NewInt(0).String()

	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Consensus.RollDPoS.FSM.UnmatchedEventInterval = 120 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.AcceptBlockTTL = 200 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.AcceptProposalEndorsementTTL = 100 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.AcceptLockEndorsementTTL = 100 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.CommitTTL = 100 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.EventChanSize = 100000
	cfg.Consensus.RollDPoS.ToleratedOvertime = 1200 * time.Millisecond
	cfg.Consensus.RollDPoS.Delay = 6 * time.Second

	cfg.API.GRPCPort = apiPort

	cfg.Genesis.Blockchain.NumSubEpochs = 4
	cfg.Genesis.Blockchain.NumDelegates = numNodes
	cfg.Genesis.Blockchain.TimeBasedRotation = true
	cfg.Genesis.Delegates = cfg.Genesis.Delegates[0:numNodes]
	cfg.Genesis.BlockInterval = 500 * time.Millisecond
	cfg.Genesis.EnableGravityChainVoting = true
	cfg.Genesis.Rewarding.FoundationBonusLastEpoch = 2

	return cfg
}
