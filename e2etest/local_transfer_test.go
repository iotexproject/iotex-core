// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"github.com/iotexproject/iotex-core/server/itx"
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
	senderPriKey    keypair.PrivateKey
	senderBalance   *big.Int
	recvAcntState   AccountState
	recvPriKey      keypair.PrivateKey
	recvBalance     *big.Int
	nonce           uint64
	amount          *big.Int
	payload         []byte
	gasLimit        uint64
	gasPrice        *big.Int
	expectedResult  TransferState
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
			TsfSuccess, "Normal transfer from an account with enough balance and gas",
		},
		{
			AcntCreate, nil, big.NewInt(1000000),
			AcntBadAddr, nil, big.NewInt(1000000),
			1, big.NewInt(100), // nonce, amount
			make([]byte, 100),             //payload
			uint64(200000), big.NewInt(1), // gasLimit, gasPrice
			TsfFail, "Normal transfer to a bad address",
		},
		{
			AcntNotRegistered, nil, big.NewInt(1000000),
			AcntCreate, nil, big.NewInt(1000000),
			1, big.NewInt(100), // nonce, amount
			make([]byte, 100),             //payload
			uint64(200000), big.NewInt(1), // gasLimit, gasPrice
			TsfFail, "Normal transfer from an address not created on block chain",
		},
		{
			AcntCreate, nil, big.NewInt(1000000),
			AcntNotRegistered, nil, big.NewInt(1000000),
			1, big.NewInt(100), // nonce, amount
			make([]byte, 100),             //payload
			uint64(200000), big.NewInt(1), // gasLimit, gasPrice
			TsfSuccess, "Normal transfer to an address not created on block chain",
		},
		{
			AcntCreate, nil, big.NewInt(1000000),
			AcntCreate, nil, big.NewInt(1000000),
			1, big.NewInt(100),
			make([]byte, 0),
			uint64(1000), big.NewInt(1),
			TsfFail, "Transfer with not enough gas limit",
		},
		{
			AcntCreate, nil, big.NewInt(232222),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(222222),
			make([]byte, 0),
			uint64(200000), big.NewInt(1),
			TsfSuccess, "Transfer with just enough balance",
		},
		{
			AcntCreate, nil, big.NewInt(232221),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(222222),
			make([]byte, 0),
			uint64(200000), big.NewInt(1),
			TsfFail, "Transfer with not enough balance",
		},
		{
			AcntCreate, nil, big.NewInt(232222),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(222222),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFail, "Transfer with not enough balance with payload",
		},
		{
			AcntCreate, nil, big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(0),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfSuccess, "Transfer with 0 amount",
		},
		{
			AcntCreate, nil, big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(-100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFail, "Transfer with negative amount",
		},
		{
			AcntCreate, nil, big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			0, big.NewInt(0),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFail, "Transfer with nonce 0",
		},
		{
			AcntExist, identityset.PrivateKey(0), big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfSuccess, "Transfer with same nonce from a single sender 1",
		},
		{
			AcntExist, identityset.PrivateKey(0), big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFail, "Transfer with same nonce from a single sender 2",
		},
		{
			AcntExist, identityset.PrivateKey(1), big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			2, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfPending, "Transfer with a sequence of nonce from a single sender 1",
		},
		{
			AcntExist, identityset.PrivateKey(1), big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			3, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfPending, "Transfer with a sequence of nonce from a single sender 2",
		},
		{
			AcntExist, identityset.PrivateKey(1), big.NewInt(100000),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFinal, "Transfer with a sequence of nonce from a single sender 3",
		},
		{
			AcntExist, getLocalKey(0), big.NewInt(30000),
			AcntCreate, nil, big.NewInt(100000),
			2, big.NewInt(20000),
			make([]byte, 0),
			uint64(200000), big.NewInt(0),
			TsfPending, "Transfer to multiple accounts with not enough total balance 1",
		},
		{
			AcntExist, getLocalKey(0), big.NewInt(30000),
			AcntCreate, nil, big.NewInt(100000),
			3, big.NewInt(20000),
			make([]byte, 4),
			uint64(200000), big.NewInt(0),
			TsfPending, "Transfer to multiple accounts with not enough total balance 2",
		},
		{
			AcntExist, getLocalKey(0), big.NewInt(30000),
			AcntCreate, nil, big.NewInt(100000),
			1, big.NewInt(20000),
			make([]byte, 4),
			uint64(200000), big.NewInt(0),
			TsfFinal, "Transfer to multiple accounts with not enough total balance 3",
		},
	}
)

