// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

type TransferState int

const (
	//This transfer should fail to be accepted into action pool
	TsfFail TransferState = iota
	//This transfer should be accepted into action pool,
	//and later on be minted into block chain after block creating interval
	TsfSuccess
	//This transfer should be accepted into action pool,
	//but will stay in action pool (not minted yet)
	//until all the blocks with preceding nonce arrive
	TsfPending
	//This transfer should enable all the pending transfer in action pool be accepted
	//into block chain. This happens when a transfer with the missing nonce arrives,
	//filling the gap between minted blocks and pending blocks.
	TsfFinal
)

type AccountState int

const (
	//This account should be created on blockchain in run time with the given balance
	AcntCreate AccountState = iota
	//This account already exist, need to load the the key, address, balance to this test case
	AcntExist
	//This account doesnt exist on blockchain, but have a valid key and address
	AcntNotRegistered
	//This account doesnt exist, the address is not valid (a random byte string)
	AcntBadAddr
)

type simpleTransferTestCfg struct {
	senderAcntState AccountState
	senderPriKey    crypto.PrivateKey
	senderBalance   *big.Int
	recvAcntState   AccountState
	recvPriKey      crypto.PrivateKey
	recvBalance     *big.Int
	nonce           uint64
	amount          *big.Int
	payload         []byte
	gasLimit        uint64
	gasPrice        *big.Int
	expectedResult  TransferState
	expectedDesc    string
	message         string
}

