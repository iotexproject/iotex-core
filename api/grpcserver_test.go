// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/testutil"
)

const lld = "lifeLongDelegates"

var (
	testTransfer, _ = action.SignedTransfer(identityset.Address(28).String(),
		identityset.PrivateKey(28), 3, big.NewInt(10), []byte{}, testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64))

	testTransferHash, _ = testTransfer.Hash()
	testTransferPb      = testTransfer.Proto()

	testExecution, _ = action.SignedExecution(identityset.Address(29).String(),
		identityset.PrivateKey(29), 1, big.NewInt(0), testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64), []byte{})

	testExecutionHash, _ = testExecution.Hash()
	testExecutionPb      = testExecution.Proto()

	testTransfer1, _ = action.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(27), 1,
		big.NewInt(10), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	transferHash1, _ = testTransfer1.Hash()
	testTransfer2, _ = action.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(30), 5,
		big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	transferHash2, _ = testTransfer2.Hash()

	testExecution1, _ = action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(30), 6,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	executionHash1, _ = testExecution1.Hash()
	testExecution3, _ = action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 2,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	executionHash3, _ = testExecution3.Hash()

	// invalid nounce
	testTransferInvalid1, _ = action.SignedTransfer(identityset.Address(28).String(),
		identityset.PrivateKey(28), 2, big.NewInt(10), []byte{}, testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64))
	testTransferInvalid1Pb = testTransferInvalid1.Proto()

	// invalid gas price
	testTransferInvalid2, _ = action.SignedTransfer(identityset.Address(28).String(),
		identityset.PrivateKey(28), 3, big.NewInt(10), []byte{}, testutil.TestGasLimit,
		big.NewInt(-1))
	testTransferInvalid2Pb = testTransferInvalid2.Proto()

	// invalid balance
	testTransferInvalid3, _ = action.SignedTransfer(identityset.Address(29).String(),
		identityset.PrivateKey(29), 3, big.NewInt(29), []byte{}, testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64))
	testTransferInvalid3Pb = testTransferInvalid3.Proto()

	// nonce is too high
	testTransferInvalid4, _ = action.SignedTransfer(identityset.Address(28).String(),
		identityset.PrivateKey(28), config.Default.ActPool.MaxNumActsPerAcct+10, big.NewInt(1),
		[]byte{}, uint64(100000), big.NewInt(0))
	testTransferInvalid4Pb = testTransferInvalid4.Proto()

	// replace act with lower gas
	testTransferInvalid5, _ = action.SignedTransfer(identityset.Address(28).String(),
		identityset.PrivateKey(28), 3, big.NewInt(10), []byte{}, 10000,
		big.NewInt(testutil.TestGasPriceInt64))
	testTransferInvalid5Pb = testTransferInvalid5.Proto()

	// gas is too low
	testTransferInvalid6, _ = action.SignedTransfer(identityset.Address(28).String(),
		identityset.PrivateKey(28), 3, big.NewInt(10), []byte{}, 100,
		big.NewInt(testutil.TestGasPriceInt64))
	testTransferInvalid6Pb = testTransferInvalid6.Proto()

	// negative transfer amout
	testTransferInvalid7, _ = action.SignedTransfer(identityset.Address(28).String(),
		identityset.PrivateKey(28), 3, big.NewInt(-10), []byte{}, 10000,
		big.NewInt(testutil.TestGasPriceInt64))
	testTransferInvalid7Pb = testTransferInvalid7.Proto()

	// gas is too large
	largeData               = make([]byte, 1e7)
	testTransferInvalid8, _ = action.SignedTransfer(identityset.Address(28).String(),
		identityset.PrivateKey(28), 3, big.NewInt(10), largeData, 10000,
		big.NewInt(testutil.TestGasPriceInt64))
	testTransferInvalid8Pb = testTransferInvalid8.Proto()

	blkHash      = map[uint64]string{}
	implicitLogs = map[hash.Hash256]*block.TransactionLog{}
)

var (
	delegates = []genesis.Delegate{
		{
			OperatorAddrStr: identityset.Address(0).String(),
			VotesStr:        "10",
		},
		{
			OperatorAddrStr: identityset.Address(1).String(),
			VotesStr:        "10",
		},
		{
			OperatorAddrStr: identityset.Address(2).String(),
			VotesStr:        "10",
		},
	}
)

