// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/hex"
	"io/ioutil"
	"math/big"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
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
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/gasstation"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_dispatcher"
	"github.com/iotexproject/iotex-core/test/mock/mock_factory"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
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
			11,
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
			0,
			5,
			5,
		},
		{
			1,
			0,
			0,
			0,
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
	}{
		{
			2,
			7,
			"6",
		},
		{
			4,
			5,
			"2",
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
			5,
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
			15,
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
			action.SuccessReceiptStatus,
			1,
		},
		{
			hex.EncodeToString(transferHash2[:]),
			action.SuccessReceiptStatus,
			2,
		},
		{
			hex.EncodeToString(executionHash2[:]),
			action.SuccessReceiptStatus,
			2,
		},
		{
			hex.EncodeToString(executionHash3[:]),
			action.SuccessReceiptStatus,
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
			protocolID: rewarding.ProtocolID,
			methodName: "UnclaimedBalance",
			addr:       identityset.Address(0).String(),
			returnErr:  false,
			balance:    unit.ConvertIotxToRau(64), // 4 block * 36 IOTX reward by default = 144 IOTX
		},
		{
			protocolID: rewarding.ProtocolID,
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
			protocolID: rewarding.ProtocolID,
			methodName: "Wrong Method",
			addr:       identityset.Address(27).String(),
			returnErr:  true,
		},
	}

	readDelegatesByEpochTests = []struct {
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
			methodName:   "DelegatesByEpoch",
			epoch:        1,
			numDelegates: 3,
		},
		{
			protocolID:   "poll",
			protocolType: "governanceChainCommittee",
			methodName:   "DelegatesByEpoch",
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
			4,
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
)

func TestServer_GetAccount(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

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
	cfg := newConfig()

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
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
	}
}

func TestServer_GetAction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

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
			blk, err := svr.bc.GetBlockByHeight(test.blkNumber)
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
	cfg := newConfig()

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
		if test.numActions > 0 {
			require.Equal(test.address, res.ActionInfo[0].Sender)
		}
	}
}

func TestServer_GetUnconfirmedActionsByAddress(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

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
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
		if test.numActions > 0 {
			require.Equal(test.address, res.ActionInfo[0].Sender)
		}
	}
}

func TestServer_GetActionsByBlock(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getActionsByBlockTests {
		header, err := svr.bc.BlockHeaderByHeight(test.blkHeight)
		require.NoError(err)
		blkHash := header.HashBlock()
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByBlk{
				ByBlk: &iotexapi.GetActionsByBlockRequest{
					BlkHash: hex.EncodeToString(blkHash[:]),
					Start:   test.start,
					Count:   test.count,
				},
			},
		}
		res, err := svr.GetActions(context.Background(), request)
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
		if test.numActions > 0 {
			require.Equal(test.blkHeight, res.ActionInfo[0].BlkHeight)
		}
	}
}

func TestServer_GetBlockMetas(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	svr, err := createServer(cfg, false)
	require.NoError(err)

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
	cfg := newConfig()

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
		require.Equal(test.numActions, blkPb.NumActions)
		require.Equal(test.transferAmount, blkPb.TransferAmount)
	}
}

func TestServer_GetChainMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var pol poll.Protocol
	for _, test := range getChainMetaTests {
		if test.pollProtocolType == lld {
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else if test.pollProtocolType == "governanceChainCommittee" {
			committee := mock_committee.NewMockCommittee(ctrl)
			pol, _ = poll.NewGovernanceChainCommitteeProtocol(
				nil,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				func(uint64) uint64 { return 1 },
				func(uint64) uint64 { return 1 },
				cfg.Genesis.NumCandidateDelegates,
				cfg.Genesis.NumDelegates,
				cfg.Chain.PollInitialCandidatesInterval,
			)
			committee.EXPECT().HeightByTime(gomock.Any()).Return(test.epoch.GravityChainStartHeight, nil)
		}

		cfg.API.TpsWindow = test.tpsWindow
		svr, err := createServer(cfg, false)
		require.NoError(err)
		if pol != nil {
			require.NoError(svr.registry.ForceRegister(poll.ProtocolID, pol))
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
	mDp := mock_dispatcher.NewMockDispatcher(ctrl)
	broadcastHandlerCount := 0
	svr := Server{bc: chain, dp: mDp, broadcastHandler: func(_ context.Context, _ uint32, _ proto.Message) error {
		broadcastHandlerCount++
		return nil
	}}

	chain.EXPECT().ChainID().Return(uint32(1)).Times(4)
	mDp.EXPECT().HandleBroadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)

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
	cfg := newConfig()

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
	cfg := newConfig()

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range readContractTests {
		hash, err := hash.HexStringToHash256(test.execHash)
		require.NoError(err)
		exec, err := svr.bc.GetActionByActionHash(hash)
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
	cfg := newConfig()

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
	cfg := newConfig()

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range estimateGasForActionTests {
		hash, err := hash.HexStringToHash256(test.actionHash)
		require.NoError(err)
		act, err := svr.bc.GetActionByActionHash(hash)
		require.NoError(err)
		request := &iotexapi.EstimateGasForActionRequest{Action: act.Proto()}

		res, err := svr.EstimateGasForAction(context.Background(), request)
		require.NoError(err)
		require.Equal(test.estimatedGas, res.Gas)
	}
}

func TestServer_ReadUnclaimedBalance(t *testing.T) {
	cfg := newConfig()
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
	cfg := newConfig()

	svr, err := createServer(cfg, false)
	require.NoError(t, err)

	out, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte(rewarding.ProtocolID),
		MethodName: []byte("TotalBalance"),
		Arguments:  nil,
	})
	require.NoError(t, err)
	val, ok := big.NewInt(0).SetString(string(out.Data), 10)
	require.True(t, ok)
	assert.Equal(t, unit.ConvertIotxToRau(1200000000), val)
}

