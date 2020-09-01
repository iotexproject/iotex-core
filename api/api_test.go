// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/hex"
	"math"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/go-pkgs/hash"
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
	"github.com/iotexproject/iotex-core/gasstation"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/testutil"
)

const lld = "lifeLongDelegates"

var (
	testTransfer, _ = testutil.SignedTransfer(identityset.Address(28).String(),
		identityset.PrivateKey(28), 3, big.NewInt(10), []byte{}, testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64))

	testTransferHash = testTransfer.Hash()
	testTransferPb   = testTransfer.Proto()

	testExecution, _ = testutil.SignedExecution(identityset.Address(29).String(),
		identityset.PrivateKey(29), 1, big.NewInt(0), testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64), []byte{})

	testExecutionHash = testExecution.Hash()
	testExecutionPb   = testExecution.Proto()

	testTransfer1, _ = testutil.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(27), 1,
		big.NewInt(10), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	transferHash1    = testTransfer1.Hash()
	testTransfer2, _ = testutil.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(30), 5,
		big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	transferHash2 = testTransfer2.Hash()

	testExecution1, _ = testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(30), 6,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	executionHash1 = testExecution1.Hash()

	testExecution2, _ = testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(30), 6,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	executionHash2 = testExecution2.Hash()

	testExecution3, _ = testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 2,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	executionHash3 = testExecution3.Hash()

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
			"9999999999999999999999999991",
			1,
			6,
			2,
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
		start   uint64
		count   uint64
		numBlks int
	}{
		{
			1,
			4,
			4,
		},
		{
			2,
			5,
			3,
		},
		{
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
			hex.EncodeToString(executionHash2[:]),
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
		execHash   string
		callerAddr string
		retValue   string
	}{
		{
			hex.EncodeToString(executionHash2[:]),
			identityset.Address(30).String(),
			"",
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
	}

	getLogsTest = []struct {
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
	}

	getImplicitTransfersByActionHashTest = []struct {
		// Arguments
		actHash hash.Hash256
		// Expected Values
		numEvmTransfer uint64
		amount         [][]byte
		from           []string
		to             []string
	}{
		{
			actHash:        testExecution.Hash(),
			numEvmTransfer: uint64(1),
			amount:         [][]byte{big.NewInt(3).Bytes()},
			from:           []string{identityset.Address(30).String()},
			to:             []string{identityset.Address(29).String()},
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
)

func TestServer_GetAccount(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, true)
	require.NoError(err)

	// success
	for _, test := range getAccountTests {
		request := &iotexapi.GetAccountRequest{Address: test.in}
		res, err := svr.GetAccount(context.Background(), request)
		require.NoError(err)
		accountMeta := res.AccountMeta
		require.Equal(test.address, accountMeta.Address)
		require.Equal(test.balance, accountMeta.Balance)
		require.Equal(test.nonce, accountMeta.Nonce)
		require.Equal(test.pendingNonce, accountMeta.PendingNonce)
		require.Equal(test.numActions, accountMeta.NumActions)
	}
	// failure
	_, err = svr.GetAccount(context.Background(), &iotexapi.GetAccountRequest{})
	require.Error(err)
}

func TestServer_GetActions(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getActionsTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByIndex{
				ByIndex: &iotexapi.GetActionsByIndexRequest{
					Start: test.start,
					Count: test.count,
				},
			},
		}
		res, err := svr.GetActions(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
			continue
		}
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
	}
}

func TestServer_GetAction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, true)
	require.NoError(err)

	for _, test := range getActionTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByHash{
				ByHash: &iotexapi.GetActionByHashRequest{
					ActionHash:   test.in,
					CheckPending: test.checkPending,
				},
			},
		}
		res, err := svr.GetActions(context.Background(), request)
		require.NoError(err)
		require.Equal(1, len(res.ActionInfo))
		act := res.ActionInfo[0]
		require.Equal(test.nonce, act.Action.GetCore().GetNonce())
		require.Equal(test.senderPubKey, hex.EncodeToString(act.Action.SenderPubKey))
		if !test.checkPending {
			blk, err := svr.dao.GetBlockByHeight(test.blkNumber)
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
}

func TestServer_GetActionsByAddress(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

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
		res, err := svr.GetActions(context.Background(), request)
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
			prevRes, err := svr.GetActions(context.Background(), request)
			require.NoError(err)
			require.True(prevRes.ActionInfo[len(prevRes.ActionInfo)-1].Timestamp.GetSeconds() <= res.ActionInfo[0].Timestamp.GetSeconds())
		}
	}
}