var (
	getAccountTests = []struct {
		in           string
		address      string
		balance      string
		nonce        uint64
		pendingNonce uint64
		numActions   uint64
	}{
		{identityset.Address(30).String(),
			"io1d4c5lp4ea4754wy439g2t99ue7wryu5r2lslh2",
			"3",
			8,
			9,
			9,
		},
		{
			identityset.Address(27).String(),
			"io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
			"9999999999999999999999898950",
			5,
			6,
			6,
		},
	}

	getActionsTests = []struct {
		start      uint64
		count      uint64
		numActions int
	}{
		{
			1,
			11,
			11,
		},
		{
			11,
			5,
			4,
		},
		{
			1,
			0,
			0,
		},
	}

	getActionTests = []struct {
		// Arguments
		checkPending bool
		in           string
		// Expected Values
		nonce        uint64
		senderPubKey string
		blkNumber    uint64
	}{
		{
			checkPending: false,
			in:           hex.EncodeToString(transferHash1[:]),
			nonce:        1,
			senderPubKey: testTransfer1.SrcPubkey().HexString(),
			blkNumber:    1,
		},
		{
			checkPending: false,
			in:           hex.EncodeToString(transferHash2[:]),
			nonce:        5,
			senderPubKey: testTransfer2.SrcPubkey().HexString(),
			blkNumber:    2,
		},
		{
			checkPending: false,
			in:           hex.EncodeToString(executionHash1[:]),
			nonce:        6,
			senderPubKey: testExecution1.SrcPubkey().HexString(),
			blkNumber:    2,
		},
	}

	getActionsByAddressTests = []struct {
		address    string
		start      uint64
		count      uint64
		numActions int
	}{
		{
			identityset.Address(27).String(),
			0,
			3,
			2,
		},
		{
			identityset.Address(30).String(),
			1,
			8,
			8,
		},
		{
			identityset.Address(33).String(),
			2,
			1,
			0,
		},
	}

	getUnconfirmedActionsByAddressTests = []struct {
		address    string
		start      uint64
		count      uint64
		numActions int
	}{
		{
			identityset.Address(27).String(),
			0,
			4,
			4,
		},
		{
			identityset.Address(27).String(),
			2,
			0,
			0,
		},
	}

	getActionsByBlockTests = []struct {
		blkHeight  uint64
		start      uint64
		count      uint64
		numActions int
	}{
		{
			2,
			0,
			7,
			7,
		},
		{
			4,
			2,
			5,
			3,
		},
		{
			3,
			0,
			0,
			0,
		},
		{
			1,
			0,
			math.MaxUint64,
			2,
		},
	}

	getBlockMetasTests = []struct {
		start, count      uint64
		numBlks           int
		gasLimit, gasUsed uint64
	}{
		{
			1,
			4,
			4,
			20000,
			10000,
		},
		{
			2,
			5,
			3,
			120000,
			60100,
		},
		{
			1,
			0,
			0,
			20000,
			10000,
		},
		// genesis block
		{
			0,
			1,
			1,
			0,
			0,
		},
	}

	getBlockMetaTests = []struct {
		blkHeight      uint64
		numActions     int64
		transferAmount string
		logsBloom      string
	}{
		{
			2,
			7,
			"6",
			"",
		},
		{
			4,
			5,
			"2",
			"",
		},
	}

	getChainMetaTests = []struct {
		// Arguments
		emptyChain       bool
		tpsWindow        int
		pollProtocolType string
		// Expected values
		height     uint64
		numActions int64
		tps        int64
		tpsFloat   float32
		epoch      iotextypes.EpochData
	}{
		{
			emptyChain: true,
		},

		{
			false,
			1,
			lld,
			4,
			15,
			1,
			5 / 10.0,
			iotextypes.EpochData{
				Num:                     1,
				Height:                  1,
				GravityChainStartHeight: 1,
			},
		},
		{
			false,
			5,
			"governanceChainCommittee",
			4,
			15,
			2,
			15 / 13.0,
			iotextypes.EpochData{
				Num:                     1,
				Height:                  1,
				GravityChainStartHeight: 100,
			},
		},
	}

	sendActionTests = []struct {
		// Arguments
		actionPb *iotextypes.Action
		// Expected Values
		actionHash string
	}{
		{
			testTransferPb,
			hex.EncodeToString(testTransferHash[:]),
		},
		{
			testExecutionPb,
			hex.EncodeToString(testExecutionHash[:]),
		},
	}

	getReceiptByActionTests = []struct {
		in        string
		status    uint64
		blkHeight uint64
	}{
		{
			hex.EncodeToString(transferHash1[:]),
			uint64(iotextypes.ReceiptStatus_Success),
			1,
		},
		{
			hex.EncodeToString(transferHash2[:]),
			uint64(iotextypes.ReceiptStatus_Success),
			2,
		},
		{
			hex.EncodeToString(executionHash1[:]),
			uint64(iotextypes.ReceiptStatus_Success),
			2,
		},
		{
			hex.EncodeToString(executionHash3[:]),
			uint64(iotextypes.ReceiptStatus_Success),
			4,
		},
	}

	readContractTests = []struct {
		execHash    string
		callerAddr  string
		actionHash  string
		retValue    string
		gasConsumed uint64
	}{
		{
			hex.EncodeToString(executionHash1[:]),
			"",
			"08b0066e10b5607e47159c2cf7ba36e36d0c980f5108dfca0ec20547a7adace4",
			"",
			10100,
		},
	}

	suggestGasPriceTests = []struct {
		defaultGasPrice   uint64
		suggestedGasPrice uint64
	}{
		{
			1,
			1,
		},
	}

	estimateGasForActionTests = []struct {
		actionHash   string
		estimatedGas uint64
	}{
		{
			hex.EncodeToString(transferHash1[:]),
			10000,
		},
		{
			hex.EncodeToString(transferHash2[:]),
			10000,
		},
	}

	readUnclaimedBalanceTests = []struct {
		// Arguments
		protocolID string
		methodName string
		addr       string
		// Expected values
		returnErr bool
		balance   *big.Int
	}{
		{
			protocolID: "rewarding",
			methodName: "UnclaimedBalance",
			addr:       identityset.Address(0).String(),
			returnErr:  false,
			balance:    unit.ConvertIotxToRau(64), // 4 block * 36 IOTX reward by default = 144 IOTX
		},
		{
			protocolID: "rewarding",
			methodName: "UnclaimedBalance",
			addr:       identityset.Address(1).String(),
			returnErr:  false,
			balance:    unit.ConvertIotxToRau(0), // 4 block * 36 IOTX reward by default = 144 IOTX
		},
		{
			protocolID: "Wrong ID",
			methodName: "UnclaimedBalance",
			addr:       identityset.Address(27).String(),
			returnErr:  true,
		},
		{
			protocolID: "rewarding",
			methodName: "Wrong Method",
			addr:       identityset.Address(27).String(),
			returnErr:  true,
		},
	}

	readCandidatesByEpochTests = []struct {
		// Arguments
		protocolID   string
		protocolType string
		methodName   string
		epoch        uint64
		// Expected Values
		numDelegates int
	}{
		{
			protocolID:   "poll",
			protocolType: lld,
			methodName:   "CandidatesByEpoch",
			epoch:        1,
			numDelegates: 3,
		},
		{
			protocolID:   "poll",
			protocolType: "governanceChainCommittee",
			methodName:   "CandidatesByEpoch",
			epoch:        1,
			numDelegates: 2,
		},
	}

	readBlockProducersByEpochTests = []struct {
		// Arguments
		protocolID            string
		protocolType          string
		methodName            string
		epoch                 uint64
		numCandidateDelegates uint64
		// Expected Values
		numBlockProducers int
	}{
		{
			protocolID:        "poll",
			protocolType:      lld,
			methodName:        "BlockProducersByEpoch",
			epoch:             1,
			numBlockProducers: 3,
		},
		{
			protocolID:            "poll",
			protocolType:          "governanceChainCommittee",
			methodName:            "BlockProducersByEpoch",
			epoch:                 1,
			numCandidateDelegates: 2,
			numBlockProducers:     2,
		},
		{
			protocolID:            "poll",
			protocolType:          "governanceChainCommittee",
			methodName:            "BlockProducersByEpoch",
			epoch:                 1,
			numCandidateDelegates: 1,
			numBlockProducers:     1,
		},
	}

	readActiveBlockProducersByEpochTests = []struct {
		// Arguments
		protocolID   string
		protocolType string
		methodName   string
		epoch        uint64
		numDelegates uint64
		// Expected Values
		numActiveBlockProducers int
	}{
		{
			protocolID:              "poll",
			protocolType:            lld,
			methodName:              "ActiveBlockProducersByEpoch",
			epoch:                   1,
			numActiveBlockProducers: 3,
		},
		{
			protocolID:              "poll",
			protocolType:            "governanceChainCommittee",
			methodName:              "ActiveBlockProducersByEpoch",
			epoch:                   1,
			numDelegates:            2,
			numActiveBlockProducers: 2,
		},
		{
			protocolID:              "poll",
			protocolType:            "governanceChainCommittee",
			methodName:              "ActiveBlockProducersByEpoch",
			epoch:                   1,
			numDelegates:            1,
			numActiveBlockProducers: 1,
		},
	}

	readRollDPoSMetaTests = []struct {
		// Arguments
		protocolID string
		methodName string
		height     uint64
		// Expected Values
		result uint64
	}{
		{
			protocolID: "rolldpos",
			methodName: "NumCandidateDelegates",
			result:     36,
		},
		{
			protocolID: "rolldpos",
			methodName: "NumDelegates",
			result:     24,
		},
	}

	readEpochCtxTests = []struct {
		// Arguments
		protocolID string
		methodName string
		argument   uint64
		// Expected Values
		result uint64
	}{
		{
			protocolID: "rolldpos",
			methodName: "NumSubEpochs",
			argument:   1,
			result:     2,
		},
		{
			protocolID: "rolldpos",
			methodName: "NumSubEpochs",
			argument:   1816201,
			result:     30,
		},
		{
			protocolID: "rolldpos",
			methodName: "EpochNumber",
			argument:   100,
			result:     3,
		},
		{
			protocolID: "rolldpos",
			methodName: "EpochHeight",
			argument:   5,
			result:     193,
		},
		{
			protocolID: "rolldpos",
			methodName: "EpochLastHeight",
			argument:   1000,
			result:     48000,
		},
		{
			protocolID: "rolldpos",
			methodName: "SubEpochNumber",
			argument:   121,
			result:     1,
		},
	}

	getEpochMetaTests = []struct {
		// Arguments
		EpochNumber      uint64
		pollProtocolType string
		// Expected Values
		epochData                     iotextypes.EpochData
		numBlksInEpoch                int
		numConsenusBlockProducers     int
		numActiveCensusBlockProducers int
	}{
		{
			1,
			lld,
			iotextypes.EpochData{
				Num:                     1,
				Height:                  1,
				GravityChainStartHeight: 1,
			},
			4,
			24,
			24,
		},
		{
			1,
			"governanceChainCommittee",
			iotextypes.EpochData{
				Num:                     1,
				Height:                  1,
				GravityChainStartHeight: 100,
			},
			4,
			6,
			6,
		},
	}

	getRawBlocksTest = []struct {
		// Arguments
		startHeight  uint64
		count        uint64
		withReceipts bool
		// Expected Values
		numBlks     int
		numActions  int
		numReceipts int
	}{
		{
			1,
			1,
			false,
			1,
			2,
			0,
		},
		{
			1,
			2,
			true,
			2,
			9,
			9,
		},
		// genesis block
		{
			0,
			1,
			true,
			1,
			0,
			0,
		},
	}

	getLogsByRangeTest = []struct {
		// Arguments
		address   []string
		topics    []*iotexapi.Topics
		fromBlock uint64
		count     uint64
		// Expected Values
		numLogs int
	}{
		{
			address:   []string{},
			topics:    []*iotexapi.Topics{},
			fromBlock: 1,
			count:     100,
			numLogs:   4,
		},
		{
			address:   []string{},
			topics:    []*iotexapi.Topics{},
			fromBlock: 1,
			count:     100,
			numLogs:   4,
		},
	}

	getImplicitLogByBlockHeightTest = []struct {
		height uint64
		code   codes.Code
	}{
		{
			1, codes.OK,
		},
		{
			2, codes.OK,
		},
		{
			3, codes.OK,
		},
		{
			4, codes.OK,
		},
		{
			5, codes.InvalidArgument,
		},
	}

	getActionByActionHashTest = []struct {
		h              hash.Hash256
		expectedNounce uint64
	}{
		{
			transferHash1,
			1,
		},
		{
			transferHash2,
			5,
		},
		{
			executionHash1,
			6,
		},
		{
			executionHash3,
			2,
		},
	}
)

func TestGrpcServer_GetAccount(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, true)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	// deploy a contract
	contractCode := "6080604052348015600f57600080fd5b5060de8061001e6000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c8063ee82ac5e14602d575b600080fd5b605660048036036020811015604157600080fd5b8101908080359060200190929190505050606c565b6040518082815260200191505060405180910390f35b60008082409050807f2d93f7749862d33969fb261757410b48065a1bc86a56da5c47820bd063e2338260405160405180910390a28091505091905056fea265627a7a723158200a258cd08ea99ee11aa68c78b6d2bf7ea912615a1e64a81b90a2abca2dd59cfa64736f6c634300050c0032"
	contract, err := deployContractV2(svr, identityset.PrivateKey(13), 1, svr.core.bc.TipHeight(), contractCode)
	require.NoError(err)
	require.True(len(contract) > 0)

	// read contract address
	request := &iotexapi.GetAccountRequest{Address: contract}
	res, err := svr.GrpcServer.GetAccount(context.Background(), request)
	require.NoError(err)
	accountMeta := res.AccountMeta
	require.Equal(contract, accountMeta.Address)
	require.Equal("0", accountMeta.Balance)
	require.EqualValues(0, accountMeta.Nonce)
	require.EqualValues(1, accountMeta.PendingNonce)
	require.EqualValues(0, accountMeta.NumActions)
	require.True(accountMeta.IsContract)
	require.True(len(accountMeta.ContractByteCode) > 0)
	require.Contains(contractCode, hex.EncodeToString(accountMeta.ContractByteCode))

	// success
	for _, test := range getAccountTests {
		request := &iotexapi.GetAccountRequest{Address: test.in}
		res, err := svr.GrpcServer.GetAccount(context.Background(), request)
		require.NoError(err)
		accountMeta := res.AccountMeta
		require.Equal(test.address, accountMeta.Address)
		require.Equal(test.balance, accountMeta.Balance)
		require.Equal(test.nonce, accountMeta.Nonce)
		require.Equal(test.pendingNonce, accountMeta.PendingNonce)
		require.Equal(test.numActions, accountMeta.NumActions)
		require.EqualValues(5, res.BlockIdentifier.Height)
		require.NotZero(res.BlockIdentifier.Hash)
	}
	// failure
	_, err = svr.GrpcServer.GetAccount(context.Background(), &iotexapi.GetAccountRequest{})
	require.Error(err)
	// error account
	_, err = svr.GrpcServer.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: "io3fn88lge6hyzmruh40cn6l3e49dfkqzqk3lgtq3"})
	require.Error(err)

	// success: reward pool
	res, err = svr.GrpcServer.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: address.RewardingPoolAddr})
	require.NoError(err)
	require.Equal(address.RewardingPoolAddr, res.AccountMeta.Address)
	require.Equal("200000000000000000000101000", res.AccountMeta.Balance)
	require.EqualValues(5, res.BlockIdentifier.Height)
	require.NotZero(res.BlockIdentifier.Hash)

	//failure: protocol staking isn't registered
	res, err = svr.GrpcServer.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: address.StakingBucketPoolAddr})
	require.Contains(err.Error(), "protocol staking isn't registered")
}

