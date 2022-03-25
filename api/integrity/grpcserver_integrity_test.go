// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package integrity

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/pb/election"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-election/types"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/api"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/chainservice"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockdao"
	"github.com/iotexproject/iotex-core/test/mock/mock_factory"
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
		firstTxGas string
	}{
		{
			2,
			0,
			7,
			7,
			"0",
		},
		{
			4,
			2,
			5,
			3,
			"0",
		},
		{
			3,
			0,
			0,
			0,
			"",
		},
		{
			1,
			0,
			math.MaxUint64,
			2,
			"0",
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

func TestGrpcServer_GetAccountIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), true)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	// deploy a contract
	contractCode := "6080604052348015600f57600080fd5b5060de8061001e6000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c8063ee82ac5e14602d575b600080fd5b605660048036036020811015604157600080fd5b8101908080359060200190929190505050606c565b6040518082815260200191505060405180910390f35b60008082409050807f2d93f7749862d33969fb261757410b48065a1bc86a56da5c47820bd063e2338260405160405180910390a28091505091905056fea265627a7a723158200a258cd08ea99ee11aa68c78b6d2bf7ea912615a1e64a81b90a2abca2dd59cfa64736f6c634300050c0032"
	contract, err := deployContractV2(cs, identityset.PrivateKey(13), 1, contractCode)
	require.NoError(err)
	require.True(len(contract) > 0)

	// read contract address
	request := &iotexapi.GetAccountRequest{Address: contract}
	res, err := grpcServer.GetAccount(context.Background(), request)
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
		res, err := grpcServer.GetAccount(context.Background(), request)
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
	_, err = grpcServer.GetAccount(context.Background(), &iotexapi.GetAccountRequest{})
	require.Error(err)
	// error account
	_, err = grpcServer.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: "io3fn88lge6hyzmruh40cn6l3e49dfkqzqk3lgtq3"})
	require.Error(err)

	// success: reward pool
	res, err = grpcServer.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: address.RewardingPoolAddr})
	require.NoError(err)
	require.Equal(address.RewardingPoolAddr, res.AccountMeta.Address)
	require.Equal("200000000000000000000101000", res.AccountMeta.Balance)
	require.EqualValues(5, res.BlockIdentifier.Height)
	require.NotZero(res.BlockIdentifier.Hash)

	// TODO: add failure case with protocol staking isn't registered
	// res, err = grpcServer.GetAccount(context.Background(), &iotexapi.GetAccountRequest{Address: address.StakingBucketPoolAddr})
	// require.Error(err)
}

func TestGrpcServer_GetActionsIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	for _, test := range getActionsTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByIndex{
				ByIndex: &iotexapi.GetActionsByIndexRequest{
					Start: test.start,
					Count: test.count,
				},
			},
		}

		res, err := grpcServer.GetActions(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
		} else {
			require.NoError(err)
			require.Equal(test.numActions, len(res.ActionInfo))
		}

		// TODO (huof6829): create a core service with hasActionIndex disabled to test
	}

	// failure: empty request
	_, err = grpcServer.GetActions(context.Background(), &iotexapi.GetActionsRequest{})
	require.Error(err)

	// failure: range exceed limit
	_, err = grpcServer.GetActions(context.Background(),
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
	_, err = grpcServer.GetActions(context.Background(),
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

func TestGrpcServer_GetActionIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), true)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	for _, test := range getActionTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByHash{
				ByHash: &iotexapi.GetActionByHashRequest{
					ActionHash:   test.in,
					CheckPending: test.checkPending,
				},
			},
		}
		res, err := grpcServer.GetActions(context.Background(), request)
		require.NoError(err)
		require.Equal(1, len(res.ActionInfo))
		act := res.ActionInfo[0]
		require.Equal(test.nonce, act.Action.GetCore().GetNonce())
		require.Equal(test.senderPubKey, hex.EncodeToString(act.Action.SenderPubKey))
		if !test.checkPending {
			blk, err := cs.BlockDAO().GetBlockByHeight(test.blkNumber)
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
	_, err = grpcServer.GetActions(context.Background(),
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

func TestGrpcServer_GetActionsByAddressIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

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
		res, err := grpcServer.GetActions(context.Background(), request)
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
			prevRes, err := grpcServer.GetActions(context.Background(), request)
			require.NoError(err)
			require.True(prevRes.ActionInfo[len(prevRes.ActionInfo)-1].Timestamp.GetSeconds() <= res.ActionInfo[0].Timestamp.GetSeconds())
		}
	}
}

func TestGrpcServer_GetUnconfirmedActionsByAddressIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), true)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

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
		res, err := grpcServer.GetActions(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
			continue
		}
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
		require.Equal(test.address, res.ActionInfo[0].Sender)
	}
}