func TestServer_GetUnconfirmedActionsByAddress(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, true)
	require.NoError(err)

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
		res, err := svr.GetActions(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
			continue
		}
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
		require.Equal(test.address, res.ActionInfo[0].Sender)
	}
}

func TestServer_GetActionsByBlock(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

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
		res, err := svr.GetActions(context.Background(), request)
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

func TestServer_GetBlockMetas(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)
	require.NotNil(svr.indexer)

	for _, test := range getBlockMetasTests {
		request := &iotexapi.GetBlockMetasRequest{
			Lookup: &iotexapi.GetBlockMetasRequest_ByIndex{
				ByIndex: &iotexapi.GetBlockMetasByIndexRequest{
					Start: test.start,
					Count: test.count,
				},
			},
		}
		res, err := svr.GetBlockMetas(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
			continue
		}
		require.NoError(err)
		require.Equal(test.numBlks, len(res.BlkMetas))
		var prevBlkPb *iotextypes.BlockMeta
		for _, blkPb := range res.BlkMetas {
			if prevBlkPb != nil {
				require.True(blkPb.Height > prevBlkPb.Height)
			}
			prevBlkPb = blkPb
		}
	}
}

func TestServer_GetBlockMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getBlockMetaTests {
		header, err := svr.bc.BlockHeaderByHeight(test.blkHeight)
		require.NoError(err)
		blkHash := header.HashBlock()
		request := &iotexapi.GetBlockMetasRequest{
			Lookup: &iotexapi.GetBlockMetasRequest_ByHash{
				ByHash: &iotexapi.GetBlockMetaByHashRequest{
					BlkHash: hex.EncodeToString(blkHash[:]),
				},
			},
		}
		res, err := svr.GetBlockMetas(context.Background(), request)
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

func TestServer_GetChainMeta(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var pol poll.Protocol
	for _, test := range getChainMetaTests {
		cfg := newConfig(t)
		if test.pollProtocolType == lld {
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else if test.pollProtocolType == "governanceChainCommittee" {
			committee := mock_committee.NewMockCommittee(ctrl)
			slasher, _ := poll.NewSlasher(
				&cfg.Genesis,
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
		svr, err := createServer(cfg, false)
		require.NoError(err)
		if pol != nil {
			require.NoError(pol.ForceRegister(svr.registry))
		}
		if test.emptyChain {
			mbc := mock_blockchain.NewMockBlockchain(ctrl)
			mbc.EXPECT().TipHeight().Return(uint64(0)).Times(1)
			svr.bc = mbc
		}
		res, err := svr.GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{})
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

func TestServer_SendAction(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	chain := mock_blockchain.NewMockBlockchain(ctrl)
	ap := mock_actpool.NewMockActPool(ctrl)
	broadcastHandlerCount := 0
	svr := Server{bc: chain, ap: ap, broadcastHandler: func(_ context.Context, _ uint32, _ proto.Message) error {
		broadcastHandlerCount++
		return nil
	}}

	chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	ap.EXPECT().Add(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	for i, test := range sendActionTests {
		request := &iotexapi.SendActionRequest{Action: test.actionPb}
		res, err := svr.SendAction(context.Background(), request)
		require.NoError(err)
		require.Equal(i+1, broadcastHandlerCount)
		require.Equal(test.actionHash, res.ActionHash)
	}
}

func TestServer_GetReceiptByAction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getReceiptByActionTests {
		request := &iotexapi.GetReceiptByActionRequest{ActionHash: test.in}
		res, err := svr.GetReceiptByAction(context.Background(), request)
		require.NoError(err)
		receiptPb := res.ReceiptInfo.Receipt
		require.Equal(test.status, receiptPb.Status)
		require.Equal(test.blkHeight, receiptPb.BlkHeight)
		require.NotEqual(hash.ZeroHash256, res.ReceiptInfo.BlkHash)
	}
}

func TestServer_ReadContract(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range readContractTests {
		hash, err := hash.HexStringToHash256(test.execHash)
		require.NoError(err)
		ai, err := svr.indexer.GetActionIndex(hash[:])
		require.NoError(err)
		exec, err := svr.dao.GetActionByActionHash(hash, ai.BlockHeight())
		require.NoError(err)
		request := &iotexapi.ReadContractRequest{
			Execution:     exec.Proto().GetCore().GetExecution(),
			CallerAddress: test.callerAddr,
		}

		res, err := svr.ReadContract(context.Background(), request)
		require.NoError(err)
		require.Equal(test.retValue, res.Data)
	}
}

func TestServer_SuggestGasPrice(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	for _, test := range suggestGasPriceTests {
		cfg.API.GasStation.DefaultGas = test.defaultGasPrice
		svr, err := createServer(cfg, false)
		require.NoError(err)
		res, err := svr.SuggestGasPrice(context.Background(), &iotexapi.SuggestGasPriceRequest{})
		require.NoError(err)
		require.Equal(test.suggestedGasPrice, res.GasPrice)
	}
}

func TestServer_EstimateGasForAction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range estimateGasForActionTests {
		hash, err := hash.HexStringToHash256(test.actionHash)
		require.NoError(err)
		ai, err := svr.indexer.GetActionIndex(hash[:])
		require.NoError(err)
		act, err := svr.dao.GetActionByActionHash(hash, ai.BlockHeight())
		require.NoError(err)
		request := &iotexapi.EstimateGasForActionRequest{Action: act.Proto()}

		res, err := svr.EstimateGasForAction(context.Background(), request)
		require.NoError(err)
		require.Equal(test.estimatedGas, res.Gas)
	}
}

func TestServer_EstimateActionGasConsumption(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	svr, err := createServer(cfg, false)
	require.NoError(err)

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
	res, err := svr.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = svr.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = svr.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = svr.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = svr.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = svr.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = svr.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = svr.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = svr.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = svr.EstimateActionGasConsumption(context.Background(), request)
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
	res, err = svr.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10000), res.Gas)

	// Case X: test for action nil
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action:        nil,
		CallerAddress: identityset.Address(0).String(),
	}
	res, err = svr.EstimateActionGasConsumption(context.Background(), request)
	require.Error(err)
}

func TestServer_ReadUnclaimedBalance(t *testing.T) {
	cfg := newConfig(t)
	cfg.Consensus.Scheme = config.RollDPoSScheme
	svr, err := createServer(cfg, false)
	require.NoError(t, err)

	for _, test := range readUnclaimedBalanceTests {
		out, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(test.addr)},
		})
		if test.returnErr {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		val, ok := big.NewInt(0).SetString(string(out.Data), 10)
		require.True(t, ok)
		assert.Equal(t, test.balance, val)
	}
}