func TestGrpcServer_GetActions(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range getActionsTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByIndex{
				ByIndex: &iotexapi.GetActionsByIndexRequest{
					Start: test.start,
					Count: test.count,
				},
			},
		}

		res, err := svr.GrpcServer.GetActions(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
		} else {
			require.NoError(err)
			require.Equal(test.numActions, len(res.ActionInfo))
		}

		svrDisableIndex := svr
		svrDisableIndex.core.hasActionIndex = false
		res, err = svrDisableIndex.GrpcServer.GetActions(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
		} else {
			require.NoError(err)
			require.Equal(test.numActions, len(res.ActionInfo))
		}

	}

	// failure: empty request
	_, err = svr.GrpcServer.GetActions(context.Background(), &iotexapi.GetActionsRequest{})
	require.Error(err)

	// failure: range exceed limit
	_, err = svr.GrpcServer.GetActions(context.Background(),
		&iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByIndex{
				ByIndex: &iotexapi.GetActionsByIndexRequest{
					Start: 1,
					Count: 100000,
				},
			},
		})
	require.Error(err)

	// failure: start exceed limit
	_, err = svr.GrpcServer.GetActions(context.Background(),
		&iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByIndex{
				ByIndex: &iotexapi.GetActionsByIndexRequest{
					Start: 100000,
					Count: 1,
				},
			},
		})
	require.Error(err)
}

func TestGrpcServer_GetAction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, true)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range getActionTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByHash{
				ByHash: &iotexapi.GetActionByHashRequest{
					ActionHash:   test.in,
					CheckPending: test.checkPending,
				},
			},
		}
		res, err := svr.GrpcServer.GetActions(context.Background(), request)
		require.NoError(err)
		require.Equal(1, len(res.ActionInfo))
		act := res.ActionInfo[0]
		require.Equal(test.nonce, act.Action.GetCore().GetNonce())
		require.Equal(test.senderPubKey, hex.EncodeToString(act.Action.SenderPubKey))
		if !test.checkPending {
			blk, err := svr.core.dao.GetBlockByHeight(test.blkNumber)
			require.NoError(err)
			timeStamp := blk.ConvertToBlockHeaderPb().GetCore().GetTimestamp()
			blkHash := blk.HashBlock()
			require.Equal(hex.EncodeToString(blkHash[:]), act.BlkHash)
			require.Equal(test.blkNumber, act.BlkHeight)
			require.Equal(timeStamp, act.Timestamp)
		} else {
			require.Equal(hex.EncodeToString(hash.ZeroHash256[:]), act.BlkHash)
			require.Nil(act.Timestamp)
			require.Equal(uint64(0), act.BlkHeight)
		}
	}

	// failure: invalid hash
	_, err = svr.GrpcServer.GetActions(context.Background(),
		&iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByHash{
				ByHash: &iotexapi.GetActionByHashRequest{
					ActionHash:   "0x58df1e9cb0572fea48e8ce9d9b787ae557c304657d01890f4fc5ea88a1f44c3e",
					CheckPending: true,
				},
			},
		})
	require.Error(err)
}

func TestGrpcServer_GetActionsByAddress(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range getActionsByAddressTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByAddr{
				ByAddr: &iotexapi.GetActionsByAddressRequest{
					Address: test.address,
					Start:   test.start,
					Count:   test.count,
				},
			},
		}
		res, err := svr.GrpcServer.GetActions(context.Background(), request)
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
		if test.numActions == 0 {
			// returns empty response body in case of no result
			require.Equal(&iotexapi.GetActionsResponse{}, res)
		}
		var prevAct *iotexapi.ActionInfo
		for _, act := range res.ActionInfo {
			if prevAct != nil {
				require.True(act.Timestamp.GetSeconds() >= prevAct.Timestamp.GetSeconds())
			}
			prevAct = act
		}
		if test.start > 0 && len(res.ActionInfo) > 0 {
			request = &iotexapi.GetActionsRequest{
				Lookup: &iotexapi.GetActionsRequest_ByAddr{
					ByAddr: &iotexapi.GetActionsByAddressRequest{
						Address: test.address,
						Start:   0,
						Count:   test.start,
					},
				},
			}
			prevRes, err := svr.GrpcServer.GetActions(context.Background(), request)
			require.NoError(err)
			require.True(prevRes.ActionInfo[len(prevRes.ActionInfo)-1].Timestamp.GetSeconds() <= res.ActionInfo[0].Timestamp.GetSeconds())
		}
	}
}

func TestGrpcServer_GetUnconfirmedActionsByAddress(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, true)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range getUnconfirmedActionsByAddressTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_UnconfirmedByAddr{
				UnconfirmedByAddr: &iotexapi.GetUnconfirmedActionsByAddressRequest{
					Address: test.address,
					Start:   test.start,
					Count:   test.count,
				},
			},
		}
		res, err := svr.GrpcServer.GetActions(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
			continue
		}
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
		require.Equal(test.address, res.ActionInfo[0].Sender)
	}
}

func TestGrpcServer_GetActionsByBlock(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range getActionsByBlockTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByBlk{
				ByBlk: &iotexapi.GetActionsByBlockRequest{
					BlkHash: blkHash[test.blkHeight],
					Start:   test.start,
					Count:   test.count,
				},
			},
		}
		res, err := svr.GrpcServer.GetActions(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
			continue
		}
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
		for _, v := range res.ActionInfo {
			require.Equal(test.blkHeight, v.BlkHeight)
			require.Equal(blkHash[test.blkHeight], v.BlkHash)
		}
	}
}

func TestGrpcServer_GetBlockMetas(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	genesis.SetGenesisTimestamp(cfg.Genesis.Timestamp)
	block.LoadGenesisHash(&cfg.Genesis)
	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range getBlockMetasTests {
		request := &iotexapi.GetBlockMetasRequest{
			Lookup: &iotexapi.GetBlockMetasRequest_ByIndex{
				ByIndex: &iotexapi.GetBlockMetasByIndexRequest{
					Start: test.start,
					Count: test.count,
				},
			},
		}
		res, err := svr.GrpcServer.GetBlockMetas(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
			continue
		}
		require.NoError(err)
		require.Equal(test.numBlks, len(res.BlkMetas))
		meta := res.BlkMetas[0]
		require.Equal(test.gasLimit, meta.GasLimit)
		require.Equal(test.gasUsed, meta.GasUsed)
		if test.start == 0 {
			// genesis block
			h := block.GenesisHash()
			require.Equal(meta.Hash, hex.EncodeToString(h[:]))
		}
		var prevBlkPb *iotextypes.BlockMeta
		for _, blkPb := range res.BlkMetas {
			if prevBlkPb != nil {
				require.True(blkPb.Height > prevBlkPb.Height)
			}
			prevBlkPb = blkPb
		}
	}
	// failure: empty request
	_, err = svr.GrpcServer.GetBlockMetas(context.Background(), &iotexapi.GetBlockMetasRequest{})
	require.Error(err)

	_, err = svr.GrpcServer.GetBlockMetas(context.Background(), &iotexapi.GetBlockMetasRequest{
		Lookup: &iotexapi.GetBlockMetasRequest_ByIndex{
			ByIndex: &iotexapi.GetBlockMetasByIndexRequest{Start: 10, Count: 1},
		},
	})
	require.Error(err)

	_, err = svr.GrpcServer.GetBlockMetas(context.Background(), &iotexapi.GetBlockMetasRequest{
		Lookup: &iotexapi.GetBlockMetasRequest_ByHash{
			ByHash: &iotexapi.GetBlockMetaByHashRequest{BlkHash: "0xa2e8e0c9cafbe93f2b7f7c9d32534bc6fde95f2185e5f2aaa6bf7ebdf1a6610a"},
		},
	})
	require.Error(err)
}

func TestGrpcServer_GetBlockMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range getBlockMetaTests {
		header, err := svr.core.bc.BlockHeaderByHeight(test.blkHeight)
		require.NoError(err)
		blkHash := header.HashBlock()
		request := &iotexapi.GetBlockMetasRequest{
			Lookup: &iotexapi.GetBlockMetasRequest_ByHash{
				ByHash: &iotexapi.GetBlockMetaByHashRequest{
					BlkHash: hex.EncodeToString(blkHash[:]),
				},
			},
		}
		res, err := svr.GrpcServer.GetBlockMetas(context.Background(), request)
		require.NoError(err)
		require.Equal(1, len(res.BlkMetas))
		blkPb := res.BlkMetas[0]
		require.Equal(test.blkHeight, blkPb.Height)
		require.Equal(test.numActions, blkPb.NumActions)
		require.Equal(test.transferAmount, blkPb.TransferAmount)
		require.Equal(header.LogsBloomfilter(), nil)
		require.Equal(test.logsBloom, blkPb.LogsBloom)
	}
}

func TestGrpcServer_GetChainMeta(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)

	var pol poll.Protocol
	for _, test := range getChainMetaTests {
		cfg := newConfig(t)
		if test.pollProtocolType == lld {
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else if test.pollProtocolType == "governanceChainCommittee" {
			committee := mock_committee.NewMockCommittee(ctrl)
			slasher, _ := poll.NewSlasher(
				func(uint64, uint64) (map[string]uint64, error) {
					return nil, nil
				},
				nil,
				nil,
				nil,
				nil,
				cfg.Genesis.NumCandidateDelegates,
				cfg.Genesis.NumDelegates,
				cfg.Genesis.DardanellesNumSubEpochs,
				cfg.Genesis.ProductivityThreshold,
				cfg.Genesis.ProbationEpochPeriod,
				cfg.Genesis.UnproductiveDelegateMaxCacheSize,
				cfg.Genesis.ProbationIntensityRate)
			pol, _ = poll.NewGovernanceChainCommitteeProtocol(
				nil,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				cfg.Chain.PollInitialCandidatesInterval,
				slasher)
			committee.EXPECT().HeightByTime(gomock.Any()).Return(test.epoch.GravityChainStartHeight, nil)
		}

		cfg.API.TpsWindow = test.tpsWindow
		svr, bfIndexFile, err := createServerV2(cfg, false)
		require.NoError(err)
		defer func() {
			testutil.CleanupPath(t, bfIndexFile)
		}()
		if pol != nil {
			require.NoError(pol.ForceRegister(svr.core.registry))
		}
		if test.emptyChain {
			mbc := mock_blockchain.NewMockBlockchain(ctrl)
			mbc.EXPECT().TipHeight().Return(uint64(0)).Times(1)
			mbc.EXPECT().ChainID().Return(uint32(1)).Times(1)
			svr.core.bc = mbc
		}
		res, err := svr.GrpcServer.GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{})
		require.NoError(err)
		chainMetaPb := res.ChainMeta
		require.Equal(test.height, chainMetaPb.Height)
		require.Equal(test.numActions, chainMetaPb.NumActions)
		require.Equal(test.tps, chainMetaPb.Tps)
		require.Equal(test.epoch.Num, chainMetaPb.Epoch.Num)
		require.Equal(test.epoch.Height, chainMetaPb.Epoch.Height)
		require.Equal(test.epoch.GravityChainStartHeight, chainMetaPb.Epoch.GravityChainStartHeight)
	}
}