func TestGrpcServer_GetActionsByBlockIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

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
		res, err := grpcServer.GetActions(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
			continue
		}
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
		if test.numActions > 0 {
			require.Equal(test.firstTxGas, res.ActionInfo[0].GasFee)
		}
		for _, v := range res.ActionInfo {
			require.Equal(test.blkHeight, v.BlkHeight)
			require.Equal(blkHash[test.blkHeight], v.BlkHash)
		}
	}
}

func TestGrpcServer_GetBlockMetasIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	genesis.SetGenesisTimestamp(cfg.Genesis.Timestamp)
	block.LoadGenesisHash(&cfg.Genesis)
	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	for _, test := range getBlockMetasTests {
		request := &iotexapi.GetBlockMetasRequest{
			Lookup: &iotexapi.GetBlockMetasRequest_ByIndex{
				ByIndex: &iotexapi.GetBlockMetasByIndexRequest{
					Start: test.start,
					Count: test.count,
				},
			},
		}
		res, err := grpcServer.GetBlockMetas(context.Background(), request)
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
	_, err = grpcServer.GetBlockMetas(context.Background(), &iotexapi.GetBlockMetasRequest{})
	require.Error(err)

	_, err = grpcServer.GetBlockMetas(context.Background(), &iotexapi.GetBlockMetasRequest{
		Lookup: &iotexapi.GetBlockMetasRequest_ByIndex{
			ByIndex: &iotexapi.GetBlockMetasByIndexRequest{Start: 10, Count: 1},
		},
	})
	require.Error(err)

	_, err = grpcServer.GetBlockMetas(context.Background(), &iotexapi.GetBlockMetasRequest{
		Lookup: &iotexapi.GetBlockMetasRequest_ByHash{
			ByHash: &iotexapi.GetBlockMetaByHashRequest{BlkHash: "0xa2e8e0c9cafbe93f2b7f7c9d32534bc6fde95f2185e5f2aaa6bf7ebdf1a6610a"},
		},
	})
	require.Error(err)
}

func TestGrpcServer_GetBlockMetaIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	for _, test := range getBlockMetaTests {
		header, err := cs.Blockchain().BlockHeaderByHeight(test.blkHeight)
		require.NoError(err)
		blkHash := header.HashBlock()
		request := &iotexapi.GetBlockMetasRequest{
			Lookup: &iotexapi.GetBlockMetasRequest_ByHash{
				ByHash: &iotexapi.GetBlockMetaByHashRequest{
					BlkHash: hex.EncodeToString(blkHash[:]),
				},
			},
		}
		res, err := grpcServer.GetBlockMetas(context.Background(), request)
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

func TestGrpcServer_GetChainMetaIntegrity(t *testing.T) {
	require := require.New(t)

	var pol poll.Protocol
	for _, test := range getChainMetaTests {
		ctrl := gomock.NewController(t)
		cfg := newConfig(t)
		if test.pollProtocolType == lld {
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else if test.pollProtocolType == "governanceChainCommittee" {
			committee := mock_committee.NewMockCommittee(ctrl)
			slasher, _ := poll.NewSlasher(
				func(uint64, uint64) (map[string]uint64, error) {
					return nil, nil
				},
				func(protocol.StateReader, uint64, bool, bool) ([]*state.Candidate, uint64, error) {
					return nil, 0, nil
				},
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
			committee.EXPECT().ResultByHeight(gomock.Any()).Return(types.NewElectionResultForTest(time.Now()), nil)
		}

		cfg.API.TpsWindow = test.tpsWindow
		builder := chainservice.NewBuilder(cfg)
		if pol != nil {
			registry := protocol.NewRegistry()
			require.NoError(account.NewProtocol(rewarding.DepositGas).ForceRegister(registry))
			require.NoError(pol.ForceRegister(registry))
			builder = builder.SetRegistry(registry)
		}
		cs, err := builder.BuildForTest()
		require.NoError(err)
		require.NoError(cs.Start(context.Background()))
		if !test.emptyChain {
			require.NoError(addTestingBlocks(cs.Blockchain(), cs.ActionPool()))
		}
		grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))
		res, err := grpcServer.GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{})
		require.NoError(err)
		chainMetaPb := res.ChainMeta
		require.Equal(test.height, chainMetaPb.Height)
		require.Equal(test.numActions, chainMetaPb.NumActions)
		require.Equal(test.tps, chainMetaPb.Tps)
		require.Equal(test.epoch.Num, chainMetaPb.Epoch.Num)
		require.Equal(test.epoch.Height, chainMetaPb.Epoch.Height)
		require.Equal(test.epoch.GravityChainStartHeight, chainMetaPb.Epoch.GravityChainStartHeight)
		require.NoError(cs.Stop(context.Background()))
	}
}