func TestServer_AvailableBalance(t *testing.T) {
	cfg := newConfig()
	cfg.Consensus.Scheme = config.RollDPoSScheme
	svr, err := createServer(cfg, false)
	require.NoError(t, err)

	out, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte(rewarding.ProtocolID),
		MethodName: []byte("AvailableBalance"),
		Arguments:  nil,
	})
	require.NoError(t, err)
	val, ok := big.NewInt(0).SetString(string(out.Data), 10)
	require.True(t, ok)
	assert.Equal(t, unit.ConvertIotxToRau(1199999936), val)
}

func TestServer_ReadDelegatesByEpoch(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mbc := mock_blockchain.NewMockBlockchain(ctrl)
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
	mbc.EXPECT().CandidatesByHeight(gomock.Any()).Return(candidates, nil).Times(1)

	for _, test := range readDelegatesByEpochTests {
		var pol poll.Protocol
		if test.protocolType == lld {
			cfg.Genesis.Delegates = delegates
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else {
			pol, _ = poll.NewGovernanceChainCommitteeProtocol(
				mbc,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				func(uint64) uint64 { return 1 },
				func(uint64) uint64 { return 1 },
				cfg.Genesis.NumCandidateDelegates,
				cfg.Genesis.NumDelegates,
				cfg.Chain.PollInitialCandidatesInterval,
			)
		}
		svr, err := createServer(cfg, false)
		require.NoError(err)
		require.NoError(svr.registry.ForceRegister(poll.ProtocolID, pol))

		res, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{byteutil.Uint64ToBytes(test.epoch)},
		})
		require.NoError(err)
		var delegates state.CandidateList
		require.NoError(delegates.Deserialize(res.Data))
		require.Equal(test.numDelegates, len(delegates))
	}
}

func TestServer_ReadBlockProducersByEpoch(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mbc := mock_blockchain.NewMockBlockchain(ctrl)
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
	mbc.EXPECT().CandidatesByHeight(gomock.Any()).Return(candidates, nil).Times(2)

	for _, test := range readBlockProducersByEpochTests {
		var pol poll.Protocol
		if test.protocolType == lld {
			cfg.Genesis.Delegates = delegates
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else {
			pol, _ = poll.NewGovernanceChainCommitteeProtocol(
				mbc,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				func(uint64) uint64 { return 1 },
				func(uint64) uint64 { return 1 },
				test.numCandidateDelegates,
				cfg.Genesis.NumDelegates,
				cfg.Chain.PollInitialCandidatesInterval,
			)
		}
		svr, err := createServer(cfg, false)
		require.NoError(err)
		require.NoError(svr.registry.ForceRegister(poll.ProtocolID, pol))

		res, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{byteutil.Uint64ToBytes(test.epoch)},
		})
		require.NoError(err)
		var blockProducers state.CandidateList
		require.NoError(blockProducers.Deserialize(res.Data))
		require.Equal(test.numBlockProducers, len(blockProducers))
	}
}