var (
	localKeys = []string{
		"fd26207d4657c422da8242686ba4f5066be11ffe9d342d37967f9538c44cebbf",
		"012d7c684388ca7508fb3483f58e29a8de327b28097dd1d207116225307c98bf",
		"0a653365c521592062fbbd3b8e1fc64a80b6199bce2b1dbac091955b5fe14125",
		"0b3eb204a1641ea072505eec5161043e8c19bd039fad7f61e2180d4d396af45b",
		"affad54ae2fd6f139c235439bebb9810ccdd016911113b220af6fd87c952b5bd",
		"d260035a571390213c8521b73fff47b6fd8ce2474e37a2421bf1d4657e06e3ea",
		"dee8d3dab8fbf36990608936241d1cc6f7d51663285919806eb05b1365dd62a3",
		"d08769fb91911eed6156b1ea7dbb8adf3a68b1ed3b4b173074e7a67996d76c5d",
		"29945a86884def518347585caaddcc9ac08c5d6ca614b8547625541b43adffe7",
		"c8018d8a2ed602831c3435b03e33669d0f59e29c939764f1b11591175f2fe615",
	}
	// In the test case:
	//  - an account with "nil" private key will be created with
	//    keys, address, and initialized with the given balance.
	// - an account with exiting private key will load exiting
	//   balance into test case.
	getSimpleTransferTests = []simpleTransferTestCfg{
		{
			AcntCreate, nil, big.NewInt(1000000),
			AcntCreate, nil, big.NewInt(1000000),
			1, big.NewInt(100), // nonce, amount
			make([]byte, 100),             //payload
			uint64(200000), big.NewInt(1), // gasLimit, gasPrice
			TsfSuccess, "",
			"Normal transfer from an account with enough balance and gas",
		},
		{
			AcntCreate, nil, big.NewInt(232222),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(222222),
			make([]byte, 0),
			uint64(200000), big.NewInt(1),
			TsfSuccess, "",
			"Transfer with just enough balance",
		},
		{
			AcntCreate, nil, big.NewInt(1000000),
			AcntNotRegistered, nil, big.NewInt(1000000),
			1, big.NewInt(100), // nonce, amount
			make([]byte, 100),             //payload
			uint64(200000), big.NewInt(1), // gasLimit, gasPrice
			TsfSuccess, "",
			"Normal transfer to an address not created on block chain",
		},
		{
			AcntCreate, nil, big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(0),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfSuccess, "",
			"Transfer with 0 amount",
		},
		{
			AcntExist, identityset.PrivateKey(0), big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfSuccess, "",
			"Transfer with same nonce from a single sender 1",
		},
		{
			AcntExist, identityset.PrivateKey(1), big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			2, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfPending, "",
			"Transfer with a sequence of nonce from a single sender 1",
		},
		{
			AcntExist, identityset.PrivateKey(1), big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			3, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfPending, "",
			"Transfer with a sequence of nonce from a single sender 2",
		},
		{
			AcntExist, getLocalKey(0), big.NewInt(30000),
			AcntCreate, nil, big.NewInt(100000),
			2, big.NewInt(20000),
			make([]byte, 0),
			uint64(200000), big.NewInt(0),
			TsfPending, "",
			"Transfer to multiple accounts with not enough total balance 1",
		},
		{
			AcntExist, getLocalKey(0), big.NewInt(30000),
			AcntCreate, nil, big.NewInt(100000),
			3, big.NewInt(20000),
			make([]byte, 4),
			uint64(200000), big.NewInt(0),
			TsfPending, "",
			"Transfer to multiple accounts with not enough total balance 2",
		},
		{
			AcntCreate, nil, big.NewInt(1000000),
			AcntBadAddr, nil, big.NewInt(1000000),
			1, big.NewInt(100), // nonce, amount
			make([]byte, 100),             //payload
			uint64(200000), big.NewInt(1), // gasLimit, gasPrice
			TsfFail, "Unknown",
			"Normal transfer to a bad address",
		},
		{
			AcntNotRegistered, nil, big.NewInt(1000000),
			AcntCreate, nil, big.NewInt(1000000),
			1, big.NewInt(100), // nonce, amount
			make([]byte, 100),             //payload
			uint64(200000), big.NewInt(1), // gasLimit, gasPrice
			TsfFail, "Invalid balance",
			"Normal transfer from an address not created on block chain",
		},
		{
			AcntCreate, nil, big.NewInt(232221),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(222222),
			make([]byte, 0),
			uint64(200000), big.NewInt(1),
			TsfFail, "Invalid balance",
			"Transfer with not enough balance",
		},
		{
			AcntCreate, nil, big.NewInt(232222),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(222222),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFail, "Invalid balance",
			"Transfer with not enough balance with payload",
		},
		{
			AcntCreate, nil, big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(-100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFail, "Invalid balance",
			"Transfer with negative amount",
		},
		{
			AcntCreate, nil, big.NewInt(1000000),
			AcntCreate, nil, big.NewInt(1000000),
			1, big.NewInt(100),
			make([]byte, 0),
			uint64(1000), big.NewInt(1),
			TsfFail, "Insufficient balance for gas",
			"Transfer with not enough gas limit",
		},
		{
			AcntCreate, nil, big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			0, big.NewInt(0),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFail, "Invalid nonce",
			"Transfer with nonce 0",
		},
		{
			AcntExist, identityset.PrivateKey(0), big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFail, "Invalid nonce",
			"Transfer with same nonce from a single sender 2",
		},
		{
			AcntExist, identityset.PrivateKey(1), big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFinal, "",
			"Transfer with a sequence of nonce from a single sender 3",
		},
		{
			AcntExist, getLocalKey(0), big.NewInt(30000),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(20000),
			make([]byte, 4),
			uint64(200000), big.NewInt(0),
			TsfFinal, "",
			"Transfer to multiple accounts with not enough total balance 3",
		},
	}
)

func TestLocalTransfer(t *testing.T) {
	require := require.New(t)

	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)
	testBloomfilterIndexPath, err := testutil.PathOfTempFile("bloomfilterIndex")
	require.NoError(err)
	testSystemLogPath, err := testutil.PathOfTempFile("systemlog")
	require.NoError(err)
	testCandidateIndexPath, err := testutil.PathOfTempFile("candidateIndex")
	require.NoError(err)

	defer func() {
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
		testutil.CleanupPath(t, testIndexPath)
		testutil.CleanupPath(t, testSystemLogPath)
		testutil.CleanupPath(t, testBloomfilterIndexPath)
		testutil.CleanupPath(t, testCandidateIndexPath)
	}()

	networkPort := 4689
	apiPort := testutil.RandomPort()
	cfg, err := newTransferConfig(testDBPath, testTriePath, testIndexPath, testBloomfilterIndexPath, testSystemLogPath, testCandidateIndexPath, networkPort, apiPort)
	defer func() {
		delete(cfg.Plugins, config.GatewayPlugin)
	}()
	require.NoError(err)

	for i, tsfTest := range getSimpleTransferTests {
		if tsfTest.senderAcntState == AcntCreate {
			sk, err := crypto.GenerateKey()
			require.NoError(err)
			addr := sk.PublicKey().Address()
			require.NotNil(addr)
			cfg.Genesis.InitBalanceMap[addr.String()] = tsfTest.senderBalance.String()
			getSimpleTransferTests[i].senderPriKey = sk
		}
		if tsfTest.recvAcntState == AcntCreate {
			sk, err := crypto.GenerateKey()
			require.NoError(err)
			addr := sk.PublicKey().Address()
			require.NotNil(addr)
			cfg.Genesis.InitBalanceMap[addr.String()] = tsfTest.recvBalance.String()
			getSimpleTransferTests[i].recvPriKey = sk
		}
	}
	for i := 0; i < len(localKeys); i++ {
		sk := getLocalKey(i)
		addr := sk.PublicKey().Address()
		require.NotNil(addr)
		cfg.Genesis.InitBalanceMap[addr.String()] = "30000"
	}

	// create server
	svr, err := itx.NewServer(cfg)
	require.NoError(err)

	// Create and start probe server
	ctx := context.Background()
	probeSvr := probe.New(7788)
	require.NoError(probeSvr.Start(ctx))

	// Start server
	ctx, stopServer := context.WithCancel(ctx)
	defer func() {
		require.NoError(probeSvr.Stop(ctx))
		stopServer()
	}()

	go itx.StartServer(ctx, svr, probeSvr, cfg)

	// target address for grpc connection. Default is "127.0.0.1:14014"
	grpcAddr := fmt.Sprintf("127.0.0.1:%d", apiPort)
	grpcctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(grpcctx, grpcAddr, grpc.WithBlock(), grpc.WithInsecure())
	require.NoError(err)
	defer conn.Close()
	client := iotexapi.NewAPIServiceClient(conn)

	chainID := cfg.Chain.ID
	bc := svr.ChainService(chainID).Blockchain()
	sf := svr.ChainService(chainID).StateFactory()
	ap := svr.ChainService(chainID).ActionPool()
	as := svr.ChainService(chainID).APIServer()

	for _, tsfTest := range getSimpleTransferTests {
		senderPriKey, senderAddr, err := initStateKeyAddr(tsfTest.senderAcntState, tsfTest.senderPriKey, tsfTest.senderBalance, bc, sf)
		require.NoError(err, tsfTest.message)

		_, recvAddr, err := initStateKeyAddr(tsfTest.recvAcntState, tsfTest.recvPriKey, tsfTest.recvBalance, bc, sf)
		require.NoError(err, tsfTest.message)

		tsf, err := action.SignedTransfer(recvAddr, senderPriKey, tsfTest.nonce, tsfTest.amount,
			tsfTest.payload, tsfTest.gasLimit, tsfTest.gasPrice)
		require.NoError(err, tsfTest.message)

		// wait 2 block time, retry 5 times
		retryInterval := cfg.Genesis.BlockInterval * 2 / 5
		bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(retryInterval), 5)
		err = backoff.Retry(func() error {
			_, err := client.SendAction(context.Background(), &iotexapi.SendActionRequest{Action: tsf.Proto()})
			return err
		}, bo)
		switch tsfTest.expectedResult {
		case TsfSuccess:
			require.NoError(err, tsfTest.message)
			// Wait long enough for a block to be minted, and check the balance of both
			// sender and receiver.
			var selp action.SealedEnvelope
			err := backoff.Retry(func() error {
				var err error
				tsfHash, err1 := tsf.Hash()
				if err1 != nil {
					return err1
				}
				selp, err = as.GetActionByActionHash(tsfHash)
				if err != nil {
					return err
				}
				return err
			}, bo)
			require.NoError(err, tsfTest.message)
			require.Equal(tsfTest.nonce, selp.Proto().GetCore().GetNonce(), tsfTest.message)
			require.Equal(senderPriKey.PublicKey().Bytes(), selp.Proto().SenderPubKey, tsfTest.message)

			newSenderState, _ := accountutil.AccountState(sf, senderAddr)
			minusAmount := big.NewInt(0).Sub(tsfTest.senderBalance, tsfTest.amount)
			gasUnitPayloadConsumed := big.NewInt(0).Mul(big.NewInt(int64(action.TransferPayloadGas)),
				big.NewInt(int64(len(tsfTest.payload))))
			gasUnitTransferConsumed := big.NewInt(int64(action.TransferBaseIntrinsicGas))
			gasUnitConsumed := big.NewInt(0).Add(gasUnitPayloadConsumed, gasUnitTransferConsumed)
			gasConsumed := big.NewInt(0).Mul(gasUnitConsumed, tsfTest.gasPrice)
			expectedSenderBalance := big.NewInt(0).Sub(minusAmount, gasConsumed)
			require.Equal(expectedSenderBalance.String(), newSenderState.Balance.String(), tsfTest.message)

			newRecvState, err := accountutil.AccountState(sf, recvAddr)
			require.NoError(err)
			expectedRecvrBalance := big.NewInt(0)
			if tsfTest.recvAcntState == AcntNotRegistered {
				expectedRecvrBalance.Set(tsfTest.amount)
			} else {
				expectedRecvrBalance.Add(tsfTest.recvBalance, tsfTest.amount)
			}
			require.Equal(expectedRecvrBalance.String(), newRecvState.Balance.String(), tsfTest.message)
		case TsfFail:
			require.Error(err, tsfTest.message)

			st, ok := status.FromError(err)
			require.True(ok, tsfTest.message)
			require.Equal(st.Code(), codes.Internal, tsfTest.message)

			details := st.Details()
			require.Equal(len(details), 1, tsfTest.message)

			detail, ok := details[0].(*errdetails.BadRequest)
			require.True(ok, tsfTest.message)
			require.Equal(len(detail.FieldViolations), 1, tsfTest.message)

			violation := detail.FieldViolations[0]
			require.Equal(violation.Description, tsfTest.expectedDesc, tsfTest.message)
			require.Equal(violation.Field, "Action rejected", tsfTest.message)

			//The transfer should be rejected right after we inject it
			//Wait long enough to make sure the failed transfer does not exit in either action pool or blockchain
			err := backoff.Retry(func() error {
				var err error
				tsfHash, err1 := tsf.Hash()
				if err1 != nil {
					return err1
				}
				_, err = ap.GetActionByHash(tsfHash)
				return err
			}, bo)
			require.Error(err, tsfTest.message)
			tsfHash, err1 := tsf.Hash()
			require.NoError(err1)
			_, err = as.GetActionByActionHash(tsfHash)
			require.Error(err, tsfTest.message)

			if tsfTest.senderAcntState == AcntCreate || tsfTest.senderAcntState == AcntExist {
				newSenderState, _ := accountutil.AccountState(sf, senderAddr)
				require.Equal(tsfTest.senderBalance.String(), newSenderState.Balance.String())
			}

		case TsfPending:
			require.NoError(err, tsfTest.message)
			//Need to wait long enough to make sure the pending transfer is not minted, only stay in action pool
			err := backoff.Retry(func() error {
				var err error
				tsfHash, err1 := tsf.Hash()
				if err1 != nil {
					return err1
				}
				_, err = ap.GetActionByHash(tsfHash)
				return err
			}, bo)
			require.NoError(err, tsfTest.message)
			tsfHash, err1 := tsf.Hash()
			require.NoError(err1)
			_, err = as.GetActionByActionHash(tsfHash)
			require.Error(err, tsfTest.message)
		case TsfFinal:
			require.NoError(err, tsfTest.message)
			//After a blocked is minted, check all the pending transfers in action pool are cleared
			//This checking procedure is simplified for this test case, because of the complexity of
			//handling pending transfers.
			time.Sleep(cfg.Genesis.BlockInterval + time.Second)
			require.Equal(0, lenPendingActionMap(ap.PendingActionMap()), tsfTest.message)

		default:
			require.True(false, tsfTest.message)

		}
	}
}