func TestGrpcServer_SendActionIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	cfg.Genesis.MidwayBlockHeight = 10

	cs, err := createChainService(cfg, nil, true)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	for _, test := range sendActionTests {
		request := &iotexapi.SendActionRequest{Action: test.actionPb}
		res, err := grpcServer.SendAction(context.Background(), request)
		require.NoError(err)
		require.Equal(test.actionHash, res.ActionHash)
	}

	// 3 failure cases
	ctx := context.Background()
	tests := []struct {
		cfg    func() config.Config
		action *iotextypes.Action
		err    string
	}{
		{
			func() config.Config {
				return newConfig(t)
			},
			&iotextypes.Action{},
			"invalid signature length",
		},
		{
			func() config.Config {
				return newConfig(t)
			},
			&iotextypes.Action{
				Signature: action.ValidSig,
			},
			"empty action proto to load",
		},
		{
			func() config.Config {
				cfg := newConfig(t)
				cfg.ActPool.MaxNumActsPerPool = 6
				return cfg
			},
			testTransferPb,
			action.ErrTxPoolOverflow.Error(),
		},
		{
			func() config.Config {
				return newConfig(t)
			},
			testTransferInvalid1Pb,
			action.ErrNonceTooLow.Error(),
		},
		{
			func() config.Config {
				return newConfig(t)
			},
			testTransferInvalid2Pb,
			action.ErrUnderpriced.Error(),
		},
		{
			func() config.Config {
				return newConfig(t)
			},
			testTransferInvalid3Pb,
			action.ErrInsufficientFunds.Error(),
		},
	}

	for _, test := range tests {
		request := &iotexapi.SendActionRequest{Action: test.action}
		cs, err := createChainService(test.cfg(), nil, true)
		require.NoError(err)
		grpcServer := api.NewGRPCServer(cs, test.cfg().API.Port, test.cfg().API.RangeQueryLimit, uint64(test.cfg().API.TpsWindow))

		_, err = grpcServer.SendAction(ctx, request)
		require.Contains(err.Error(), test.err)
	}
}

func TestGrpcServer_StreamLogsIntegrity(t *testing.T) {
	// TODO: add back unit test for stream log
}

func TestGrpcServer_GetReceiptByActionIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	for _, test := range getReceiptByActionTests {
		request := &iotexapi.GetReceiptByActionRequest{ActionHash: test.in}
		res, err := grpcServer.GetReceiptByAction(context.Background(), request)
		require.NoError(err)
		receiptPb := res.ReceiptInfo.Receipt
		require.Equal(test.status, receiptPb.Status)
		require.Equal(test.blkHeight, receiptPb.BlkHeight)
		require.NotEqual(hash.ZeroHash256, res.ReceiptInfo.BlkHash)
	}

	// failure: empty request
	_, err = grpcServer.GetReceiptByAction(context.Background(), &iotexapi.GetReceiptByActionRequest{ActionHash: "0x"})
	require.Error(err)
	// failure: wrong hash
	_, err = grpcServer.GetReceiptByAction(context.Background(), &iotexapi.GetReceiptByActionRequest{ActionHash: "b7faffcb8b01fa9f32112155bcb93d714f599eab3178e577e88dafd2140bfc5a"})
	require.Error(err)

}

func TestGrpcServer_GetServerMetaIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	resProto, err := grpcServer.GetServerMeta(context.Background(), &iotexapi.GetServerMetaRequest{})
	require.NoError(err)
	res := resProto.GetServerMeta()
	require.Equal(res.BuildTime, version.BuildTime)
	require.Equal(res.GoVersion, version.GoVersion)
	require.Equal(res.GitStatus, version.GitStatus)
	require.Equal(res.PackageCommitID, version.PackageCommitID)
	require.Equal(res.PackageVersion, version.PackageVersion)
}

func TestGrpcServer_ReadContractIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	for _, test := range readContractTests {
		hash, err := hash.HexStringToHash256(test.execHash)
		require.NoError(err)
		ai, err := cs.Indexer().GetActionIndex(hash[:])
		require.NoError(err)
		exec, _, err := cs.BlockDAO().GetActionByActionHash(hash, ai.BlockHeight())
		require.NoError(err)
		request := &iotexapi.ReadContractRequest{
			Execution:     exec.Proto().GetCore().GetExecution(),
			CallerAddress: test.callerAddr,
			GasLimit:      exec.GasLimit(),
			GasPrice:      big.NewInt(unit.Qev).String(),
		}

		res, err := grpcServer.ReadContract(context.Background(), request)
		require.NoError(err)
		require.Equal(test.retValue, res.Data)
		require.EqualValues(1, res.Receipt.Status)
		require.Equal(test.actionHash, hex.EncodeToString(res.Receipt.ActHash))
		require.Equal(test.gasConsumed, res.Receipt.GasConsumed)
	}
}