func TestGrpcServer_SendAction(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)

	chain := mock_blockchain.NewMockBlockchain(ctrl)
	ap := mock_actpool.NewMockActPool(ctrl)
	broadcastHandlerCount := 0
	core := &coreService{
		bc: chain,
		ap: ap,
		broadcastHandler: func(_ context.Context, _ uint32, _ proto.Message) error {
			broadcastHandlerCount++
			return nil
		},
	}
	core.cfg.Genesis.ToBeEnabledBlockHeight = 10
	svr := NewGRPCServer(core, 141014)
	chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	chain.EXPECT().TipHeight().Return(uint64(4)).Times(2)
	ap.EXPECT().Add(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	for i, test := range sendActionTests {
		request := &iotexapi.SendActionRequest{Action: test.actionPb}
		res, err := svr.SendAction(context.Background(), request)
		require.NoError(err)
		require.Equal(i+1, broadcastHandlerCount)
		require.Equal(test.actionHash, res.ActionHash)
	}

	// 3 failure cases
	ctx := context.Background()
	tests := []struct {
		server func() (*ServerV2, string, error)
		action *iotextypes.Action
		err    string
	}{
		{
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			&iotextypes.Action{},
			"invalid signature length",
		},
		{
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			&iotextypes.Action{
				Signature: action.ValidSig,
			},
			"empty action proto to load",
		},
		{
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				cfg.ActPool.MaxNumActsPerPool = 8
				return createServerV2(cfg, true)
			},
			testTransferPb,
			action.ErrTxPoolOverflow.Error(),
		},
		{
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			testTransferInvalid1Pb,
			action.ErrNonceTooLow.Error(),
		},
		{
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			testTransferInvalid2Pb,
			action.ErrUnderpriced.Error(),
		},
		{
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			testTransferInvalid3Pb,
			action.ErrInsufficientFunds.Error(),
		},
	}

	for _, test := range tests {
		request := &iotexapi.SendActionRequest{Action: test.action}
		svr, file, err := test.server()
		require.NoError(err)
		defer func() {
			testutil.CleanupPath(t, file)
		}()

		_, err = svr.GrpcServer.SendAction(ctx, request)
		require.Contains(err.Error(), test.err)
	}
}

func TestGrpcServer_StreamLogs(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, true)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	err = svr.GrpcServer.StreamLogs(&iotexapi.StreamLogsRequest{}, nil)
	require.Error(err)
}

func TestGrpcServer_GetReceiptByAction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range getReceiptByActionTests {
		request := &iotexapi.GetReceiptByActionRequest{ActionHash: test.in}
		res, err := svr.GrpcServer.GetReceiptByAction(context.Background(), request)
		require.NoError(err)
		receiptPb := res.ReceiptInfo.Receipt
		require.Equal(test.status, receiptPb.Status)
		require.Equal(test.blkHeight, receiptPb.BlkHeight)
		require.NotEqual(hash.ZeroHash256, res.ReceiptInfo.BlkHash)
	}

	// failure: empty request
	_, err = svr.GrpcServer.GetReceiptByAction(context.Background(), &iotexapi.GetReceiptByActionRequest{ActionHash: "0x"})
	require.Error(err)
	// failure: wrong hash
	_, err = svr.GrpcServer.GetReceiptByAction(context.Background(), &iotexapi.GetReceiptByActionRequest{ActionHash: "b7faffcb8b01fa9f32112155bcb93d714f599eab3178e577e88dafd2140bfc5a"})
	require.Error(err)

}

func TestGrpcServer_GetServerMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	resProto, err := svr.GrpcServer.GetServerMeta(context.Background(), &iotexapi.GetServerMetaRequest{})
	res := resProto.GetServerMeta()
	require.Equal(res.BuildTime, version.BuildTime)
	require.Equal(res.GoVersion, version.GoVersion)
	require.Equal(res.GitStatus, version.GitStatus)
	require.Equal(res.PackageCommitID, version.PackageCommitID)
	require.Equal(res.PackageVersion, version.PackageVersion)
}

func TestGrpcServer_ReadContract(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range readContractTests {
		hash, err := hash.HexStringToHash256(test.execHash)
		require.NoError(err)
		ai, err := svr.core.indexer.GetActionIndex(hash[:])
		require.NoError(err)
		exec, _, err := svr.core.dao.GetActionByActionHash(hash, ai.BlockHeight())
		require.NoError(err)
		request := &iotexapi.ReadContractRequest{
			Execution:     exec.Proto().GetCore().GetExecution(),
			CallerAddress: test.callerAddr,
			GasLimit:      exec.GasLimit(),
			GasPrice:      big.NewInt(unit.Qev).String(),
		}

		res, err := svr.GrpcServer.ReadContract(context.Background(), request)
		require.NoError(err)
		require.Equal(test.retValue, res.Data)
		require.EqualValues(1, res.Receipt.Status)
		require.Equal(test.actionHash, hex.EncodeToString(res.Receipt.ActHash))
		require.Equal(test.gasConsumed, res.Receipt.GasConsumed)
	}
}

func TestGrpcServer_SuggestGasPrice(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	for _, test := range suggestGasPriceTests {
		cfg.API.GasStation.DefaultGas = test.defaultGasPrice
		svr, bfIndexFile, err := createServerV2(cfg, false)
		require.NoError(err)
		defer func() {
			testutil.CleanupPath(t, bfIndexFile)
		}()
		res, err := svr.GrpcServer.SuggestGasPrice(context.Background(), &iotexapi.SuggestGasPriceRequest{})
		require.NoError(err)
		require.Equal(test.suggestedGasPrice, res.GasPrice)
	}
}

func TestGrpcServer_EstimateGasForAction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range estimateGasForActionTests {
		hash, err := hash.HexStringToHash256(test.actionHash)
		require.NoError(err)
		ai, err := svr.core.indexer.GetActionIndex(hash[:])
		require.NoError(err)
		act, _, err := svr.core.dao.GetActionByActionHash(hash, ai.BlockHeight())
		require.NoError(err)
		request := &iotexapi.EstimateGasForActionRequest{Action: act.Proto()}

		res, err := svr.GrpcServer.EstimateGasForAction(context.Background(), request)
		require.NoError(err)
		require.Equal(test.estimatedGas, res.Gas)
	}
}

func TestGrpcServer_EstimateActionGasConsumption(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	// test for contract deploy
	data := "608060405234801561001057600080fd5b50610123600102600281600019169055503373ffffffffffffffffffffffffffffffffffffffff166001026003816000191690555060035460025417600481600019169055506102ae806100656000396000f300608060405260043610610078576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680630cc0e1fb1461007d57806328f371aa146100b05780636b1d752b146100df578063d4b8399214610112578063daea85c514610145578063eb6fd96a14610188575b600080fd5b34801561008957600080fd5b506100926101bb565b60405180826000191660001916815260200191505060405180910390f35b3480156100bc57600080fd5b506100c56101c1565b604051808215151515815260200191505060405180910390f35b3480156100eb57600080fd5b506100f46101d7565b60405180826000191660001916815260200191505060405180910390f35b34801561011e57600080fd5b506101276101dd565b60405180826000191660001916815260200191505060405180910390f35b34801561015157600080fd5b50610186600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506101e3565b005b34801561019457600080fd5b5061019d61027c565b60405180826000191660001916815260200191505060405180910390f35b60035481565b6000600454600019166001546000191614905090565b60025481565b60045481565b3373ffffffffffffffffffffffffffffffffffffffff166001028173ffffffffffffffffffffffffffffffffffffffff16600102176001816000191690555060016000808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81548160ff02191690831515021790555050565b600154815600a165627a7a7230582089b5f99476d642b66a213c12cd198207b2e813bb1caf3bd75e22be535ebf5d130029"
	byteCodes, err := hex.DecodeString(data)
	require.NoError(err)
	execution, err := action.NewExecution("", 1, big.NewInt(0), 0, big.NewInt(0), byteCodes)
	require.NoError(err)
	request := &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_Execution{
			Execution: execution.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err := svr.GrpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(286579), res.Gas)

	// test for transfer
	tran, err := action.NewTransfer(0, big.NewInt(0), "", []byte("123"), 0, big.NewInt(0))
	require.NoError(err)
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_Transfer{
			Transfer: tran.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err = svr.GrpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10300), res.Gas)

	var (
		gaslimit   = uint64(1000000)
		gasprice   = big.NewInt(10)
		canAddress = "io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza"
		payload    = []byte("123")
		amount     = big.NewInt(10)
		nonce      = uint64(0)
		duration   = uint32(1000)
		autoStake  = true
		index      = uint64(10)
	)

	// staking related
	// case I: test for StakeCreate
	cs, err := action.NewCreateStake(nonce, canAddress, amount.String(), duration, autoStake, payload, gaslimit, gasprice)
	require.NoError(err)
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_StakeCreate{
			StakeCreate: cs.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err = svr.GrpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10300), res.Gas)

	// case II: test for StakeUnstake
	us, err := action.NewUnstake(nonce, index, payload, gaslimit, gasprice)
	require.NoError(err)
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_StakeUnstake{
			StakeUnstake: us.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err = svr.GrpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10300), res.Gas)

	// case III: test for StakeWithdraw
	ws, err := action.NewWithdrawStake(nonce, index, payload, gaslimit, gasprice)
	require.NoError(err)
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_StakeWithdraw{
			StakeWithdraw: ws.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err = svr.GrpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10300), res.Gas)

	// Case IV: test for StakeDeposit
	ds, err := action.NewDepositToStake(nonce, 1, amount.String(), payload, gaslimit, gasprice)
	require.NoError(err)
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_StakeAddDeposit{
			StakeAddDeposit: ds.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err = svr.GrpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10300), res.Gas)

	// Case V: test for StakeChangeCandidate
	cc, err := action.NewChangeCandidate(nonce, canAddress, index, payload, gaslimit, gasprice)
	require.NoError(err)
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_StakeChangeCandidate{
			StakeChangeCandidate: cc.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err = svr.GrpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10300), res.Gas)

	// Case VI: test for StakeRestake
	rs, err := action.NewRestake(nonce, index, duration, autoStake, payload, gaslimit, gasprice)
	require.NoError(err)
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_StakeRestake{
			StakeRestake: rs.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err = svr.GrpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10300), res.Gas)

	// Case VII: test for StakeTransfer
	ts, err := action.NewTransferStake(nonce, canAddress, index, payload, gaslimit, gasprice)
	require.NoError(err)
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_StakeTransferOwnership{
			StakeTransferOwnership: ts.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err = svr.GrpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10300), res.Gas)

	// Case VIII: test for CandidateRegister
	cr, err := action.NewCandidateRegister(nonce, canAddress, canAddress, canAddress, canAddress, amount.String(), duration, autoStake, payload, gaslimit, gasprice)
	require.NoError(err)
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_CandidateRegister{
			CandidateRegister: cr.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err = svr.GrpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10300), res.Gas)

	// Case IX: test for CandidateUpdate
	cu, err := action.NewCandidateUpdate(nonce, canAddress, canAddress, canAddress, gaslimit, gasprice)
	require.NoError(err)
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_CandidateUpdate{
			CandidateUpdate: cu.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err = svr.GrpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10000), res.Gas)

	// Case X: test for action nil
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action:        nil,
		CallerAddress: identityset.Address(0).String(),
	}
	_, err = svr.GrpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.Error(err)
}