func TestServer_ReadActiveBlockProducersByEpoch(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mbc := mock_blockchain.NewMockBlockchain(ctrl)
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
	mbc.EXPECT().CandidatesByHeight(gomock.Any()).Return(candidates, nil).Times(2)

	for _, test := range readActiveBlockProducersByEpochTests {
		var pol poll.Protocol
		if test.protocolType == lld {
			cfg.Genesis.Delegates = delegates
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else {
			pol, _ = poll.NewGovernanceChainCommitteeProtocol(
				mbc,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				func(uint64) uint64 { return 1 },
				func(uint64) uint64 { return 1 },
				cfg.Genesis.NumCandidateDelegates,
				test.numDelegates,
				cfg.Chain.PollInitialCandidatesInterval,
			)
		}
		svr, err := createServer(cfg, false)
		require.NoError(err)
		require.NoError(svr.registry.ForceRegister(poll.ProtocolID, pol))

		res, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{byteutil.Uint64ToBytes(test.epoch)},
		})
		require.NoError(err)
		var activeBlockProducers state.CandidateList
		require.NoError(activeBlockProducers.Deserialize(res.Data))
		require.Equal(test.numActiveBlockProducers, len(activeBlockProducers))
	}
}

func TestServer_GetEpochMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, test := range getEpochMetaTests {
		svr, err := createServer(cfg, false)
		require.NoError(err)
		if test.pollProtocolType == lld {
			pol := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
			require.NoError(svr.registry.ForceRegister(poll.ProtocolID, pol))
		} else if test.pollProtocolType == "governanceChainCommittee" {
			committee := mock_committee.NewMockCommittee(ctrl)
			mbc := mock_blockchain.NewMockBlockchain(ctrl)
			msf := mock_factory.NewMockFactory(ctrl)
			pol, _ := poll.NewGovernanceChainCommitteeProtocol(
				mbc,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				func(uint64) uint64 { return 1 },
				func(uint64) uint64 { return 1 },
				cfg.Genesis.NumCandidateDelegates,
				cfg.Genesis.NumDelegates,
				cfg.Chain.PollInitialCandidatesInterval,
			)
			require.NoError(svr.registry.ForceRegister(poll.ProtocolID, pol))
			committee.EXPECT().HeightByTime(gomock.Any()).Return(test.epochData.GravityChainStartHeight, nil)
			mbc.EXPECT().TipHeight().Return(uint64(4)).Times(2)
			mbc.EXPECT().GetFactory().Return(msf).Times(2)
			msf.EXPECT().NewWorkingSet().Return(nil, nil).Times(2)

			candidates := []*state.Candidate{
				{
					Address:       "address1",
					Votes:         big.NewInt(6),
					RewardAddress: "rewardAddress",
				},
				{
					Address:       "address2",
					Votes:         big.NewInt(5),
					RewardAddress: "rewardAddress",
				},
				{
					Address:       "address3",
					Votes:         big.NewInt(4),
					RewardAddress: "rewardAddress",
				},
				{
					Address:       "address4",
					Votes:         big.NewInt(3),
					RewardAddress: "rewardAddress",
				},
				{
					Address:       "address5",
					Votes:         big.NewInt(2),
					RewardAddress: "rewardAddress",
				},
				{
					Address:       "address6",
					Votes:         big.NewInt(1),
					RewardAddress: "rewardAddress",
				},
			}
			blksPerDelegate := map[string]uint64{
				"address1": uint64(1),
				"address2": uint64(1),
				"address3": uint64(1),
				"address4": uint64(1),
			}
			mbc.EXPECT().ProductivityByEpoch(test.EpochNumber).Return(uint64(4), blksPerDelegate, nil).Times(1)
			mbc.EXPECT().CandidatesByHeight(uint64(1)).
				Return(candidates, nil).Times(1)
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
	cfg := newConfig()

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

func addProducerToFactory(sf factory.Factory) error {
	ws, err := sf.NewWorkingSet()
	if err != nil {
		return err
	}
	if _, err = accountutil.LoadOrCreateAccount(
		ws,
		identityset.Address(27).String(),
		unit.ConvertIotxToRau(10000000000),
	); err != nil {
		return err
	}
	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			Producer: identityset.Address(27),
			GasLimit: gasLimit,
		})
	if _, err = ws.RunActions(ctx, 0, nil); err != nil {
		return err
	}
	return sf.Commit(ws)
}