func TestLocalTransfer(t *testing.T) {

	require := require.New(t)

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()

	networkPort := 4689
	apiPort := testutil.RandomPort()
	cfg, err := newTransferConfig(testDBPath, testTriePath, networkPort, apiPort)
	require.NoError(err)

	// create server
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.Nil(err)

	// Create and start probe server
	probeSvr := probe.New(7788)
	require.NoError(probeSvr.Start(context.Background()))

	// Start server
	go itx.StartServer(context.Background(), svr, probeSvr, cfg)
	defer func() {
		require.Nil(svr.Stop(ctx))
	}()

	// target address for grpc connection. Default is "127.0.0.1:14014"
	grpcAddr := fmt.Sprintf("127.0.0.1:%d", apiPort)
	grpcctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(grpcctx, grpcAddr, grpc.WithBlock(), grpc.WithInsecure())
	require.NoError(err)
	client := iotexapi.NewAPIServiceClient(conn)

	chainID := cfg.Chain.ID
	bc := svr.ChainService(chainID).Blockchain()
	ap := svr.ChainService(chainID).ActionPool()
	preProcessTestCases(t, bc)
	initExistingAccounts(t, big.NewInt(30000), bc)

	for _, tsfTest := range getSimpleTransferTests {
		senderPriKey, senderAddr, err := initStateKeyAddr(tsfTest.senderAcntState, tsfTest.senderPriKey, tsfTest.senderBalance, bc)
		require.NoError(err, tsfTest.message)

		_, recvAddr, err := initStateKeyAddr(tsfTest.recvAcntState, tsfTest.recvPriKey, tsfTest.recvBalance, bc)
		require.NoError(err, tsfTest.message)

		tsf, err := testutil.SignedTransfer(recvAddr, senderPriKey, tsfTest.nonce, tsfTest.amount,
			tsfTest.payload, tsfTest.gasLimit, tsfTest.gasPrice)
		require.NoError(err, tsfTest.message)

		// wait 2 block time, retry 5 times
		retryInterval := cfg.Genesis.BlockInterval * 2 / 5
		bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(retryInterval), 5)
		err = backoff.Retry(func() error {
			_, err := client.SendAction(context.Background(), &iotexapi.SendActionRequest{Action: tsf.Proto()})
			return err
		}, bo)
		require.NoError(err, tsfTest.message)

		switch tsfTest.expectedResult {
		case TsfSuccess:
			//Wait long enough for a block to be minted, and check the balance of both
			//sender and receiver.
			var selp action.SealedEnvelope
			err := backoff.Retry(func() error {
				var err error
				selp, err = bc.GetActionByActionHash(tsf.Hash())
				return err
			}, bo)
			require.NoError(err, tsfTest.message)
			require.Equal(tsfTest.nonce, selp.Proto().GetCore().GetNonce(), tsfTest.message)
			require.Equal(senderPriKey.PublicKey().Bytes(), selp.Proto().SenderPubKey, tsfTest.message)

			newSenderBalance, _ := bc.Balance(senderAddr)
			minusAmount := big.NewInt(0).Sub(tsfTest.senderBalance, tsfTest.amount)
			gasUnitPayloadConsumed := big.NewInt(0).Mul(big.NewInt(int64(action.TransferPayloadGas)),
				big.NewInt(int64(len(tsfTest.payload))))
			gasUnitTransferConsumed := big.NewInt(int64(action.TransferBaseIntrinsicGas))
			gasUnitConsumed := big.NewInt(0).Add(gasUnitPayloadConsumed, gasUnitTransferConsumed)
			gasConsumed := big.NewInt(0).Mul(gasUnitConsumed, tsfTest.gasPrice)
			expectedSenderBalance := big.NewInt(0).Sub(minusAmount, gasConsumed)
			require.Equal(expectedSenderBalance.String(), newSenderBalance.String(), tsfTest.message)

			newRecvBalance, err := bc.Balance(recvAddr)
			require.NoError(err)
			expectedRecvrBalance := big.NewInt(0)
			if tsfTest.recvAcntState == AcntNotRegistered {
				expectedRecvrBalance.Set(tsfTest.amount)
			} else {
				expectedRecvrBalance.Add(tsfTest.recvBalance, tsfTest.amount)
			}
			require.Equal(expectedRecvrBalance.String(), newRecvBalance.String(), tsfTest.message)
		case TsfFail:
			//The transfer should be rejected right after we inject it
			//Wait long enough to make sure the failed transfer does not exit in either action pool or blockchain
			err := backoff.Retry(func() error {
				var err error
				_, err = ap.GetActionByHash(tsf.Hash())
				return err
			}, bo)
			require.Error(err, tsfTest.message)
			_, err = bc.GetActionByActionHash(tsf.Hash())
			require.Error(err, tsfTest.message)

			if tsfTest.senderAcntState == AcntCreate || tsfTest.senderAcntState == AcntExist {
				newSenderBalance, _ := bc.Balance(senderAddr)
				require.Equal(tsfTest.senderBalance.String(), newSenderBalance.String())
			}

		case TsfPending:
			//Need to wait long enough to make sure the pending transfer is not minted, only stay in action pool
			err := backoff.Retry(func() error {
				var err error
				_, err = ap.GetActionByHash(tsf.Hash())
				return err
			}, bo)
			require.NoError(err, tsfTest.message)
			_, err = bc.GetActionByActionHash(tsf.Hash())
			require.Error(err, tsfTest.message)
		case TsfFinal:
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
	privateKey keypair.PrivateKey,
	initBalance *big.Int,
	bc blockchain.Blockchain,
) (keypair.PrivateKey, string, error) {
	retKey := privateKey
	retAddr := ""
	switch accountState {
	case AcntCreate:
		addr, err := address.FromBytes(retKey.PublicKey().Hash())
		if err != nil {
			return nil, "", err
		}
		retAddr = addr.String()

	case AcntExist:
		addr, err := address.FromBytes(retKey.PublicKey().Hash())
		if err != nil {
			return nil, "", err
		}
		retAddr = addr.String()
		existBalance, err := bc.Balance(retAddr)
		if err != nil {
			return nil, "", err
		}
		initBalance.Set(existBalance)
	case AcntNotRegistered:
		sk, err := keypair.GenerateKey()
		if err != nil {
			return nil, "", err
		}
		addr, err := address.FromBytes(sk.PublicKey().Hash())
		if err != nil {
			return nil, "", err
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

//Initialize accounts that could be used multiple times in some test cases
func initExistingAccounts(
	t *testing.T,
	initBalance *big.Int,
	bc blockchain.Blockchain,
) {
	for i := 0; i < len(localKeys); i++ {
		sk := getLocalKey(i)
		addr, err := address.FromBytes(sk.PublicKey().Hash())
		require.NoError(t, err)
		_, err = bc.CreateState(addr.String(), initBalance)
		require.NoError(t, err)
	}

}

func getLocalKey(i int) keypair.PrivateKey {
	sk, _ := keypair.HexStringToPrivateKey(localKeys[i])
	return sk
}

// Pre processing test cases with "AcntCreate" flag by creating new keys and accounts,
// then filling that test case with the newly created key
func preProcessTestCases(
	t *testing.T,
	bc blockchain.Blockchain,
) {
	for i, tsfTest := range getSimpleTransferTests {
		if tsfTest.senderAcntState == AcntCreate {
			sk, err := keypair.GenerateKey()
			require.NoError(t, err)
			addr, err := address.FromBytes(sk.PublicKey().Hash())
			require.NoError(t, err)
			_, err = bc.CreateState(addr.String(), tsfTest.senderBalance)
			require.NoError(t, err)
			getSimpleTransferTests[i].senderPriKey = sk
		}
		if tsfTest.recvAcntState == AcntCreate {
			sk, err := keypair.GenerateKey()
			require.NoError(t, err)
			addr, err := address.FromBytes(sk.PublicKey().Hash())
			require.NoError(t, err)
			_, err = bc.CreateState(addr.String(), tsfTest.recvBalance)
			require.NoError(t, err)
			getSimpleTransferTests[i].recvPriKey = sk
		}
	}

}

func newTransferConfig(
	chainDBPath,
	trieDBPath string,
	networkPort,
	apiPort int,
) (config.Config, error) {

	cfg := config.Default
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Network.Port = networkPort
	cfg.Chain.ID = 1
	cfg.Chain.ChainDBPath = chainDBPath
	cfg.Chain.TrieDBPath = trieDBPath
	cfg.Chain.EnableAsyncIndexWrite = true
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Consensus.Scheme = config.StandaloneScheme
	cfg.API.Port = apiPort
	cfg.Genesis.BlockInterval = 2 * time.Second

	return cfg, nil
}

func lenPendingActionMap(acts map[string][]action.SealedEnvelope) int {
	l := 0
	for _, part := range acts {
		l += len(part)
	}
	return l
}