func TestGrpcServer_ReadUnclaimedBalance(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	cfg.Consensus.Scheme = config.RollDPoSScheme
	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range readUnclaimedBalanceTests {
		out, err := svr.GrpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(test.addr)},
		})
		if test.returnErr {
			require.Error(err)
			continue
		}
		require.NoError(err)
		val, ok := big.NewInt(0).SetString(string(out.Data), 10)
		require.True(ok)
		require.Equal(test.balance, val)
	}
}

func TestGrpcServer_TotalBalance(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	out, err := svr.GrpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte("TotalBalance"),
		Arguments:  nil,
	})
	require.NoError(err)
	val, ok := big.NewInt(0).SetString(string(out.Data), 10)
	require.True(ok)
	require.Equal(unit.ConvertIotxToRau(200000000), val)
}

func TestGrpcServer_AvailableBalance(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	cfg.Consensus.Scheme = config.RollDPoSScheme
	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	out, err := svr.GrpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte("AvailableBalance"),
		Arguments:  nil,
	})
	require.NoError(err)
	val, ok := big.NewInt(0).SetString(string(out.Data), 10)
	require.True(ok)
	require.Equal(unit.ConvertIotxToRau(199999936), val)
}

func TestGrpcServer_ReadCandidatesByEpoch(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)
	committee := mock_committee.NewMockCommittee(ctrl)
	candidates := []*state.Candidate{
		{
			Address:       "address1",
			Votes:         big.NewInt(1),
			RewardAddress: "rewardAddress",
		},
		{
			Address:       "address2",
			Votes:         big.NewInt(1),
			RewardAddress: "rewardAddress",
		},
	}

	for _, test := range readCandidatesByEpochTests {
		var pol poll.Protocol
		if test.protocolType == lld {
			cfg.Genesis.Delegates = delegates
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else {
			indexer, err := poll.NewCandidateIndexer(db.NewMemKVStore())
			require.NoError(err)
			slasher, _ := poll.NewSlasher(
				func(uint64, uint64) (map[string]uint64, error) {
					return nil, nil
				},
				func(protocol.StateReader, uint64, bool, bool) ([]*state.Candidate, uint64, error) {
					return candidates, 0, nil
				},
				nil,
				nil,
				indexer,
				cfg.Genesis.NumCandidateDelegates,
				cfg.Genesis.NumDelegates,
				cfg.Genesis.DardanellesNumSubEpochs,
				cfg.Genesis.ProductivityThreshold,
				cfg.Genesis.ProbationEpochPeriod,
				cfg.Genesis.UnproductiveDelegateMaxCacheSize,
				cfg.Genesis.ProbationIntensityRate)
			pol, _ = poll.NewGovernanceChainCommitteeProtocol(
				indexer,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				cfg.Chain.PollInitialCandidatesInterval,
				slasher)
		}
		svr, bfIndexFile, err := createServerV2(cfg, false)
		require.NoError(err)
		defer func() {
			testutil.CleanupPath(t, bfIndexFile)
		}()
		require.NoError(pol.ForceRegister(svr.core.registry))

		res, err := svr.GrpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(strconv.FormatUint(test.epoch, 10))},
		})
		require.NoError(err)
		var delegates state.CandidateList
		require.NoError(delegates.Deserialize(res.Data))
		require.Equal(test.numDelegates, len(delegates))
	}
}

func TestGrpcServer_ReadBlockProducersByEpoch(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)
	committee := mock_committee.NewMockCommittee(ctrl)
	candidates := []*state.Candidate{
		{
			Address:       "address1",
			Votes:         big.NewInt(1),
			RewardAddress: "rewardAddress",
		},
		{
			Address:       "address2",
			Votes:         big.NewInt(1),
			RewardAddress: "rewardAddress",
		},
	}

	for _, test := range readBlockProducersByEpochTests {
		var pol poll.Protocol
		if test.protocolType == lld {
			cfg.Genesis.Delegates = delegates
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else {
			indexer, err := poll.NewCandidateIndexer(db.NewMemKVStore())
			require.NoError(err)
			slasher, _ := poll.NewSlasher(
				func(uint64, uint64) (map[string]uint64, error) {
					return nil, nil
				},
				func(protocol.StateReader, uint64, bool, bool) ([]*state.Candidate, uint64, error) {
					return candidates, 0, nil
				},
				nil,
				nil,
				indexer,
				test.numCandidateDelegates,
				cfg.Genesis.NumDelegates,
				cfg.Genesis.DardanellesNumSubEpochs,
				cfg.Genesis.ProductivityThreshold,
				cfg.Genesis.ProbationEpochPeriod,
				cfg.Genesis.UnproductiveDelegateMaxCacheSize,
				cfg.Genesis.ProbationIntensityRate)

			pol, _ = poll.NewGovernanceChainCommitteeProtocol(
				indexer,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				cfg.Chain.PollInitialCandidatesInterval,
				slasher)
		}
		svr, bfIndexFile, err := createServerV2(cfg, false)
		require.NoError(err)
		defer func() {
			testutil.CleanupPath(t, bfIndexFile)
		}()
		require.NoError(pol.ForceRegister(svr.core.registry))
		res, err := svr.GrpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(strconv.FormatUint(test.epoch, 10))},
		})
		require.NoError(err)
		var blockProducers state.CandidateList
		require.NoError(blockProducers.Deserialize(res.Data))
		require.Equal(test.numBlockProducers, len(blockProducers))
	}
}

func TestGrpcServer_ReadActiveBlockProducersByEpoch(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)
	committee := mock_committee.NewMockCommittee(ctrl)
	candidates := []*state.Candidate{
		{
			Address:       "address1",
			Votes:         big.NewInt(1),
			RewardAddress: "rewardAddress",
		},
		{
			Address:       "address2",
			Votes:         big.NewInt(1),
			RewardAddress: "rewardAddress",
		},
	}

	for _, test := range readActiveBlockProducersByEpochTests {
		var pol poll.Protocol
		if test.protocolType == lld {
			cfg.Genesis.Delegates = delegates
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else {
			indexer, err := poll.NewCandidateIndexer(db.NewMemKVStore())
			require.NoError(err)
			slasher, _ := poll.NewSlasher(
				func(uint64, uint64) (map[string]uint64, error) {
					return nil, nil
				},
				func(protocol.StateReader, uint64, bool, bool) ([]*state.Candidate, uint64, error) {
					return candidates, 0, nil
				},
				nil,
				nil,
				indexer,
				cfg.Genesis.NumCandidateDelegates,
				test.numDelegates,
				cfg.Genesis.DardanellesNumSubEpochs,
				cfg.Genesis.ProductivityThreshold,
				cfg.Genesis.ProbationEpochPeriod,
				cfg.Genesis.UnproductiveDelegateMaxCacheSize,
				cfg.Genesis.ProbationIntensityRate)
			pol, _ = poll.NewGovernanceChainCommitteeProtocol(
				indexer,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				cfg.Chain.PollInitialCandidatesInterval,
				slasher)
		}
		svr, bfIndexFile, err := createServerV2(cfg, false)
		require.NoError(err)
		defer func() {
			testutil.CleanupPath(t, bfIndexFile)
		}()
		require.NoError(pol.ForceRegister(svr.core.registry))

		res, err := svr.GrpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(strconv.FormatUint(test.epoch, 10))},
		})
		require.NoError(err)
		var activeBlockProducers state.CandidateList
		require.NoError(activeBlockProducers.Deserialize(res.Data))
		require.Equal(test.numActiveBlockProducers, len(activeBlockProducers))
	}
}