func TestServer_TotalBalance(t *testing.T) {
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(t, err)

	out, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte("TotalBalance"),
		Arguments:  nil,
	})
	require.NoError(t, err)
	val, ok := big.NewInt(0).SetString(string(out.Data), 10)
	require.True(t, ok)
	assert.Equal(t, unit.ConvertIotxToRau(200000000), val)
}

func TestServer_AvailableBalance(t *testing.T) {
	cfg := newConfig(t)
	cfg.Consensus.Scheme = config.RollDPoSScheme
	svr, err := createServer(cfg, false)
	require.NoError(t, err)

	out, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte("AvailableBalance"),
		Arguments:  nil,
	})
	require.NoError(t, err)
	val, ok := big.NewInt(0).SetString(string(out.Data), 10)
	require.True(t, ok)
	assert.Equal(t, unit.ConvertIotxToRau(199999936), val)
}

func TestServer_ReadCandidatesByEpoch(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
				&cfg.Genesis,
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
		svr, err := createServer(cfg, false)
		require.NoError(err)
		require.NoError(pol.ForceRegister(svr.registry))

		res, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
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

func TestServer_ReadBlockProducersByEpoch(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
				&cfg.Genesis,
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
		svr, err := createServer(cfg, false)
		require.NoError(err)
		require.NoError(pol.ForceRegister(svr.registry))
		res, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
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

func TestServer_ReadActiveBlockProducersByEpoch(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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
				&cfg.Genesis,
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
		svr, err := createServer(cfg, false)
		require.NoError(err)
		require.NoError(pol.ForceRegister(svr.registry))

		res, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
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

func TestServer_ReadRollDPoSMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	for _, test := range readRollDPoSMetaTests {
		svr, err := createServer(cfg, false)
		require.NoError(err)
		res, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
		})
		require.NoError(err)
		result, err := strconv.ParseUint(string(res.Data), 10, 64)
		require.NoError(err)
		require.Equal(test.result, result)
	}
}