func TestGrpcServer_SuggestGasPriceIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	for _, test := range suggestGasPriceTests {
		cfg.API.GasStation.DefaultGas = test.defaultGasPrice
		cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
		require.NoError(err)
		grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

		res, err := grpcServer.SuggestGasPrice(context.Background(), &iotexapi.SuggestGasPriceRequest{})
		require.NoError(err)
		require.Equal(test.suggestedGasPrice, res.GasPrice)
	}
}

func TestGrpcServer_EstimateGasForActionIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	for _, test := range estimateGasForActionTests {
		hash, err := hash.HexStringToHash256(test.actionHash)
		require.NoError(err)
		ai, err := cs.Indexer().GetActionIndex(hash[:])
		require.NoError(err)
		act, _, err := cs.BlockDAO().GetActionByActionHash(hash, ai.BlockHeight())
		require.NoError(err)
		request := &iotexapi.EstimateGasForActionRequest{Action: act.Proto()}

		res, err := grpcServer.EstimateGasForAction(context.Background(), request)
		require.NoError(err)
		require.Equal(test.estimatedGas, res.Gas)
	}
}

func TestGrpcServer_EstimateActionGasConsumptionIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

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
	res, err := grpcServer.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = grpcServer.EstimateActionGasConsumption(context.Background(), request)
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
	createStake, err := action.NewCreateStake(nonce, canAddress, amount.String(), duration, autoStake, payload, gaslimit, gasprice)
	require.NoError(err)
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_StakeCreate{
			StakeCreate: createStake.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err = grpcServer.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = grpcServer.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = grpcServer.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = grpcServer.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = grpcServer.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = grpcServer.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = grpcServer.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = grpcServer.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = grpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10000), res.Gas)

	// Case X: test for action nil
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action:        nil,
		CallerAddress: identityset.Address(0).String(),
	}
	_, err = grpcServer.EstimateActionGasConsumption(context.Background(), request)
	require.Error(err)
}

func TestGrpcServer_ReadUnclaimedBalanceIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	for _, test := range readUnclaimedBalanceTests {
		out, err := grpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(test.addr)},
		})
		if test.returnErr {
			require.Error(err)
			continue
		}
		require.NoError(err)
		val, ok := new(big.Int).SetString(string(out.Data), 10)
		require.True(ok)
		require.Equal(test.balance, val)
	}
}

func TestGrpcServer_TotalBalanceIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	out, err := grpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte("TotalBalance"),
		Arguments:  nil,
	})
	require.NoError(err)
	val, ok := new(big.Int).SetString(string(out.Data), 10)
	require.True(ok)
	require.Equal(unit.ConvertIotxToRau(200000000), val)
}

func TestGrpcServer_AvailableBalanceIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	out, err := grpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte("AvailableBalance"),
		Arguments:  nil,
	})
	require.NoError(err)
	val, ok := new(big.Int).SetString(string(out.Data), 10)
	require.True(ok)
	require.Equal(unit.ConvertIotxToRau(199999936), val)
}

func TestGrpcServer_ReadCandidatesByEpochIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)
	committee := mock_committee.NewMockCommittee(ctrl)
	committee.EXPECT().ResultByHeight(gomock.Any()).Return(types.NewElectionResultForTest(time.Now()), nil)
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
		builder := chainservice.NewBuilder(cfg)
		var pol poll.Protocol
		if test.protocolType == lld {
			cfg.Genesis.Delegates = delegates
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else {
			cfg.Consensus.Scheme = config.RollDPoSScheme
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
			dao := mock_blockdao.NewMockBlockDAO(ctrl)
			dao.EXPECT().GetBlockHash(gomock.Any()).Return(hash.Hash256{}, nil).Times(1)
			dao.EXPECT().Height().Return(uint64(0), nil).Times(1)
			dao.EXPECT().Start(gomock.Any()).Return(nil).Times(1)
			dao.EXPECT().Stop(gomock.Any()).Return(nil).Times(1)
			factory := mock_factory.NewMockFactory(ctrl)
			factory.EXPECT().Height().Return(uint64(0), nil).Times(4)
			builder = builder.SetBlockDAO(dao).SetFactory(factory)
		}
		registry := protocol.NewRegistry()
		require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
		require.NoError(pol.ForceRegister(registry))
		cs, err := builder.SetRegistry(registry).BuildForTest()
		require.NoError(err)
		ctx := context.Background()

		require.NoError(cs.Start(ctx))
		grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

		res, err := grpcServer.ReadState(ctx, &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(strconv.FormatUint(test.epoch, 10))},
		})
		require.NoError(err)
		var delegates state.CandidateList
		require.NoError(delegates.Deserialize(res.Data))
		require.Equal(test.numDelegates, len(delegates))
		require.NoError(cs.Stop(ctx))
	}
}