func TestGrpcServer_ReadRollDPoSMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	for _, test := range readRollDPoSMetaTests {
		svr, bfIndexFile, err := createServerV2(cfg, false)
		require.NoError(err)
		defer func() {
			testutil.CleanupPath(t, bfIndexFile)
		}()
		res, err := svr.GrpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
		})
		require.NoError(err)
		result, err := strconv.ParseUint(string(res.Data), 10, 64)
		require.NoError(err)
		require.Equal(test.result, result)
	}
}

func TestGrpcServer_ReadEpochCtx(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	for _, test := range readEpochCtxTests {
		svr, bfIndexFile, err := createServerV2(cfg, false)
		require.NoError(err)
		defer func() {
			testutil.CleanupPath(t, bfIndexFile)
		}()
		res, err := svr.GrpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(strconv.FormatUint(test.argument, 10))},
		})
		require.NoError(err)
		result, err := strconv.ParseUint(string(res.Data), 10, 64)
		require.NoError(err)
		require.Equal(test.result, result)
	}
}

func TestGrpcServer_GetEpochMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()
	for _, test := range getEpochMetaTests {
		if test.pollProtocolType == lld {
			pol := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
			require.NoError(pol.ForceRegister(svr.core.registry))
		} else if test.pollProtocolType == "governanceChainCommittee" {
			committee := mock_committee.NewMockCommittee(ctrl)
			mbc := mock_blockchain.NewMockBlockchain(ctrl)
			indexer, err := poll.NewCandidateIndexer(db.NewMemKVStore())
			require.NoError(err)
			slasher, _ := poll.NewSlasher(
				func(uint64, uint64) (map[string]uint64, error) {
					return nil, nil
				},
				func(protocol.StateReader, uint64, bool, bool) ([]*state.Candidate, uint64, error) {
					return []*state.Candidate{
						{
							Address:       identityset.Address(1).String(),
							Votes:         big.NewInt(6),
							RewardAddress: "rewardAddress",
						},
						{
							Address:       identityset.Address(2).String(),
							Votes:         big.NewInt(5),
							RewardAddress: "rewardAddress",
						},
						{
							Address:       identityset.Address(3).String(),
							Votes:         big.NewInt(4),
							RewardAddress: "rewardAddress",
						},
						{
							Address:       identityset.Address(4).String(),
							Votes:         big.NewInt(3),
							RewardAddress: "rewardAddress",
						},
						{
							Address:       identityset.Address(5).String(),
							Votes:         big.NewInt(2),
							RewardAddress: "rewardAddress",
						},
						{
							Address:       identityset.Address(6).String(),
							Votes:         big.NewInt(1),
							RewardAddress: "rewardAddress",
						},
					}, 0, nil
				},
				nil,
				nil,
				indexer,
				cfg.Genesis.NumCandidateDelegates,
				cfg.Genesis.NumDelegates,
				cfg.Genesis.DardanellesNumSubEpochs,
				cfg.Genesis.ProductivityThreshold,
				cfg.Genesis.ProbationEpochPeriod,
				cfg.Genesis.UnproductiveDelegateMaxCacheSize,
				cfg.Genesis.ProbationIntensityRate)
			pol, _ := poll.NewGovernanceChainCommitteeProtocol(
				indexer,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				cfg.Chain.PollInitialCandidatesInterval,
				slasher)
			require.NoError(pol.ForceRegister(svr.core.registry))
			committee.EXPECT().HeightByTime(gomock.Any()).Return(test.epochData.GravityChainStartHeight, nil)

			mbc.EXPECT().TipHeight().Return(uint64(4)).Times(4)
			mbc.EXPECT().BlockHeaderByHeight(gomock.Any()).DoAndReturn(func(height uint64) (*block.Header, error) {
				if height > 0 && height <= 4 {
					pk := identityset.PrivateKey(int(height))
					blk, err := block.NewBuilder(
						block.NewRunnableActionsBuilder().Build(),
					).
						SetHeight(height).
						SetTimestamp(time.Time{}).
						SignAndBuild(pk)
					if err != nil {
						return &block.Header{}, err
					}
					return &blk.Header, nil
				}
				return &block.Header{}, errors.Errorf("invalid block height %d", height)
			}).AnyTimes()
			svr.core.bc = mbc
		}
		svr.core.readCache.Clear()
		res, err := svr.GrpcServer.GetEpochMeta(context.Background(), &iotexapi.GetEpochMetaRequest{EpochNumber: test.EpochNumber})
		require.NoError(err)
		require.Equal(test.epochData.Num, res.EpochData.Num)
		require.Equal(test.epochData.Height, res.EpochData.Height)
		require.Equal(test.epochData.GravityChainStartHeight, res.EpochData.GravityChainStartHeight)
		require.Equal(test.numBlksInEpoch, int(res.TotalBlocks))
		require.Equal(test.numConsenusBlockProducers, len(res.BlockProducersInfo))
		var numActiveBlockProducers int
		var prevInfo *iotexapi.BlockProducerInfo
		for _, bp := range res.BlockProducersInfo {
			if bp.Active {
				numActiveBlockProducers++
			}
			if prevInfo != nil {
				prevVotes, _ := strconv.Atoi(prevInfo.Votes)
				currVotes, _ := strconv.Atoi(bp.Votes)
				require.True(prevVotes >= currVotes)
			}
			prevInfo = bp
		}
		require.Equal(test.numActiveCensusBlockProducers, numActiveBlockProducers)
	}

	// failure: epoch number
	_, err = svr.GrpcServer.GetEpochMeta(context.Background(), &iotexapi.GetEpochMetaRequest{EpochNumber: 0})
	require.Error(err)
}

func TestGrpcServer_GetRawBlocks(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range getRawBlocksTest {
		request := &iotexapi.GetRawBlocksRequest{
			StartHeight:  test.startHeight,
			Count:        test.count,
			WithReceipts: test.withReceipts,
		}
		res, err := svr.GrpcServer.GetRawBlocks(context.Background(), request)
		require.NoError(err)
		blkInfos := res.Blocks
		require.Equal(test.numBlks, len(blkInfos))
		if test.startHeight == 0 {
			// verify genesis block
			header := blkInfos[0].Block.Header.Core
			require.EqualValues(version.ProtocolVersion, header.Version)
			require.Zero(header.Height)
			ts, err := ptypes.TimestampProto(time.Unix(genesis.Timestamp(), 0))
			require.NoError(err)
			require.Equal(ts, header.Timestamp)
			require.Equal(0, bytes.Compare(hash.ZeroHash256[:], header.PrevBlockHash))
			require.Equal(0, bytes.Compare(hash.ZeroHash256[:], header.TxRoot))
			require.Equal(0, bytes.Compare(hash.ZeroHash256[:], header.DeltaStateDigest))
			require.Equal(0, bytes.Compare(hash.ZeroHash256[:], header.ReceiptRoot))
		}
		var numActions, numReceipts int
		for _, blkInfo := range blkInfos {
			numActions += len(blkInfo.Block.Body.Actions)
			numReceipts += len(blkInfo.Receipts)
		}
		require.Equal(test.numActions, numActions)
		require.Equal(test.numReceipts, numReceipts)
	}

	// failure: invalid count
	_, err = svr.GrpcServer.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{
		StartHeight:  1,
		Count:        0,
		WithReceipts: true,
	})
	require.Error(err)

	// failure: invalid startHeight
	_, err = svr.GrpcServer.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{
		StartHeight:  1000000,
		Count:        10,
		WithReceipts: true,
	})
	require.Error(err)

	// failure: invalid endHeight
	_, err = svr.GrpcServer.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{
		StartHeight:  3,
		Count:        1000,
		WithReceipts: true,
	})
	require.Error(err)

}

func TestGrpcServer_GetLogs(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range getLogsByRangeTest {
		request := &iotexapi.GetLogsRequest{
			Filter: &iotexapi.LogsFilter{
				Address: test.address,
				Topics:  test.topics,
			},
			Lookup: &iotexapi.GetLogsRequest_ByRange{
				ByRange: &iotexapi.GetLogsByRange{
					FromBlock: test.fromBlock,
					ToBlock:   test.fromBlock + test.count - 1,
				},
			},
		}
		res, err := svr.GrpcServer.GetLogs(context.Background(), request)
		require.NoError(err)
		logs := res.Logs
		require.Equal(test.numLogs, len(logs))
	}

	for _, v := range blkHash {
		h, _ := hash.HexStringToHash256(v)
		request := &iotexapi.GetLogsRequest{
			Filter: &iotexapi.LogsFilter{
				Address: []string{},
				Topics:  []*iotexapi.Topics{},
			},
			Lookup: &iotexapi.GetLogsRequest_ByBlock{
				ByBlock: &iotexapi.GetLogsByBlock{
					BlockHash: h[:],
				},
			},
		}
		res, err := svr.GrpcServer.GetLogs(context.Background(), request)
		require.NoError(err)
		logs := res.Logs
		require.Equal(1, len(logs))
	}

	// failure: empty request
	_, err = svr.GrpcServer.GetLogs(context.Background(), &iotexapi.GetLogsRequest{
		Filter: &iotexapi.LogsFilter{},
	})
	require.Error(err)

	// failure: empty filter
	_, err = svr.GrpcServer.GetLogs(context.Background(), &iotexapi.GetLogsRequest{})
	require.Error(err)
}

func TestGrpcServer_GetElectionBuckets(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	// failure: no native election
	request := &iotexapi.GetElectionBucketsRequest{
		EpochNum: 0,
	}
	_, err = svr.GrpcServer.GetElectionBuckets(context.Background(), request)
	require.Error(err)
}

func TestGrpcServer_GetActionByActionHash(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	for _, test := range getActionByActionHashTest {
		ret, _, _, _, err := svr.core.ActionByActionHash(test.h)
		require.NoError(err)
		require.Equal(test.expectedNounce, ret.Envelope.Nonce())
	}
}

