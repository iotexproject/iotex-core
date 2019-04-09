package e2etest

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
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
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()

	cfg := config.Default
	cfg.Consensus.Scheme = config.StandaloneScheme
	cfg.Genesis.BlockInterval = time.Second
	cfg.Genesis.EnableGravityChainVoting = true
	cfg.Chain.ProducerPrivKey = identityset.PrivateKey(0).HexString()
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Network.Port = testutil.RandomPort()

	svr, err := itx.NewServer(cfg)
	require.NoError(t, err)
	require.NoError(t, svr.Start(context.Background()))
	defer func() {
		require.NoError(t, svr.Stop(context.Background()))
	}()

	require.NoError(t, testutil.WaitUntil(100*time.Millisecond, 20*time.Second, func() (b bool, e error) {
		return svr.ChainService(1).Blockchain().TipHeight() >= 5, nil
	}))

	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{})

	p, ok := svr.ChainService(1).Registry().Find(rewarding.ProtocolID)
	require.True(t, ok)
	rp, ok := p.(*rewarding.Protocol)
	require.True(t, ok)
	sf := svr.ChainService(1).Blockchain().GetFactory()
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)

	sk, err := keypair.HexStringToPrivateKey(cfg.Chain.ProducerPrivKey)
	require.NoError(t, err)
	addr, err := address.FromBytes(sk.PublicKey().Hash())
	require.NoError(t, err)

	blockReward, err := rp.BlockReward(ctx, ws)
	require.NoError(t, err)
	balance, err := rp.UnclaimedBalance(ctx, ws, addr)
	require.NoError(t, err)
	assert.True(t, balance.Cmp(big.NewInt(0).Mul(blockReward, big.NewInt(5))) >= 0)

	for i := 1; i <= 5; i++ {
		blk, err := svr.ChainService(1).Blockchain().GetBlockByHeight(uint64(i))
		require.NoError(t, err)
		ok := false
		var gr *action.GrantReward
		for _, act := range blk.Body.Actions {
			gr, ok = act.Action().(*action.GrantReward)
			if ok {
				assert.Equal(t, uint64(i), gr.Height())
				break
			}
		}
		assert.True(t, ok)
	}
}