func TestServer_ReadEpochCtx(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	for _, test := range readEpochCtxTests {
		svr, err := createServer(cfg, false)
		require.NoError(err)
		res, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
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

func TestServer_GetEpochMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svr, err := createServer(cfg, false)
	require.NoError(err)
	for _, test := range getEpochMetaTests {
		if test.pollProtocolType == lld {
			pol := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
			require.NoError(pol.ForceRegister(svr.registry))
		} else if test.pollProtocolType == "governanceChainCommittee" {
			committee := mock_committee.NewMockCommittee(ctrl)
			mbc := mock_blockchain.NewMockBlockchain(ctrl)
			indexer, err := poll.NewCandidateIndexer(db.NewMemKVStore())
			require.NoError(err)
			slasher, _ := poll.NewSlasher(
				&cfg.Genesis,
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
			require.NoError(pol.ForceRegister(svr.registry))
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
			svr.bc = mbc
		}
		res, err := svr.GetEpochMeta(context.Background(), &iotexapi.GetEpochMetaRequest{EpochNumber: test.EpochNumber})
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

func TestServer_GetRawBlocks(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getRawBlocksTest {
		request := &iotexapi.GetRawBlocksRequest{
			StartHeight:  test.startHeight,
			Count:        test.count,
			WithReceipts: test.withReceipts,
		}
		res, err := svr.GetRawBlocks(context.Background(), request)
		require.NoError(err)
		blkInfos := res.Blocks
		require.Equal(test.numBlks, len(blkInfos))
		var numActions int
		var numReceipts int
		for _, blkInfo := range blkInfos {
			numActions += len(blkInfo.Block.Body.Actions)
			numReceipts += len(blkInfo.Receipts)
		}
		require.Equal(test.numActions, numActions)
		require.Equal(test.numReceipts, numReceipts)
	}
}

func TestServer_GetLogs(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getLogsTest {
		request := &iotexapi.GetLogsRequest{
			Filter: &iotexapi.LogsFilter{
				Address: test.address,
				Topics:  test.topics,
			},
			Lookup: &iotexapi.GetLogsRequest_ByRange{
				ByRange: &iotexapi.GetLogsByRange{
					FromBlock: test.fromBlock,
					Count:     test.count,
				},
			},
		}
		res, err := svr.GetLogs(context.Background(), request)
		require.NoError(err)
		logs := res.Logs
		require.Equal(test.numLogs, len(logs))
	}
}

func TestServer_GetTransactionLogByActionHash(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	request := &iotexapi.GetTransactionLogByActionHashRequest{
		ActionHash: hex.EncodeToString(hash.ZeroHash256[:]),
	}
	_, err = svr.GetTransactionLogByActionHash(context.Background(), request)
	require.Error(err)
	sta, ok := status.FromError(err)
	require.Equal(true, ok)
	require.Equal(codes.NotFound, sta.Code())

	for h, log := range implicitLogs {
		request.ActionHash = hex.EncodeToString(h[:])
		res, err := svr.GetTransactionLogByActionHash(context.Background(), request)
		require.NoError(err)
		require.Equal(log.Proto(), res.TransactionLog)
	}

	// check implicit transfer receiver balance
	state, err := accountutil.LoadAccount(svr.sf, hash.BytesToHash160(identityset.Address(31).Bytes()))
	require.NoError(err)
	require.Equal(big.NewInt(5), state.Balance)
}

func TestServer_GetEvmTransfersByBlockHeight(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	request := &iotexapi.GetTransactionLogByBlockHeightRequest{}
	for _, test := range getImplicitLogByBlockHeightTest {
		request.BlockHeight = test.height
		res, err := svr.GetTransactionLogByBlockHeight(context.Background(), request)
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
	addr0 := identityset.Address(27).String()
	priKey0 := identityset.PrivateKey(27)
	addr1 := identityset.Address(28).String()
	priKey1 := identityset.PrivateKey(28)
	addr2 := identityset.Address(29).String()
	addr3 := identityset.Address(30).String()
	priKey3 := identityset.PrivateKey(30)
	addr4 := identityset.Address(31).String()
	// Add block 1
	// Producer transfer--> C
	tsf, err := testutil.SignedTransfer(addr3, priKey0, 1, big.NewInt(10), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	implicitLogs[tsf.Hash()] = block.NewTransactionLog(tsf.Hash(),
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "10", addr0, addr3)},
	)

	blk1Time := testutil.TimestampNow()
	if err := ap.Add(context.Background(), tsf); err != nil {
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
		selp, err := testutil.SignedTransfer(recipient, priKey3, uint64(i+1), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		if err != nil {
			return err
		}
		if err := ap.Add(context.Background(), selp); err != nil {
			return err
		}
		implicitLogs[selp.Hash()] = block.NewTransactionLog(selp.Hash(),
			[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "1", addr3, recipient)},
		)
	}
	selp, err := testutil.SignedTransfer(addr3, priKey3, uint64(5), big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	implicitLogs[selp.Hash()] = block.NewTransactionLog(selp.Hash(),
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "2", addr3, addr3)},
	)
	if err := ap.Add(context.Background(), selp); err != nil {
		return err
	}
	execution1, err := testutil.SignedExecution(addr4, priKey3, 6,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}
	implicitLogs[execution1.Hash()] = block.NewTransactionLog(
		execution1.Hash(),
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, "1", addr3, addr4)},
	)
	if err := ap.Add(context.Background(), execution1); err != nil {
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
	tsf1, err := testutil.SignedTransfer(addr3, priKey3, uint64(7), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	implicitLogs[tsf1.Hash()] = block.NewTransactionLog(tsf1.Hash(),
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "1", addr3, addr3)},
	)
	if err := ap.Add(context.Background(), tsf1); err != nil {
		return err
	}
	tsf2, err := testutil.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	implicitLogs[tsf2.Hash()] = block.NewTransactionLog(tsf2.Hash(),
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_NATIVE_TRANSFER, "1", addr1, addr1)},
	)
	if err := ap.Add(context.Background(), tsf2); err != nil {
		return err
	}
	execution1, err = testutil.SignedExecution(addr4, priKey3, 8,
		big.NewInt(2), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}
	implicitLogs[execution1.Hash()] = block.NewTransactionLog(
		execution1.Hash(),
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, "2", addr3, addr4)},
	)
	if err := ap.Add(context.Background(), execution1); err != nil {
		return err
	}
	execution2, err := testutil.SignedExecution(addr4, priKey1, 2,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}
	implicitLogs[execution2.Hash()] = block.NewTransactionLog(
		execution2.Hash(),
		[]*block.TokenTxRecord{block.NewTokenTxRecord(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, "1", addr1, addr4)},
	)
	if err := ap.Add(context.Background(), execution2); err != nil {
		return err
	}
	if blk, err = bc.MintNewBlock(blk1Time.Add(time.Second * 3)); err != nil {
		return err
	}
	h = blk.HashBlock()
	blkHash[4] = hex.EncodeToString(h[:])
	return bc.CommitBlock(blk)
}