// initStateKeyAddr, if the given private key is nil,
// creates key, address, and init the new account with given balance
// otherwise, calculate the the address, and load test with existing
// balance state.
func initStateKeyAddr(
	accountState AccountState,
	privateKey crypto.PrivateKey,
	initBalance *big.Int,
	bc blockchain.Blockchain,
	sf factory.Factory,
) (crypto.PrivateKey, string, error) {
	retKey := privateKey
	retAddr := ""
	switch accountState {
	case AcntCreate:
		addr := retKey.PublicKey().Address()
		if addr == nil {
			return nil, "", errors.New("failed to get address")
		}
		retAddr = addr.String()

	case AcntExist:
		addr := retKey.PublicKey().Address()
		if addr == nil {
			return nil, "", errors.New("failed to get address")
		}
		retAddr = addr.String()
		existState, err := accountutil.AccountState(sf, retAddr)
		if err != nil {
			return nil, "", err
		}
		initBalance.Set(existState.Balance)
	case AcntNotRegistered:
		sk, err := crypto.GenerateKey()
		if err != nil {
			return nil, "", err
		}
		addr := sk.PublicKey().Address()
		if addr == nil {
			return nil, "", errors.New("failed to get address")
		}
		retAddr = addr.String()
		retKey = sk
	case AcntBadAddr:
		rand.Seed(time.Now().UnixNano())
		b := make([]byte, 41)
		for i := range b {
			b[i] = byte(65 + rand.Intn(26))
		}
		retAddr = string(b)
	}
	return retKey, retAddr, nil
}

