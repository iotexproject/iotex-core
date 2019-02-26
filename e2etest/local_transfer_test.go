// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"encoding/hex"

	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/iotexproject/go-ethereum/crypto"
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
	//untill all the blocks with preceeding nonces arrive
	TsfPending
	//This transfer should enable all the pending transfer in action pool be accepted
	//into block chain. This happens when a transfer with the missing nouce arrives,
	//filling the gap between minted blocks and pending blocks.
	TsfFinal
)

type simpleTransferTestCfg struct {
	senderPriKey   keypair.PrivateKey
	senderBalance  *big.Int
	recvPriKey     keypair.PrivateKey
	recvBalance    *big.Int
	nonce          uint64
	amount         *big.Int
	payload        []byte
	gasLimit       uint64
	gasPrice       *big.Int
	expectedResult TransferState
	message        string
}

var (
	// In the test case:
	//  - an account with "nil" private key will be created with
	//    keys, address, and initialized with the given balance.
	// - an account with exiting private key will load exiting
	//   balance into test case.
	getSimpleTransferTests = []simpleTransferTestCfg{
		{
			nil, big.NewInt(1000000), //sender private key, balance
			nil, big.NewInt(1000000), //reciver privae key , balance
			1, big.NewInt(100), // nonce, amount
			make([]byte, 100),             //payload
			uint64(200000), big.NewInt(1), // gasLimit, gasPrice
			TsfSuccess, "Normal transfer from an account with enough balance and gas",
		},
		{
			nil, big.NewInt(1000000),
			nil, big.NewInt(1000000),
			1, big.NewInt(100),
			make([]byte, 0),
			uint64(1000), big.NewInt(1),
			TsfFail, "Transfer with not enough gas limit",
		},
		{
			nil, big.NewInt(232222),
			nil, big.NewInt(100000),
			1, big.NewInt(222222),
			make([]byte, 0),
			uint64(200000), big.NewInt(1),
			TsfSuccess, "Transfer with just enough balance",
		},
		{
			nil, big.NewInt(232221),
			nil, big.NewInt(100000),
			1, big.NewInt(222222),
			make([]byte, 0),
			uint64(200000), big.NewInt(1),
			TsfFail, "Transfer with not enough balance",
		},
		{
			nil, big.NewInt(232222),
			nil, big.NewInt(100000),
			1, big.NewInt(222222),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFail, "Transfer with not enough balance with payload",
		},
		{
			nil, big.NewInt(100000),
			nil, big.NewInt(100000),
			1, big.NewInt(0),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfSuccess, "Transfer with 0 amount",
		},
		{
			nil, big.NewInt(100000),
			nil, big.NewInt(100000),
			1, big.NewInt(-100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFail, "Transfer with negtive amount",
		},
		{
			nil, big.NewInt(100000),
			nil, big.NewInt(100000),
			0, big.NewInt(0),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFail, "Transfer with nonce 0",
		},
		{
			identityset.PrivateKey(0), big.NewInt(100000),
			nil, big.NewInt(100000),
			1, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfSuccess, "Transfer with same nouce from a single sender 1",
		},
		{
			identityset.PrivateKey(0), big.NewInt(100000),
			nil, big.NewInt(100000),
			1, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFail, "Transfer with same nouce from a single sender 2",
		},
		{
			identityset.PrivateKey(1), big.NewInt(100000),
			nil, big.NewInt(100000),
			2, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfPending, "Transfer with a sequence of nouce from a single sender 1",
		},
		{
			identityset.PrivateKey(1), big.NewInt(100000),
			nil, big.NewInt(100000),
			3, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfPending, "Transfer with a sequence of nouce from a single sender 2",
		},
		{
			identityset.PrivateKey(1), big.NewInt(100000),
			nil, big.NewInt(100000),
			1, big.NewInt(100),
			make([]byte, 4),
			uint64(200000), big.NewInt(1),
			TsfFinal, "Transfer with a sequence of nouce from a single sender 3",
		},
	}
)

func TestLocalTransfer(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)

	networkPort := 4689
	apiPort := 14014
	sk, err := crypto.GenerateKey()
	require.NoError(err)
	cfg, err := newTransferConfig(testDBPath, testTriePath, sk, networkPort, apiPort)
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

	chainID := cfg.Chain.ID
	require.NotNil(svr.ChainService(chainID).ActionPool())

	err = testutil.WaitUntil(10*time.Millisecond, 10*time.Second, func() (bool, error) {
		ret := true
		resp, err := http.Get("http://localhost:7788/readiness")
		if err != nil || http.StatusOK != resp.StatusCode {
			ret = false
		}

		return ret, nil
	})
	require.NoError(err)

	// target address for grpc connection. Default is "127.0.0.1:14014"
	grpcAddr := "127.0.0.1:14014"
	conn, err := grpc.Dial(grpcAddr, grpc.WithInsecure())
	require.NoError(err)

	client := iotexapi.NewAPIServiceClient(conn)

	defer func() {
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
	}()

	bc := svr.ChainService(chainID).Blockchain()
	ap := svr.ChainService(chainID).ActionPool()

	for _, tsfTest := range getSimpleTransferTests {

		senderPriKey, senderAddr, err := initStateKeyAddr(tsfTest.senderPriKey, tsfTest.senderBalance, bc)
		require.NoError(err, tsfTest.message)

		_, recvAddr, err := initStateKeyAddr(tsfTest.recvPriKey, tsfTest.recvBalance, bc)
		require.NoError(err, tsfTest.message)

		tsf, err := testutil.SignedTransfer(recvAddr, senderPriKey, tsfTest.nonce, tsfTest.amount,
			tsfTest.payload, tsfTest.gasLimit, tsfTest.gasPrice)
		require.NoError(err, tsfTest.message)

		retryNum := 5
		retryInterval := 1
		bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Duration(retryInterval)*time.Second), uint64(retryNum))
		err = backoff.Retry(func() error {
			_, err := client.SendAction(context.Background(), &iotexapi.SendActionRequest{Action: tsf.Proto()})
			return err
		}, bo)
		require.NoError(err, tsfTest.message)

		switch tsfTest.expectedResult {
		case TsfSuccess:
			//Wait long enough for a block to be minted, and check the balance of both
			//sender and receiver.
			time.Sleep(cfg.Consensus.BlockCreationInterval + time.Second)
			selp, err := bc.GetActionByActionHash(tsf.Hash())
			require.NoError(err, tsfTest.message)
			require.Equal(tsfTest.nonce, selp.Proto().GetCore().GetNonce(), tsfTest.message)
			require.Equal(keypair.EncodePublicKey(&senderPriKey.PublicKey),
				hex.EncodeToString(selp.Proto().SenderPubKey), tsfTest.message)

			newSenderBalance, _ := bc.Balance(senderAddr)
			minusAmount := big.NewInt(0).Sub(tsfTest.senderBalance, tsfTest.amount)
			gasUnitPayloadConsumed := big.NewInt(0).Mul(big.NewInt(int64(action.TransferPayloadGas)),
				big.NewInt(int64(len(tsfTest.payload))))
			gasUnitTransferConsumed := big.NewInt(int64(action.TransferBaseIntrinsicGas))
			gasUnitConsumed := big.NewInt(0).Add(gasUnitPayloadConsumed, gasUnitTransferConsumed)
			gasConsumed := big.NewInt(0).Mul(gasUnitConsumed, tsfTest.gasPrice)
			expectedSenderBalance := big.NewInt(0).Sub(minusAmount, gasConsumed)
			require.Equal(newSenderBalance.String(), expectedSenderBalance.String(), tsfTest.message)

			newRecvBalance, _ := bc.Balance(recvAddr)
			expectedRecvrBalance := big.NewInt(0).Add(tsfTest.recvBalance, tsfTest.amount)
			require.Equal(newRecvBalance.String(), expectedRecvrBalance.String(), tsfTest.message)

		case TsfFail:
			//The transfer should be rejected right after we inject it
			time.Sleep(1 * time.Second)
			//In case of failed transfer, the transfer should not exit in either action pool or blockchain
			_, err := ap.GetActionByHash(tsf.Hash())
			require.Error(err, tsfTest.message)
			_, err = bc.GetActionByActionHash(tsf.Hash())
			require.Error(err, tsfTest.message)

		case TsfPending:
			//Need to wait long enough to make sure the pending transfer is not minted, only stay in action pool
			time.Sleep(cfg.Consensus.BlockCreationInterval + time.Second)
			_, err := ap.GetActionByHash(tsf.Hash())
			require.NoError(err, tsfTest.message)
			_, err = bc.GetActionByActionHash(tsf.Hash())
			require.Error(err, tsfTest.message)
		case TsfFinal:
			//After a blocked is minted, check all the pending transfers in action pool are cleared
			//This checking procedue is simplied for this test case, because of the complexity of
			//handling pending transfers.
			time.Sleep(cfg.Consensus.BlockCreationInterval + time.Second)
			acts := ap.PickActs()
			require.Equal(len(acts), 0)

		default:

		}
	}

}