func addActsToActPool(ctx context.Context, ap actpool.ActPool) error {
	// Producer transfer--> A
	tsf1, err := testutil.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 2, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// Producer transfer--> P
	tsf2, err := testutil.SignedTransfer(identityset.Address(27).String(), identityset.PrivateKey(27), 3, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// Producer transfer--> B
	tsf3, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 4, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// Producer exec--> D
	execution1, err := testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(27), 5,
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

func setupChain(cfg config.Config) (blockchain.Blockchain, blockdao.BlockDAO, blockindex.Indexer, factory.Factory, actpool.ActPool, *protocol.Registry, error) {
	cfg.Chain.ProducerPrivKey = hex.EncodeToString(identityset.PrivateKey(0).Bytes())
	registry := protocol.NewRegistry()
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	ap, err := setupActPool(sf, cfg.ActPool)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	cfg.Genesis.InitBalanceMap[identityset.Address(28).String()] = unit.ConvertIotxToRau(10000000000).String()
	// create indexer
	indexer, err := blockindex.NewIndexer(db.NewMemKVStore(), cfg.Genesis.Hash())
	if err != nil {
		return nil, nil, nil, nil, nil, nil, errors.New("failed to create indexer")
	}
	// create BlockDAO
	dao := blockdao.NewBlockDAOInMemForTest([]blockdao.BlockIndexer{sf, indexer}, cfg.DB)
	if dao == nil {
		return nil, nil, nil, nil, nil, nil, errors.New("failed to create blockdao")
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
		return nil, nil, nil, nil, nil, nil, errors.New("failed to create blockchain")
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
	r := rewarding.NewProtocol(0, 0)

	if err := rolldposProtocol.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	if err := acc.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	if err := evm.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	if err := r.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	if err := p.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	return bc, dao, indexer, sf, ap, registry, nil
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

func createServer(cfg config.Config, needActPool bool) (*Server, error) {
	// TODO (zhi): revise
	bc, dao, indexer, sf, ap, registry, err := setupChain(cfg)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	// Start blockchain
	if err := bc.Start(ctx); err != nil {
		return nil, err
	}
	// Add testing blocks
	if err := addTestingBlocks(bc, ap); err != nil {
		return nil, err
	}

	if needActPool {
		// Add actions to actpool
		ctx := protocol.WithRegistry(context.Background(), registry)
		if err := addActsToActPool(ctx, ap); err != nil {
			return nil, err
		}
	}

	svr := &Server{
		bc:             bc,
		sf:             sf,
		dao:            dao,
		indexer:        indexer,
		ap:             ap,
		cfg:            cfg,
		gs:             gasstation.NewGasStation(bc, sf.SimulateExecution, dao, cfg.API),
		registry:       registry,
		hasActionIndex: true,
	}

	return svr, nil
}