func TestGrpcServer_GetTransactionLogByActionHash(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	request := &iotexapi.GetTransactionLogByActionHashRequest{
		ActionHash: hex.EncodeToString(hash.ZeroHash256[:]),
	}
	_, err = svr.GrpcServer.GetTransactionLogByActionHash(context.Background(), request)
	require.Error(err)
	sta, ok := status.FromError(err)
	require.Equal(true, ok)
	require.Equal(codes.NotFound, sta.Code())

	for h, log := range implicitLogs {
		request.ActionHash = hex.EncodeToString(h[:])
		res, err := svr.GrpcServer.GetTransactionLogByActionHash(context.Background(), request)
		require.NoError(err)
		require.Equal(log.Proto(), res.TransactionLog)
	}

	// check implicit transfer receiver balance
	state, err := accountutil.LoadAccount(svr.core.sf, identityset.Address(31))
	require.NoError(err)
	require.Equal(big.NewInt(5), state.Balance)
}

func TestGrpcServer_GetEvmTransfersByBlockHeight(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	request := &iotexapi.GetTransactionLogByBlockHeightRequest{}
	for _, test := range getImplicitLogByBlockHeightTest {
		request.BlockHeight = test.height
		res, err := svr.GrpcServer.GetTransactionLogByBlockHeight(context.Background(), request)
		if test.code != codes.OK {
			require.Error(err)
			sta, ok := status.FromError(err)
			require.Equal(true, ok)
			require.Equal(test.code, sta.Code())
		} else {
			require.NotNil(res)
			// verify log
			for _, log := range res.TransactionLogs.Logs {
				l, ok := implicitLogs[hash.BytesToHash256(log.ActionHash)]
				require.True(ok)
				require.Equal(l.Proto(), log)
			}
			require.Equal(test.height, res.BlockIdentifier.Height)
			require.Equal(blkHash[test.height], res.BlockIdentifier.Hash)
		}
	}
}

func addTestingBlocks(bc blockchain.Blockchain, ap actpool.ActPool) error {
	ctx := context.Background()
	addr0 := identityset.Address(27).String()
	addr1 := identityset.Address(28).String()
	addr2 := identityset.Address(29).String()
	addr3 := identityset.Address(30).String()
	priKey3 := identityset.PrivateKey(30)
	addr4 := identityset.Address(31).String()
	// Add block 1
	// Producer transfer--> C
	implicitLogs[transferHash1] = block.NewTransactionLog(transferHash1,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "10", addr0, addr3)},
	)

	blk1Time := testutil.TimestampNow()
	if err := ap.Add(ctx, testTransfer1); err != nil {
		return err
	}
	blk, err := bc.MintNewBlock(blk1Time)
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	ap.Reset()
	h := blk.HashBlock()
	blkHash[1] = hex.EncodeToString(h[:])

	// Add block 2
	// Charlie transfer--> A, B, D, P
	// Charlie transfer--> C
	// Charlie exec--> D
	recipients := []string{addr1, addr2, addr4, addr0}
	for i, recipient := range recipients {
		selp, err := action.SignedTransfer(recipient, priKey3, uint64(i+1), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		if err != nil {
			return err
		}
		if err := ap.Add(ctx, selp); err != nil {
			return err
		}
		selpHash, err := selp.Hash()
		if err != nil {
			return err
		}
		implicitLogs[selpHash] = block.NewTransactionLog(selpHash,
			[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "1", addr3, recipient)},
		)
	}
	implicitLogs[transferHash2] = block.NewTransactionLog(transferHash2,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "2", addr3, addr3)},
	)
	if err := ap.Add(ctx, testTransfer2); err != nil {
		return err
	}
	implicitLogs[executionHash1] = block.NewTransactionLog(
		executionHash1,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, "1", addr3, addr4)},
	)
	if err := ap.Add(ctx, testExecution1); err != nil {
		return err
	}
	if blk, err = bc.MintNewBlock(blk1Time.Add(time.Second)); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	ap.Reset()
	h = blk.HashBlock()
	blkHash[2] = hex.EncodeToString(h[:])

	// Add block 3
	// Empty actions
	if blk, err = bc.MintNewBlock(blk1Time.Add(time.Second * 2)); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	ap.Reset()
	h = blk.HashBlock()
	blkHash[3] = hex.EncodeToString(h[:])

	// Add block 4
	// Charlie transfer--> C
	// Alfa transfer--> A
	// Charlie exec--> D
	// Alfa exec--> D
	tsf1, err := action.SignedTransfer(addr3, priKey3, uint64(7), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf1Hash, err := tsf1.Hash()
	if err != nil {
		return err
	}
	implicitLogs[tsf1Hash] = block.NewTransactionLog(tsf1Hash,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "1", addr3, addr3)},
	)
	if err := ap.Add(ctx, tsf1); err != nil {
		return err
	}
	tsf2, err := action.SignedTransfer(addr1, identityset.PrivateKey(28), uint64(1), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2Hash, err := tsf2.Hash()
	if err != nil {
		return err
	}
	implicitLogs[tsf2Hash] = block.NewTransactionLog(tsf2Hash,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "1", addr1, addr1)},
	)
	if err := ap.Add(ctx, tsf2); err != nil {
		return err
	}
	execution1, err := action.SignedExecution(addr4, priKey3, 8,
		big.NewInt(2), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}
	execution1Hash, err := execution1.Hash()
	if err != nil {
		return err
	}
	implicitLogs[execution1Hash] = block.NewTransactionLog(
		execution1Hash,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, "2", addr3, addr4)},
	)
	if err := ap.Add(ctx, execution1); err != nil {
		return err
	}
	implicitLogs[executionHash3] = block.NewTransactionLog(
		executionHash3,
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, "1", addr1, addr4)},
	)
	if err := ap.Add(ctx, testExecution3); err != nil {
		return err
	}
	if blk, err = bc.MintNewBlock(blk1Time.Add(time.Second * 3)); err != nil {
		return err
	}
	h = blk.HashBlock()
	blkHash[4] = hex.EncodeToString(h[:])
	return bc.CommitBlock(blk)
}

func deployContractV2(svr *ServerV2, key crypto.PrivateKey, nonce, height uint64, code string) (string, error) {
	data, _ := hex.DecodeString(code)
	ex1, err := action.SignedExecution(action.EmptyAddress, key, nonce, big.NewInt(0), 500000, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return "", err
	}
	if err := svr.core.ap.Add(context.Background(), ex1); err != nil {
		return "", err
	}
	blk, err := svr.core.bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return "", err
	}
	if err := svr.core.bc.CommitBlock(blk); err != nil {
		return "", err
	}
	svr.core.ap.Reset()
	// get deployed contract address
	var contract string
	if svr.core.dao != nil {
		ex1Hash, err := ex1.Hash()
		if err != nil {
			return "", err
		}
		r, err := svr.core.dao.GetReceiptByActionHash(ex1Hash, height+1)
		if err != nil {
			return "", err
		}
		contract = r.ContractAddress
	}
	return contract, nil
}

func addActsToActPool(ctx context.Context, ap actpool.ActPool) error {
	// Producer transfer--> A
	tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 2, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// Producer transfer--> P
	tsf2, err := action.SignedTransfer(identityset.Address(27).String(), identityset.PrivateKey(27), 3, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// Producer transfer--> B
	tsf3, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 4, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// Producer exec--> D
	execution1, err := action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(27), 5,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(10), []byte{1})
	if err != nil {
		return err
	}

	if err := ap.Add(ctx, tsf1); err != nil {
		return err
	}
	if err := ap.Add(ctx, tsf2); err != nil {
		return err
	}
	if err := ap.Add(ctx, tsf3); err != nil {
		return err
	}
	return ap.Add(ctx, execution1)
}

func setupChain(cfg config.Config) (blockchain.Blockchain, blockdao.BlockDAO, blockindex.Indexer, blockindex.BloomFilterIndexer, factory.Factory, actpool.ActPool, *protocol.Registry, string, error) {
	cfg.Chain.ProducerPrivKey = hex.EncodeToString(identityset.PrivateKey(0).Bytes())
	registry := protocol.NewRegistry()
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}
	ap, err := setupActPool(sf, cfg.ActPool)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = unit.ConvertIotxToRau(10000000000).String()
	// create indexer
	indexer, err := blockindex.NewIndexer(db.NewMemKVStore(), cfg.Genesis.Hash())
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", errors.New("failed to create indexer")
	}
	testPath, _ := testutil.PathOfTempFile("bloomfilter")
	cfg.DB.DbPath = testPath
	bfIndexer, err := blockindex.NewBloomfilterIndexer(db.NewBoltDB(cfg.DB), cfg.Indexer)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", errors.New("failed to create bloomfilter indexer")
	}
	// create BlockDAO
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf, indexer, bfIndexer})
	if dao == nil {
		return nil, nil, nil, nil, nil, nil, nil, "", errors.New("failed to create blockdao")
	}
	// create chain
	bc := blockchain.NewBlockchain(
		cfg,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	if bc == nil {
		return nil, nil, nil, nil, nil, nil, nil, "", errors.New("failed to create blockchain")
	}
	defer func() {
		delete(cfg.Plugins, config.GatewayPlugin)
	}()

	acc := account.NewProtocol(rewarding.DepositGas)
	evm := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
	p := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
	rolldposProtocol := rolldpos.NewProtocol(
		genesis.Default.NumCandidateDelegates,
		genesis.Default.NumDelegates,
		genesis.Default.NumSubEpochs,
		rolldpos.EnableDardanellesSubEpoch(cfg.Genesis.DardanellesBlockHeight, cfg.Genesis.DardanellesNumSubEpochs),
	)
	r := rewarding.NewProtocol(cfg.Genesis.Rewarding)

	if err := rolldposProtocol.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}
	if err := acc.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}
	if err := evm.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}
	if err := r.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}
	if err := p.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, "", err
	}

	return bc, dao, indexer, bfIndexer, sf, ap, registry, testPath, nil
}

func setupActPool(sf factory.Factory, cfg config.ActPool) (actpool.ActPool, error) {
	ap, err := actpool.NewActPool(sf, cfg, actpool.EnableExperimentalActions())
	if err != nil {
		return nil, err
	}

	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	return ap, nil
}