func TestBlockEpochReward(t *testing.T) {

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
		networkPort := 4689 + i
		apiPort := 14014 + i
		config := newConfig(chainDBPath, trieDBPath, identityset.PrivateKey(i),
			networkPort, apiPort, uint64(numNodes))
		if i == 0 {
			config.Network.BootstrapNodes = []string{}
			config.Network.MasterKey = "bootnode"
		}

		//Set Operator and Reward address
		config.Genesis.Delegates[i].RewardAddrStr = identityset.Address(i + numNodes).String()
		config.Genesis.Delegates[i].OperatorAddrStr = identityset.Address(i).String()
		//Generate random votes  from [1000,2000]
		config.Genesis.Delegates[i].VotesStr = strconv.Itoa(1000 + rand.Intn(1000))
		configs[i] = config
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

	// Create a probe server
	probeSvr := probe.New(7788)

	// Start mini-cluster
	for i := 0; i < numNodes; i++ {
		go itx.StartServer(context.Background(), svrs[i], probeSvr, configs[i])
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
	wss := make([]factory.WorkingSet, numNodes)
	chains := make([]blockchain.Blockchain, numNodes)
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
		p, ok := svrs[i].ChainService(configs[i].Chain.ID).Registry().Find(rewarding.ProtocolID)
		require.True(t, ok)
		rp, ok := p.(*rewarding.Protocol)
		require.True(t, ok)
		rps[i] = rp

		sf := svrs[i].ChainService(configs[i].Chain.ID).Blockchain().GetFactory()
		ws, err := sf.NewWorkingSet()
		require.NoError(t, err)
		wss[i] = ws

		chains[i] = svrs[i].ChainService(configs[i].Chain.ID).Blockchain()

		rewardAddrStr := identityset.Address(i + numNodes).String()
		exptUnclaimed[rewardAddrStr] = big.NewInt(0)
		initBalances[rewardAddrStr], err = chains[i].Balance(rewardAddrStr)
		require.NoError(t, err)

		operatorAddrStr := identityset.Address(i).String()
		initBalances[operatorAddrStr], err = chains[i].Balance(operatorAddrStr)
		require.NoError(t, err)

		claimedAmount[rewardAddrStr] = big.NewInt(0)

		getRewardAddStr[identityset.Address(i).String()] = rewardAddrStr

	}

	blocksPerEpoch := configs[0].Genesis.Blockchain.NumDelegates * configs[0].Genesis.Blockchain.NumSubEpochs

	blockReward, err := rps[0].BlockReward(context.Background(), wss[0])
	require.NoError(t, err)

	//Calculate epoch reward shares for each delegate based on their weight (votes number)
	epochReward, err := rps[0].EpochReward(context.Background(), wss[0])
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

			err = testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
				//This Waituntil block guarantees that we can get a consistent snapshot of the followings at some height:
				// 1) all unclaimed balance live
				// 2) expected unclaimed balance
				// The test keeps comparing these values (after Waituntil block) to make sure everything is correct
				curHigh := chains[0].TipHeight()

				//check pending Claim actions, if a claim is executed, then adjust the expectation accordingly
				updateExpectationWithPendingClaimList(t, chains[0], exptUnclaimed, claimedAmount, pendingClaimActions)
				startPendingActNum := len(pendingClaimActions)

				for i := 0; i < numNodes; i++ {
					rewardAddr := identityset.Address(i + numNodes)
					unClaimedBalances[rewardAddr.String()], err =
						rps[0].UnclaimedBalance(context.Background(), wss[0], rewardAddr)
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
						foundationBonusLastEpoch, err := rps[0].FoundationBonusLastEpoch(context.Background(), wss[0])
						require.NoError(t, err)
						foundationBonus, err := rps[0].FoundationBonus(context.Background(), wss[0])
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
				updateExpectationWithPendingClaimList(t, chains[0], exptUnclaimed, claimedAmount, pendingClaimActions)
				endPendingActNum := len(pendingClaimActions)

				curHighCheck := chains[0].TipHeight()
				preHeight = curHighCheck

				//If chain height or pending action changes, we need to take snapshot again.
				return curHigh == curHighCheck && startPendingActNum == endPendingActNum, nil

			})
			require.NoError(t, err)

			//Comparing the expected and real unclaimed balance
			for i := 0; i < numNodes; i++ {
				rewardAddrStr := identityset.Address(i + numNodes).String()

				//This to work around the rare case that a balance is deducted from unclaimed while the receipt
				//is not received yet to update the expectation.
				if unClaimedBalances[rewardAddrStr].Cmp(exptUnclaimed[rewardAddrStr]) < 0 {
					log.L().Info("Claim action execution status not in sync, recalibrating...")
					waitActionToSettle(t, rewardAddrStr, chains[0], exptUnclaimed, claimedAmount, unClaimedBalances, pendingClaimActions)
					require.NoError(t, err)
				}
				fmt.Println("Server ", i, " ", rewardAddrStr,
					" unclaimed ", unClaimedBalances[rewardAddrStr].String(), " height ", preHeight)
				fmt.Println("Server ", i, " ", rewardAddrStr,
					"  expected ", exptUnclaimed[rewardAddrStr].String())

				require.Equal(t, exptUnclaimed[rewardAddrStr].String(), unClaimedBalances[rewardAddrStr].String())
			}

			// Perform a random claim and record the amount
			for i := 0; i < numNodes; i++ {

				var amount *big.Int
				rewardAddrStr := identityset.Address(i + numNodes).String()
				rewardPriKey := identityset.PrivateKey(i + numNodes)
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
					rewardPriKey = identityset.PrivateKey(i)
					amount = big.NewInt(12345)
					expectedSuccess = false
				default:
					continue
				}

				injectClaim(t, nil, client, rewardPriKey, amount,
					expectedSuccess, 3, 1, pendingClaimActions)
			}
		}

		return height > runToHeight, nil
	}); err != nil {

		log.L().Error(err.Error())
	}

	//Wait until all the pending actions are settled
	err = testutil.WaitUntil(100*time.Millisecond, 20*time.Second, func() (bool, error) {
		updateExpectationWithPendingClaimList(t, chains[0], exptUnclaimed, claimedAmount, pendingClaimActions)
		return len(pendingClaimActions) == 0, nil
	})
	require.NoError(t, err)

	for i := 0; i < numNodes; i++ {
		//Check Reward address balance
		rewardAddrStr := identityset.Address(i + numNodes).String()
		endBalance, err := chains[0].Balance(rewardAddrStr)
		fmt.Println("Server ", i, " ", rewardAddrStr, " Closing Balance ", endBalance.String())
		require.NoError(t, err)
		expectBalance := big.NewInt(0).Add(initBalances[rewardAddrStr], claimedAmount[rewardAddrStr])
		fmt.Println("Server ", i, " ", rewardAddrStr, "Expected Balance ", expectBalance.String())
		require.Equal(t, expectBalance.String(), endBalance.String())

		//Make sure the non-reward addresses have not received money
		operatorAddrStr := identityset.Address(i).String()
		operatorBalance, err := chains[i].Balance(operatorAddrStr)
		require.NoError(t, err)
		require.Equal(t, initBalances[operatorAddrStr], operatorBalance)
	}

	return

}