func TestGrpcServer_ReadBlockProducersByEpochIntegrity(t *testing.T) {
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
		builder := chainservice.NewBuilder(cfg)
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
		factory := mock_factory.NewMockFactory(ctrl)
		factory.EXPECT().Height().Return(uint64(0), nil).Times(4)
		registry := protocol.NewRegistry()
		require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
		require.NoError(pol.ForceRegister(registry))
		cs, err := builder.SetRegistry(registry).SetFactory(factory).BuildForTest()
		require.NoError(err)
		grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

		require.NoError(pol.ForceRegister(cs.Registry()))
		res, err := grpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
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

func TestGrpcServer_ReadActiveBlockProducersByEpochIntegrity(t *testing.T) {
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
		builder := chainservice.NewBuilder(cfg)
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
		factory := mock_factory.NewMockFactory(ctrl)
		factory.EXPECT().Height().Return(uint64(0), nil).Times(4)
		registry := protocol.NewRegistry()
		require.NoError(account.NewProtocol(rewarding.DepositGas).Register(registry))
		require.NoError(pol.ForceRegister(registry))
		cs, err := builder.SetRegistry(registry).SetFactory(factory).BuildForTest()
		require.NoError(err)
		grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

		require.NoError(pol.ForceRegister(cs.Registry()))

		res, err := grpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
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

func TestGrpcServer_ReadRollDPoSMetaIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	for _, test := range readRollDPoSMetaTests {
		cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
		require.NoError(err)
		grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))
		res, err := grpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
		})
		require.NoError(err)
		result, err := strconv.ParseUint(string(res.Data), 10, 64)
		require.NoError(err)
		require.Equal(test.result, result)
	}
}

func TestGrpcServer_ReadEpochCtxIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	for _, test := range readEpochCtxTests {
		cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
		require.NoError(err)
		grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

		res, err := grpcServer.ReadState(context.Background(), &iotexapi.ReadStateRequest{
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

func TestGrpcServer_GetEpochMetaIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)
	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	t.Run("failure", func(t *testing.T) {
		grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))
		// failure: epoch number
		_, err = grpcServer.GetEpochMeta(context.Background(), &iotexapi.GetEpochMetaRequest{EpochNumber: 0})
		require.Error(err)
	})
	registry := cs.Registry()
	for _, test := range getEpochMetaTests {
		if test.pollProtocolType == lld {
			pol := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
			require.NoError(pol.ForceRegister(registry))
		} else if test.pollProtocolType == "governanceChainCommittee" {
			cfg.Genesis.PollMode = "governanceMix"
			committee := mock_committee.NewMockCommittee(ctrl)
			mbc := mock_blockchain.NewMockBlockchain(ctrl)
			mbc.EXPECT().Genesis().Return(cfg.Genesis).Times(10)
			indexer, err := poll.NewCandidateIndexer(db.NewMemKVStore())
			require.NoError(err)
			candidates := []*state.Candidate{
				{
					Address:       identityset.Address(1).String(),
					Votes:         big.NewInt(6),
					RewardAddress: identityset.Address(7).String(),
				},
				{
					Address:       identityset.Address(2).String(),
					Votes:         big.NewInt(5),
					RewardAddress: identityset.Address(7).String(),
				},
				{
					Address:       identityset.Address(3).String(),
					Votes:         big.NewInt(4),
					RewardAddress: identityset.Address(7).String(),
				},
				{
					Address:       identityset.Address(4).String(),
					Votes:         big.NewInt(3),
					RewardAddress: identityset.Address(7).String(),
				},
				{
					Address:       identityset.Address(5).String(),
					Votes:         big.NewInt(2),
					RewardAddress: identityset.Address(7).String(),
				},
				{
					Address:       identityset.Address(6).String(),
					Votes:         big.NewInt(1),
					RewardAddress: identityset.Address(7).String(),
				},
			}
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
			pol, _ := poll.NewGovernanceChainCommitteeProtocol(
				indexer,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				cfg.Chain.PollInitialCandidatesInterval,
				slasher)
			require.NoError(pol.ForceRegister(registry))
			committee.EXPECT().HeightByTime(gomock.Any()).DoAndReturn(func(time.Time) (uint64, error) {
				return test.epochData.GravityChainStartHeight, nil
			})
			result := &types.ElectionResult{}
			electionResult := &election.ElectionResult{Timestamp: timestamppb.Now()}
			for i, candidate := range candidates {
				electionResult.Delegates = append(electionResult.Delegates, &election.Candidate{
					Name:            []byte(fmt.Sprintf("name %d", i)),
					Address:         []byte(candidate.Address),
					RewardAddress:   []byte(candidate.RewardAddress),
					OperatorAddress: []byte(identityset.Address(i).String()),
					Score:           candidate.Votes.Bytes(),
				})
				electionResult.DelegateVotes = append(electionResult.DelegateVotes, &election.VoteList{
					Votes: []*election.Vote{{
						Candidate:      candidate.CanName,
						Amount:         candidate.Votes.Bytes(),
						WeightedAmount: candidate.Votes.Bytes(),
						StartTime:      timestamppb.Now(),
						Duration:       durationpb.New(24 * time.Hour),
					}},
				})
			}
			require.NoError(result.FromProtoMsg(electionResult))
			committee.EXPECT().ResultByHeight(gomock.Any()).Return(result, nil)

			mbc.EXPECT().TipHeight().Return(uint64(4)).Times(4)
			mbc.EXPECT().AddSubscriber(gomock.Any()).Return(nil).Times(3)
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
			cs, err = chainservice.NewBuilder(cfg).SetBlockchain(mbc).SetRegistry(registry).BuildForTest()
			require.NoError(err)
			ctx := protocol.WithFeatureWithHeightCtx(
				genesis.WithGenesisContext(
					protocol.WithBlockchainCtx(
						context.Background(),
						protocol.BlockchainCtx{
							Tip: protocol.TipInfo{},
						},
					),
					cfg.Genesis,
				))
			require.NoError(cs.BlockDAO().Start(ctx))
		}
		grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

		require.NoError(cs.ReceiveBlock(nil))
		res, err := grpcServer.GetEpochMeta(context.Background(), &iotexapi.GetEpochMetaRequest{EpochNumber: test.EpochNumber})
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

}