func getLocalKey(i int) crypto.PrivateKey {
	sk, _ := crypto.HexStringToPrivateKey(localKeys[i])
	return sk
}

func newTransferConfig(
	chainDBPath,
	trieDBPath,
	indexDBPath string,
	bloomfilterIndex string,
	systemLogDBPath string,
	candidateIndexDBPath string,
	networkPort,
	apiPort int,
) (config.Config, error) {

	cfg := config.Default
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Network.Port = networkPort
	cfg.Chain.ID = 1
	cfg.Chain.ChainDBPath = chainDBPath
	cfg.Chain.TrieDBPath = trieDBPath
	cfg.Chain.IndexDBPath = indexDBPath
	cfg.Chain.BloomfilterIndexDBPath = bloomfilterIndex
	cfg.System.SystemLogDBPath = systemLogDBPath
	cfg.Chain.CandidateIndexDBPath = candidateIndexDBPath
	cfg.Chain.EnableAsyncIndexWrite = true
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Consensus.Scheme = config.StandaloneScheme
	cfg.API.Port = apiPort
	cfg.Genesis.BlockInterval = 800 * time.Millisecond

	return cfg, nil
}

func lenPendingActionMap(acts map[string][]action.SealedEnvelope) int {
	l := 0
	for _, part := range acts {
		l += len(part)
	}
	return l
}