func injectClaim(
	t *testing.T,
	wg *sync.WaitGroup,
	c iotexapi.APIServiceClient,
	beneficiaryPri keypair.PrivateKey,
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
	beneficiaryAddr, err := address.FromBytes(beneficiaryPri.PublicKey().Hash())
	ctx := context.Background()
	request := iotexapi.GetAccountRequest{Address: beneficiaryAddr.String()}
	response, err := c.GetAccount(ctx, &request)
	require.NoError(t, err)
	nonce := response.AccountMeta.PendingNonce

	b := &action.ClaimFromRewardingFundBuilder{}
	act := b.SetAmount(amount).SetData(payload).Build()
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(big.NewInt(0)).
		SetGasLimit(100000).
		SetAction(&act).Build()

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
		pendingClaimActions[selp.Hash()] = expectedSuccess
	}

	if wg != nil {
		wg.Done()
	}
}

func updateExpectationWithPendingClaimList(
	t *testing.T,
	bc blockchain.Blockchain,
	exptUnclaimed map[string]*big.Int,
	claimedAmount map[string]*big.Int,
	pendingClaimActions map[hash.Hash256]bool,
) bool {
	updated := false
	for selpHash, expectedSuccess := range pendingClaimActions {
		receipt, receipterr := bc.GetReceiptByActionHash(selpHash)
		selp, err := bc.GetActionByActionHash(selpHash)
		require.Equal(t, err == nil, receipterr == nil)

		if err == nil {
			addr, err := address.FromBytes(selp.SrcPubkey().Hash())
			require.NoError(t, err)

			act := &action.ClaimFromRewardingFund{}
			err = act.LoadProto(selp.Proto().Core.GetClaimFromRewardingFund())
			require.NoError(t, err)
			amount := act.Amount()

			if receipt.Status == action.SuccessReceiptStatus {
				newExpectUnclaimed := big.NewInt(0).Sub(exptUnclaimed[addr.String()], amount)
				exptUnclaimed[addr.String()] = newExpectUnclaimed

				newClaimedAmount := big.NewInt(0).Add(claimedAmount[addr.String()], amount)
				claimedAmount[addr.String()] = newClaimedAmount
				updated = true

				//An test case expected to fail should never success
				require.NotEqual(t, expectedSuccess, false)
			}

			delete(pendingClaimActions, selpHash)
		}
	}

	return updated
}