func TestGrpcServer_GetRawBlocksIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	for _, test := range getRawBlocksTest {
		request := &iotexapi.GetRawBlocksRequest{
			StartHeight:  test.startHeight,
			Count:        test.count,
			WithReceipts: test.withReceipts,
		}
		res, err := grpcServer.GetRawBlocks(context.Background(), request)
		require.NoError(err)
		blkInfos := res.Blocks
		require.Equal(test.numBlks, len(blkInfos))
		if test.startHeight == 0 {
			// verify genesis block
			header := blkInfos[0].Block.Header.Core
			require.EqualValues(version.ProtocolVersion, header.Version)
			require.Zero(header.Height)
			ts := timestamppb.New(time.Unix(genesis.Timestamp(), 0))
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
	_, err = grpcServer.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{
		StartHeight:  1,
		Count:        0,
		WithReceipts: true,
	})
	require.Error(err)

	// failure: invalid startHeight
	_, err = grpcServer.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{
		StartHeight:  1000000,
		Count:        10,
		WithReceipts: true,
	})
	require.Error(err)

	// failure: invalid endHeight
	_, err = grpcServer.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{
		StartHeight:  3,
		Count:        1000,
		WithReceipts: true,
	})
	require.Error(err)

}

func TestGrpcServer_GetLogsIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	dbConfig := cfg.DB
	dbConfig.DbPath = t.TempDir() + "/bf.index"
	bfIndexer, err := blockindex.NewBloomfilterIndexer(db.NewBoltDB(dbConfig), cfg.Indexer)
	require.NoError(err)
	defer testutil.CleanupPath(dbConfig.DbPath)
	cs, err := chainservice.NewBuilder(cfg).SetBloomFilterIndexer(bfIndexer).BuildForTest()
	require.NoError(err)

	ctx := context.Background()

	require.NoError(cs.Start(ctx))
	// Add testing blocks
	require.NoError(addTestingBlocks(cs.Blockchain(), cs.ActionPool()))
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

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
		res, err := grpcServer.GetLogs(context.Background(), request)
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
		res, err := grpcServer.GetLogs(context.Background(), request)
		require.NoError(err)
		logs := res.Logs
		require.Equal(1, len(logs))
	}

	// failure: empty request
	_, err = grpcServer.GetLogs(context.Background(), &iotexapi.GetLogsRequest{
		Filter: &iotexapi.LogsFilter{},
	})
	require.Error(err)

	// failure: empty filter
	_, err = grpcServer.GetLogs(context.Background(), &iotexapi.GetLogsRequest{})
	require.Error(err)
}

func TestGrpcServer_GetElectionBucketsIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)

	// failure: no native election
	request := &iotexapi.GetElectionBucketsRequest{
		EpochNum: 0,
	}
	_, err = api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow)).GetElectionBuckets(context.Background(), request)
	require.Error(err)
}

func TestGrpcServer_GetActionByActionHashIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)

	for _, test := range getActionByActionHashTest {
		ret, _, _, _, err := cs.ActionByActionHash(test.h)
		require.NoError(err)
		require.Equal(test.expectedNounce, ret.Envelope.Nonce())
	}
}