type chainCase1 struct {
	cfgChainID       uint32
	cfgJutlandHeight uint64
	testChainID      uint32
	err              error
}

func TestChainIDWithJutland(t *testing.T) {

	testCase := []chainCase1{
		{
			1, 30, 1, nil, // tx chainID = node chainID, height < jutland
		},
		{
			2, 31, 2, nil, // tx chainID = node chainID, height >= jutland
		},
		{
			4, 31, 2, nil, // tx chainID != node chainID, height < jutland
		},
		{
			2, 31, 4, nil, // tx chainID != node chainID, height < jutland
		},
		{
			4, 30, 2, action.ErrChainID, // tx chainID != node chainID, height >= jutland
		},
		{
			2, 30, 4, action.ErrChainID, // tx chainID != node chainID, height >= jutland
		},
		{
			4, 10, 2, action.ErrChainID, // tx chainID != node chainID, height >= jutland
		},
		{
			2, 10, 4, action.ErrChainID, // tx chainID != node chainID, height >= jutland
		},
	}

	testFunc := func(c chainCase1) error {
		ctx := context.Background()
		cfg := config.Default
		cfg.Genesis.BlockGasLimit = uint64(100000)
		cfg.Chain.ID = c.cfgChainID
		cfg.Genesis.JutlandBlockHeight = c.cfgJutlandHeight
		registry := protocol.NewRegistry()
		acc := account.NewProtocol(rewarding.DepositGas)
		require.NoError(t, acc.Register(registry))

		sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
		if err != nil {
			return err
		}
		ap, err := actpool.NewActPool(sf, cfg.ActPool)
		if err != nil {
			return err
		}

		blkMemDao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf})
		bc := blockchain.NewBlockchain(
			cfg,
			blkMemDao,
			factory.NewMinter(sf, ap),
		)
		err = bc.Start(ctx)
		if err != nil {
			return err
		}
		for i := 0; i < 30; i++ {
			tsf, err := action.NewTransfer(
				uint64(i)+1,
				big.NewInt(100),
				identityset.Address(27).String(),
				[]byte{}, uint64(100000),
				big.NewInt(1).Mul(big.NewInt(int64(i)+10), big.NewInt(unit.Qev)),
			)
			if err != nil {
				return err
			}

			bd := &action.EnvelopeBuilder{}
			elp1 := bd.SetAction(tsf).
				SetChainID(c.testChainID).
				SetNonce(uint64(i) + 1).
				SetGasLimit(100000).
				SetGasPrice(big.NewInt(1).Mul(big.NewInt(int64(i)+10), big.NewInt(unit.Qev))).Build()
			selp1, err := action.Sign(elp1, identityset.PrivateKey(0))
			if err != nil {
				return err
			}

			err = ap.Add(context.Background(), selp1)
			if err != nil {
				return err
			}
			blk, err := bc.MintNewBlock(testutil.TimestampNow())
			if err != nil {
				return err
			}
			err = bc.CommitBlock(blk)
			if err != nil {
				return err
			}
			err = bc.Stop(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	}
	for _, c := range testCase {
		err := testFunc(c)
		if c.err == nil || err == nil {
			require.Equal(t, err, c.err)
		} else {
			require.ErrorIs(t, err, c.err)
		}
	}
}