//waitActionToSettle wait the claim action on given reward address to settle to update expection correctly
func waitActionToSettle(
	t *testing.T,
	rewardAddrStr string,
	bc blockchain.Blockchain,
	exptUnclaimed map[string]*big.Int,
	claimedAmount map[string]*big.Int,
	unClaimedBalances map[string]*big.Int,
	pendingClaimActions map[hash.Hash256]bool,
) error {
	err := testutil.WaitUntil(100*time.Millisecond, 20*time.Second, func() (bool, error) {
		for selpHash, expectedSuccess := range pendingClaimActions {
			receipt, receipterr := bc.GetReceiptByActionHash(selpHash)
			selp, err := bc.GetActionByActionHash(selpHash)
			require.Equal(t, err == nil, receipterr == nil)

			if err == nil {
				addr, err := address.FromBytes(selp.SrcPubkey().Hash())
				require.NoError(t, err)
				if addr.String() != rewardAddrStr {
					continue
				}

				act := &action.ClaimFromRewardingFund{}
				err = act.LoadProto(selp.Proto().Core.GetClaimFromRewardingFund())
				require.NoError(t, err)
				amount := act.Amount()

				newExpectUnclaimed := big.NewInt(0).Sub(exptUnclaimed[addr.String()], amount)

				if newExpectUnclaimed.Cmp(unClaimedBalances[rewardAddrStr]) != 0 {
					continue
				}
				require.Equal(t, receipt.Status, action.SuccessReceiptStatus)

				exptUnclaimed[addr.String()] = newExpectUnclaimed

				newClaimedAmount := big.NewInt(0).Add(claimedAmount[addr.String()], amount)
				claimedAmount[addr.String()] = newClaimedAmount

				//An test case expected to fail should never success
				require.NotEqual(t, expectedSuccess, false)

				delete(pendingClaimActions, selpHash)
				return true, nil
			}
		}
		return false, nil
	})

	return err
}

func newConfig(
	chainDBPath,
	trieDBPath string,
	producerPriKey keypair.PrivateKey,
	networkPort,
	apiPort int,
	numNodes uint64,
) config.Config {
	cfg := config.Default

	cfg.Plugins[config.GatewayPlugin] = true

	cfg.Network.Port = networkPort
	cfg.Network.BootstrapNodes = []string{"/ip4/127.0.0.1/tcp/4689/ipfs/12D3KooWJwW6pUpTkxPTMv84RPLPMQVEAjZ6fvJuX4oZrvW5DAGQ"}

	cfg.Chain.ID = 1
	cfg.Chain.ChainDBPath = chainDBPath
	cfg.Chain.TrieDBPath = trieDBPath
	cfg.Chain.CompressBlock = true
	cfg.Chain.ProducerPrivKey = producerPriKey.HexString()

	cfg.ActPool.MinGasPriceStr = big.NewInt(0).String()

	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Consensus.RollDPoS.FSM.UnmatchedEventInterval = 40 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.AcceptBlockTTL = 30 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.AcceptProposalEndorsementTTL = 30 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.AcceptLockEndorsementTTL = 30 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.EventChanSize = 100000
	cfg.Consensus.RollDPoS.ToleratedOvertime = 1200 * time.Millisecond
	cfg.Consensus.RollDPoS.Delay = 6 * time.Second

	cfg.API.Port = apiPort

	cfg.Genesis.Blockchain.NumSubEpochs = 4
	cfg.Genesis.Blockchain.NumDelegates = numNodes
	cfg.Genesis.Blockchain.TimeBasedRotation = true
	cfg.Genesis.Delegates = cfg.Genesis.Delegates[0:numNodes]

	cfg.Genesis.BlockInterval = 100 * time.Millisecond
	cfg.Genesis.EnableGravityChainVoting = true

	cfg.Genesis.Rewarding.FoundationBonusLastEpoch = 2
	return cfg
}