func TestGrpcServer_GetTransactionLogByActionHashIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	request := &iotexapi.GetTransactionLogByActionHashRequest{
		ActionHash: hex.EncodeToString(hash.ZeroHash256[:]),
	}
	_, err = grpcServer.GetTransactionLogByActionHash(context.Background(), request)
	require.Error(err)
	sta, ok := status.FromError(err)
	require.Equal(true, ok)
	require.Equal(codes.NotFound, sta.Code())

	for h, log := range implicitLogs {
		request.ActionHash = hex.EncodeToString(h[:])
		res, err := grpcServer.GetTransactionLogByActionHash(context.Background(), request)
		require.NoError(err)
		require.Equal(log.Proto(), res.TransactionLog)
	}

	// check implicit transfer receiver balance
	state, err := accountutil.LoadAccount(cs.StateFactory(), identityset.Address(31))
	require.NoError(err)
	require.Equal(big.NewInt(5), state.Balance)
}

func TestGrpcServer_GetEvmTransfersByBlockHeightIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))
	request := &iotexapi.GetTransactionLogByBlockHeightRequest{}
	for _, test := range getImplicitLogByBlockHeightTest {
		request.BlockHeight = test.height
		res, err := grpcServer.GetTransactionLogByBlockHeight(context.Background(), request)
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

func TestGrpcServer_GetActPoolActionsIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	ctx := context.Background()

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), false)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))
	res, err := grpcServer.GetActPoolActions(ctx, &iotexapi.GetActPoolActionsRequest{})
	require.NoError(err)
	actPool := cs.ActionPool()
	require.Equal(len(actPool.PendingActionMap()[identityset.Address(27).String()]), len(res.Actions))

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

	require.NoError(actPool.Add(ctx, tsf1))
	require.NoError(actPool.Add(ctx, tsf2))
	require.NoError(actPool.Add(ctx, execution1))

	var requests []string
	h1, err := tsf1.Hash()
	require.NoError(err)
	requests = append(requests, hex.EncodeToString(h1[:]))

	res, err = grpcServer.GetActPoolActions(context.Background(), &iotexapi.GetActPoolActionsRequest{})
	require.NoError(err)
	require.Equal(len(actPool.PendingActionMap()[identityset.Address(27).String()]), len(res.Actions))

	res, err = grpcServer.GetActPoolActions(context.Background(), &iotexapi.GetActPoolActionsRequest{ActionHashes: requests})
	require.NoError(err)
	require.Equal(1, len(res.Actions))

	h2, err := tsf2.Hash()
	require.NoError(err)
	requests = append(requests, hex.EncodeToString(h2[:]))
	res, err = grpcServer.GetActPoolActions(context.Background(), &iotexapi.GetActPoolActionsRequest{ActionHashes: requests})
	require.NoError(err)
	require.Equal(2, len(res.Actions))

	h3, err := tsf3.Hash()
	require.NoError(err)
	_, err = grpcServer.GetActPoolActions(context.Background(), &iotexapi.GetActPoolActionsRequest{ActionHashes: []string{hex.EncodeToString(h3[:])}})
	require.Error(err)
}

func TestGrpcServer_GetEstimateGasSpecialIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), true)
	require.NoError(err)

	// deploy self-desturct contract
	contractCode := "608060405234801561001057600080fd5b50336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550610196806100606000396000f3fe608060405234801561001057600080fd5b50600436106100415760003560e01c80632e64cec11461004657806343d726d6146100645780636057361d1461006e575b600080fd5b61004e61008a565b60405161005b9190610124565b60405180910390f35b61006c610094565b005b610088600480360381019061008391906100ec565b6100cd565b005b6000600154905090565b60008054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16ff5b8060018190555050565b6000813590506100e681610149565b92915050565b6000602082840312156100fe57600080fd5b600061010c848285016100d7565b91505092915050565b61011e8161013f565b82525050565b60006020820190506101396000830184610115565b92915050565b6000819050919050565b6101528161013f565b811461015d57600080fd5b5056fea264697066735822122060e7a28baea4232a95074b94b50009d1d7b99302ef6556a1f3ce7f46a49f8cc064736f6c63430008000033"
	contract, err := deployContractV2(cs, identityset.PrivateKey(13), 1, contractCode)

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
	res, err := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow)).EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10777), res.Gas)
}