func newConfig(t *testing.T) config.Config {
	r := require.New(t)
	cfg := config.Default

	testTriePath, err := testutil.PathOfTempFile("trie")
	r.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	r.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	r.NoError(err)
	testSystemLogPath, err := testutil.PathOfTempFile("systemlog")
	r.NoError(err)

	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.System.SystemLogDBPath = testSystemLogPath
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.EnableGravityChainVoting = true
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.API.RangeQueryLimit = 100

	return cfg
}

func createServerV2(cfg config.Config, needActPool bool) (*ServerV2, string, error) {
	// TODO (zhi): revise
	bc, dao, indexer, bfIndexer, sf, ap, registry, bfIndexFile, err := setupChain(cfg)
	if err != nil {
		return nil, "", err
	}

	ctx := context.Background()

	// Start blockchain
	if err := bc.Start(ctx); err != nil {
		return nil, "", err
	}
	// Add testing blocks
	if err := addTestingBlocks(bc, ap); err != nil {
		return nil, "", err
	}

	if needActPool {
		// Add actions to actpool
		ctx = protocol.WithRegistry(ctx, registry)
		if err := addActsToActPool(ctx, ap); err != nil {
			return nil, "", err
		}
	}
	svr, err := NewServerV2(cfg, bc, nil, sf, dao, indexer, bfIndexer, ap, registry,
		WithBroadcastOutbound(func(ctx context.Context, chainID uint32, msg proto.Message) error {
			return nil
		}))
	if err != nil {
		return nil, "", err
	}
	svr.core.hasActionIndex = true
	return svr, bfIndexFile, nil
}

func TestGrpcServer_GetActPoolActions(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	ctx := context.Background()

	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	res, err := svr.GrpcServer.GetActPoolActions(ctx, &iotexapi.GetActPoolActionsRequest{})
	require.NoError(err)
	require.Equal(len(svr.core.ap.PendingActionMap()[identityset.Address(27).String()]), len(res.Actions))

	tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 2,
		big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(identityset.Address(27).String(), identityset.PrivateKey(27), 3,
		big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 4,
		big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	execution1, err := action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(27), 5,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(10), []byte{1})
	require.NoError(err)

	err = svr.core.ap.Add(ctx, tsf1)
	require.NoError(err)
	err = svr.core.ap.Add(ctx, tsf2)
	require.NoError(err)
	err = svr.core.ap.Add(ctx, execution1)
	require.NoError(err)

	var requests []string
	h1, err := tsf1.Hash()
	require.NoError(err)
	requests = append(requests, hex.EncodeToString(h1[:]))

	res, err = svr.GrpcServer.GetActPoolActions(context.Background(), &iotexapi.GetActPoolActionsRequest{})
	require.NoError(err)
	require.Equal(len(svr.core.ap.PendingActionMap()[identityset.Address(27).String()]), len(res.Actions))

	res, err = svr.GrpcServer.GetActPoolActions(context.Background(), &iotexapi.GetActPoolActionsRequest{ActionHashes: requests})
	require.NoError(err)
	require.Equal(1, len(res.Actions))

	h2, err := tsf2.Hash()
	require.NoError(err)
	requests = append(requests, hex.EncodeToString(h2[:]))
	res, err = svr.GrpcServer.GetActPoolActions(context.Background(), &iotexapi.GetActPoolActionsRequest{ActionHashes: requests})
	require.NoError(err)
	require.Equal(2, len(res.Actions))

	h3, err := tsf3.Hash()
	require.NoError(err)
	_, err = svr.GrpcServer.GetActPoolActions(context.Background(), &iotexapi.GetActPoolActionsRequest{ActionHashes: []string{hex.EncodeToString(h3[:])}})
	require.Error(err)
}

func TestGrpcServer_GetEstimateGasSpecial(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, true)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	// deploy self-desturct contract
	contractCode := "608060405234801561001057600080fd5b50336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550610196806100606000396000f3fe608060405234801561001057600080fd5b50600436106100415760003560e01c80632e64cec11461004657806343d726d6146100645780636057361d1461006e575b600080fd5b61004e61008a565b60405161005b9190610124565b60405180910390f35b61006c610094565b005b610088600480360381019061008391906100ec565b6100cd565b005b6000600154905090565b60008054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16ff5b8060018190555050565b6000813590506100e681610149565b92915050565b6000602082840312156100fe57600080fd5b600061010c848285016100d7565b91505092915050565b61011e8161013f565b82525050565b60006020820190506101396000830184610115565b92915050565b6000819050919050565b6101528161013f565b811461015d57600080fd5b5056fea264697066735822122060e7a28baea4232a95074b94b50009d1d7b99302ef6556a1f3ce7f46a49f8cc064736f6c63430008000033"
	contract, err := deployContractV2(svr, identityset.PrivateKey(13), 1, svr.core.bc.TipHeight(), contractCode)

	require.NoError(err)
	require.True(len(contract) > 0)

	// call self-destuct func, which will invoke gas refund policy
	data := "43d726d6"
	byteCodes, err := hex.DecodeString(data)
	require.NoError(err)
	execution, err := action.NewExecution(contract, 2, big.NewInt(0), 0, big.NewInt(0), byteCodes)
	require.NoError(err)
	request := &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_Execution{
			Execution: execution.Proto(),
		},
		CallerAddress: identityset.Address(13).String(),
	}
	res, err := svr.GrpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10777), res.Gas)
}

func TestChainlinkErrTest(t *testing.T) {
	require := require.New(t)

	gethFatal := regexp.MustCompile(`(: |^)(exceeds block gas limit|invalid sender|negative value|oversized data|gas uint64 overflow|intrinsic gas too low|nonce too high)$`)

	tests := []struct {
		testName string
		server   func() (*ServerV2, string, error)
		actions  []*iotextypes.Action
		errRegex *regexp.Regexp
	}{
		{
			"NonceTooLow",
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			[]*iotextypes.Action{testTransferInvalid1Pb},
			regexp.MustCompile(`(: |^)nonce too low$`),
		},
		{
			"TerminallyUnderpriced",
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			[]*iotextypes.Action{testTransferInvalid2Pb},
			regexp.MustCompile(`(: |^)transaction underpriced$`),
		},
		{
			"InsufficientEth",
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			[]*iotextypes.Action{testTransferInvalid3Pb},
			regexp.MustCompile(`(: |^)(insufficient funds for transfer|insufficient funds for gas \* price \+ value|insufficient balance for transfer)$`),
		},

		{
			"NonceTooHigh",
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			[]*iotextypes.Action{testTransferInvalid4Pb},
			gethFatal,
		},
		{
			"TransactionAlreadyInMempool",
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			[]*iotextypes.Action{testTransferPb, testTransferPb},
			regexp.MustCompile(`(: |^)(?i)(known transaction|already known)`),
		},
		{
			"ReplacementTransactionUnderpriced",
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			[]*iotextypes.Action{testTransferPb, testTransferInvalid5Pb},
			regexp.MustCompile(`(: |^)replacement transaction underpriced$`),
		},
		{
			"IntrinsicGasTooLow",
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			[]*iotextypes.Action{testTransferInvalid6Pb},
			gethFatal,
		},
		{
			"NegativeValue",
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			[]*iotextypes.Action{testTransferInvalid7Pb},
			gethFatal,
		},
		{
			"ExceedsBlockGasLimit",
			func() (*ServerV2, string, error) {
				cfg := newConfig(t)
				return createServerV2(cfg, true)
			},
			[]*iotextypes.Action{testTransferInvalid8Pb},
			gethFatal,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			svr, file, err := test.server()
			require.NoError(err)
			defer func() {
				testutil.CleanupPath(t, file)
			}()

			for _, action := range test.actions {
				_, err = svr.GrpcServer.SendAction(context.Background(), &iotexapi.SendActionRequest{Action: action})
				if err != nil {
					break
				}
			}
			s, ok := status.FromError(err)
			require.True(ok)
			require.True(test.errRegex.MatchString(s.Message()))
		})
	}
}

func TestGrpcServer_TraceTransactionStructLogs(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, bfIndexFile, err := createServerV2(cfg, true)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	request := &iotexapi.TraceTransactionStructLogsRequest{
		ActionHash: hex.EncodeToString(hash.ZeroHash256[:]),
	}
	_, err = svr.GrpcServer.TraceTransactionStructLogs(context.Background(), request)
	require.Error(err)

	//unsupport type
	request.ActionHash = hex.EncodeToString(transferHash1[:])
	_, err = svr.GrpcServer.TraceTransactionStructLogs(context.Background(), request)
	require.Error(err)

	// deploy a contract
	contractCode := "6080604052348015600f57600080fd5b5060de8061001e6000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c8063ee82ac5e14602d575b600080fd5b605660048036036020811015604157600080fd5b8101908080359060200190929190505050606c565b6040518082815260200191505060405180910390f35b60008082409050807f2d93f7749862d33969fb261757410b48065a1bc86a56da5c47820bd063e2338260405160405180910390a28091505091905056fea265627a7a723158200a258cd08ea99ee11aa68c78b6d2bf7ea912615a1e64a81b90a2abca2dd59cfa64736f6c634300050c0032"

	data, _ := hex.DecodeString(contractCode)
	ex1, err := action.SignedExecution(action.EmptyAddress, identityset.PrivateKey(13), 1, big.NewInt(0), 500000, big.NewInt(testutil.TestGasPriceInt64), data)
	require.NoError(err)
	svr.core.ap.Add(context.Background(), ex1)
	require.NoError(err)
	blk, err := svr.core.bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	svr.core.bc.CommitBlock(blk)
	require.NoError(err)
	svr.core.ap.Reset()
	ex1Hash, _ := ex1.Hash()
	request.ActionHash = hex.EncodeToString(ex1Hash[:])
	ret, err := svr.GrpcServer.TraceTransactionStructLogs(context.Background(), request)
	require.NoError(err)
	require.Equal(len(ret.StructLogs), 17)
	log := ret.StructLogs[0]
	require.Equal(log.Depth, int32(1))
	require.Equal(log.Gas, uint64(0x4bc1c0))
	require.Equal(log.GasCost, uint64(0x3))
	require.Equal(log.Op, uint64(0x60))
	require.Equal(log.OpName, "PUSH1")
}