// initStateKeyAddr, if the given privatekey is nil,
// creates key, address, and init the new account with given balance
// otherwise, calulate the the address, and load test with exsiting
// balance state.
func initStateKeyAddr(
	privateKey keypair.PrivateKey,
	initBalance *big.Int,
	bc blockchain.Blockchain,
) (keypair.PrivateKey, string, error) {
	retKey := privateKey
	retAddr := ""
	if retKey == nil {
		sk, err := crypto.GenerateKey()
		if err != nil {
			return nil, "", err
		}
		pk := &sk.PublicKey
		pkHash := keypair.HashPubKey(pk)
		addr, err := address.FromBytes(pkHash[:])
		addrStr := addr.String()
		_, err = bc.CreateState(
			addrStr,
			initBalance,
		)
		if err != nil {
			return nil, "", err
		}
		retKey = sk
		retAddr = addrStr
	} else {
		pk := &retKey.PublicKey
		pkHash := keypair.HashPubKey(pk)
		addr, err := address.FromBytes(pkHash[:])
		retAddr = addr.String()
		existBalance, err := bc.Balance(retAddr)
		if err != nil {
			return nil, "", err
		}
		initBalance.Set(existBalance)
	}
	return retKey, retAddr, nil
}

func newTransferConfig(
	chainDBPath,
	trieDBPath string,
	producerPriKey keypair.PrivateKey,
	networkPort,
	apiPort int,
) (config.Config, error) {

	cfg := config.Default

	cfg.NodeType = config.DelegateType
	cfg.Network.Port = networkPort
	cfg.Chain.ID = 1
	cfg.Chain.ChainDBPath = chainDBPath
	cfg.Chain.TrieDBPath = trieDBPath
	cfg.Chain.EnableIndex = true
	cfg.Chain.EnableAsyncIndexWrite = true
	cfg.Consensus.Scheme = config.StandaloneScheme
	cfg.Consensus.BlockCreationInterval = 5 * time.Second
	cfg.API.Enabled = true
	cfg.API.Port = apiPort

	return cfg, nil
}