func TestChainlinkErrIntegrity(t *testing.T) {
	require := require.New(t)

	gethFatal := regexp.MustCompile(`(: |^)(exceeds block gas limit|invalid sender|negative value|oversized data|gas uint64 overflow|intrinsic gas too low|nonce too high)$`)

	tests := []struct {
		testName string
		cfg      func() config.Config
		actions  []*iotextypes.Action
		errRegex *regexp.Regexp
	}{
		{
			"NonceTooLow",
			func() config.Config {
				return newConfig(t)
			},
			[]*iotextypes.Action{testTransferInvalid1Pb},
			regexp.MustCompile(`(: |^)nonce too low$`),
		},
		{
			"TerminallyUnderpriced",
			func() config.Config {
				return newConfig(t)
			},
			[]*iotextypes.Action{testTransferInvalid2Pb},
			regexp.MustCompile(`(: |^)transaction underpriced$`),
		},
		{
			"InsufficientEth",
			func() config.Config {
				return newConfig(t)
			},
			[]*iotextypes.Action{testTransferInvalid3Pb},
			regexp.MustCompile(`(: |^)(insufficient funds for transfer|insufficient funds for gas \* price \+ value|insufficient balance for transfer)$`),
		},

		{
			"NonceTooHigh",
			func() config.Config {
				return newConfig(t)
			},
			[]*iotextypes.Action{testTransferInvalid4Pb},
			gethFatal,
		},
		{
			"TransactionAlreadyInMempool",
			func() config.Config {
				return newConfig(t)
			},
			[]*iotextypes.Action{testTransferPb, testTransferPb},
			regexp.MustCompile(`(: |^)(?i)(known transaction|already known)`),
		},
		{
			"ReplacementTransactionUnderpriced",
			func() config.Config {
				return newConfig(t)
			},
			[]*iotextypes.Action{testTransferPb, testTransferInvalid5Pb},
			regexp.MustCompile(`(: |^)replacement transaction underpriced$`),
		},
		{
			"IntrinsicGasTooLow",
			func() config.Config {
				return newConfig(t)
			},
			[]*iotextypes.Action{testTransferInvalid6Pb},
			gethFatal,
		},
		{
			"NegativeValue",
			func() config.Config {
				return newConfig(t)
			},
			[]*iotextypes.Action{testTransferInvalid7Pb},
			gethFatal,
		},
		{
			"ExceedsBlockGasLimit",
			func() config.Config {
				return newConfig(t)
			},
			[]*iotextypes.Action{testTransferInvalid8Pb},
			gethFatal,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			cs, err := createChainService(test.cfg(), p2p.NewDummyAgent(), true)
			require.NoError(err)
			grpcServer := api.NewGRPCServer(cs, test.cfg().API.Port, test.cfg().API.RangeQueryLimit, uint64(test.cfg().API.TpsWindow))

			for _, action := range test.actions {
				_, err = grpcServer.SendAction(context.Background(), &iotexapi.SendActionRequest{Action: action})
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

func TestGrpcServer_TraceTransactionStructLogsIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	cs, err := createChainService(cfg, p2p.NewDummyAgent(), true)
	require.NoError(err)
	grpcServer := api.NewGRPCServer(cs, cfg.API.Port, cfg.API.RangeQueryLimit, uint64(cfg.API.TpsWindow))

	request := &iotexapi.TraceTransactionStructLogsRequest{
		ActionHash: hex.EncodeToString(hash.ZeroHash256[:]),
	}
	_, err = grpcServer.TraceTransactionStructLogs(context.Background(), request)
	require.Error(err)

	//unsupport type
	request.ActionHash = hex.EncodeToString(transferHash1[:])
	_, err = grpcServer.TraceTransactionStructLogs(context.Background(), request)
	require.Error(err)

	// deploy a contract
	contractCode := "6080604052348015600f57600080fd5b5060de8061001e6000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c8063ee82ac5e14602d575b600080fd5b605660048036036020811015604157600080fd5b8101908080359060200190929190505050606c565b6040518082815260200191505060405180910390f35b60008082409050807f2d93f7749862d33969fb261757410b48065a1bc86a56da5c47820bd063e2338260405160405180910390a28091505091905056fea265627a7a723158200a258cd08ea99ee11aa68c78b6d2bf7ea912615a1e64a81b90a2abca2dd59cfa64736f6c634300050c0032"

	data, _ := hex.DecodeString(contractCode)
	ex1, err := action.SignedExecution(action.EmptyAddress, identityset.PrivateKey(13), 1, big.NewInt(0), 500000, big.NewInt(testutil.TestGasPriceInt64), data)
	require.NoError(err)
	cs.ActionPool().Add(context.Background(), ex1)
	require.NoError(err)
	bc := cs.Blockchain()
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	bc.CommitBlock(blk)
	require.NoError(err)
	cs.ActionPool().Reset()
	ex1Hash, _ := ex1.Hash()
	request.ActionHash = hex.EncodeToString(ex1Hash[:])
	ret, err := grpcServer.TraceTransactionStructLogs(context.Background(), request)
	require.NoError(err)
	require.Equal(len(ret.StructLogs), 17)
	log := ret.StructLogs[0]
	require.Equal(log.Depth, int32(1))
	require.Equal(log.Gas, uint64(0x4bc1c0))
	require.Equal(log.GasCost, uint64(0x3))
	require.Equal(log.Op, uint64(0x60))
	require.Equal(log.OpName, "PUSH1")
}