func addTestingBlocks(bc blockchain.Blockchain) error {
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

	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[addr0] = []action.SealedEnvelope{tsf}
	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 2
	// Charlie transfer--> A, B, D, P
	// Charlie transfer--> C
	// Charlie exec--> D
	recipients := []string{addr1, addr2, addr4, addr0}
	selps := make([]action.SealedEnvelope, 0)
	for i, recipient := range recipients {
		selp, err := testutil.SignedTransfer(recipient, priKey3, uint64(i+1), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		if err != nil {
			return err
		}
		selps = append(selps, selp)
	}
	selp, err := testutil.SignedTransfer(addr3, priKey3, uint64(5), big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	execution1, err := testutil.SignedExecution(addr4, priKey3, 6,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}
	selps = append(selps, selp)
	selps = append(selps, execution1)
	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[addr3] = selps
	if blk, err = bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	); err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	// Empty actions
	if blk, err = bc.MintNewBlock(
		nil,
		testutil.TimestampNow(),
	); err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	// Charlie transfer--> C
	// Alfa transfer--> A
	// Charlie exec--> D
	// Alfa exec--> D
	tsf1, err := testutil.SignedTransfer(addr3, priKey3, uint64(7), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2, err := testutil.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	execution1, err = testutil.SignedExecution(addr4, priKey3, 8,
		big.NewInt(2), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}
	execution2, err := testutil.SignedExecution(addr4, priKey1, 2,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[addr3] = []action.SealedEnvelope{tsf1, execution1}
	actionMap[addr1] = []action.SealedEnvelope{tsf2, execution2}
	if blk, err = bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	); err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}

func addActsToActPool(ap actpool.ActPool) error {
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
	if err := ap.Add(tsf1); err != nil {
		return err
	}
	if err := ap.Add(tsf2); err != nil {
		return err
	}
	if err := ap.Add(tsf3); err != nil {
		return err
	}
	return ap.Add(execution1)
}

func setupChain(cfg config.Config) (blockchain.Blockchain, *protocol.Registry, error) {
	cfg.Chain.ProducerPrivKey = hex.EncodeToString(identityset.PrivateKey(0).Bytes())
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	if err != nil {
		return nil, nil, err
	}

	// create chain
	registry := protocol.Registry{}
	bc := blockchain.NewBlockchain(
		cfg,
		blockchain.PrecreatedStateFactoryOption(sf),
		blockchain.InMemDaoOption(),
		blockchain.RegistryOption(&registry),
		blockchain.EnableExperimentalActions(),
	)
	if bc == nil {
		return nil, nil, errors.New("failed to create blockchain")
	}

	acc := account.NewProtocol(0)
	evm := execution.NewProtocol(bc, 0)
	p := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
	rolldposProtocol := rolldpos.NewProtocol(
		genesis.Default.NumCandidateDelegates,
		genesis.Default.NumDelegates,
		genesis.Default.NumSubEpochs,
	)
	r := rewarding.NewProtocol(bc, rolldposProtocol)

	if err := registry.Register(rolldpos.ProtocolID, rolldposProtocol); err != nil {
		return nil, nil, err
	}
	if err := registry.Register(account.ProtocolID, acc); err != nil {
		return nil, nil, err
	}
	if err := registry.Register(execution.ProtocolID, evm); err != nil {
		return nil, nil, err
	}
	if err := registry.Register(rewarding.ProtocolID, r); err != nil {
		return nil, nil, err
	}
	if err := registry.Register(poll.ProtocolID, p); err != nil {
		return nil, nil, err
	}
	sf.AddActionHandlers(acc, evm, r)
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	bc.Validator().AddActionValidators(acc, evm, r)

	return bc, &registry, nil
}

func setupActPool(bc blockchain.Blockchain, cfg config.ActPool) (actpool.ActPool, error) {
	ap, err := actpool.NewActPool(bc, cfg, actpool.EnableExperimentalActions())
	if err != nil {
		return nil, err
	}
	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(bc, genesis.Default.ActionGasLimit))
	ap.AddActionValidators(execution.NewProtocol(bc, 0))

	return ap, nil
}

func newConfig() config.Config {
	cfg := config.Default

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()

	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.EnableGravityChainVoting = true
	cfg.ActPool.MinGasPriceStr = "0"
	return cfg
}

func createServer(cfg config.Config, needActPool bool) (*Server, error) {
	bc, registry, err := setupChain(cfg)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	// Start blockchain
	if err := bc.Start(ctx); err != nil {
		return nil, err
	}

	// Create state for producer
	if err := addProducerToFactory(bc.GetFactory()); err != nil {
		return nil, err
	}

	// Add testing blocks
	if err := addTestingBlocks(bc); err != nil {
		return nil, err
	}

	var ap actpool.ActPool
	if needActPool {
		ap, err = setupActPool(bc, cfg.ActPool)
		if err != nil {
			return nil, err
		}
		// Add actions to actpool
		if err := addActsToActPool(ap); err != nil {
			return nil, err
		}
	}

	apiCfg := config.API{TpsWindow: cfg.API.TpsWindow, GasStation: cfg.API.GasStation, RangeQueryLimit: 100}
	svr := &Server{
		bc:             bc,
		ap:             ap,
		cfg:            apiCfg,
		gs:             gasstation.NewGasStation(bc, apiCfg, config.Default.Genesis.ActionGasLimit),
		registry:       registry,
		hasActionIndex: true,
	}

	return svr, nil
}
