package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/go-pkgs/util"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	mock_apitypes "github.com/iotexproject/iotex-core/v2/test/mock/mock_apiresponder"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestGetWeb3Reqs(t *testing.T) {
	require := require.New(t)
	testData := []struct {
		testName  string
		req       *http.Request
		hasHeader bool
		hasError  bool
	}{
		{
			testName:  "EmptyData",
			req:       httptest.NewRequest(http.MethodPost, "http://url.com", strings.NewReader("")),
			hasHeader: true,
			hasError:  true,
		},
		{
			testName:  "InvalidHttpMethod",
			req:       httptest.NewRequest(http.MethodPut, "http://url.com", strings.NewReader("")),
			hasHeader: false,
			hasError:  true,
		},
		{
			testName:  "MissingIdField",
			req:       httptest.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_clientVersion"}`)),
			hasHeader: true,
			hasError:  true,
		},
		{
			testName:  "MissingMethodField",
			req:       httptest.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","id":67}`)),
			hasHeader: true,
			hasError:  true,
		},
		{
			testName:  "MissingParamsField",
			req:       httptest.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_clientVersion","id":67}`)),
			hasHeader: true,
			hasError:  false,
		},
		{
			testName:  "Valid",
			req:       httptest.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_clientVersion","params":[],"id":67}`)),
			hasHeader: true,
			hasError:  false,
		},
	}

	for _, test := range testData {
		t.Run(test.testName, func(t *testing.T) {
			if test.hasHeader {
				test.req.Header.Set("Content-Type", "application/json")
			}
			_, err := parseWeb3Reqs(test.req.Body)
			if test.hasError {
				require.Error(err)
			} else {
				require.NoError(err)
			}
		})
	}
}

func TestHandlePost(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	core.EXPECT().Track(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
	svr := newHTTPHandler(NewWeb3Handler(core, "", _defaultBatchRequestLimit))
	getServerResp := func(svr *hTTPHandler, req *http.Request) *httptest.ResponseRecorder {
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		svr.ServeHTTP(resp, req)
		return resp
	}

	// web3 req without params
	request2, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"eth_getBalance","id":67}`))
	response2 := getServerResp(svr, request2)
	bodyBytes2, _ := io.ReadAll(response2.Body)
	require.Contains(string(bodyBytes2), "invalid format")

	// missing web3 method
	request3, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_foo","params":[],"id":67}`))
	response3 := getServerResp(svr, request3)
	bodyBytes3, _ := io.ReadAll(response3.Body)
	require.Contains(string(bodyBytes3), "method not found")

	// single web3 req
	request4, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"eth_mining","params":[],"id":67}`))
	response4 := getServerResp(svr, request4)
	bodyBytes4, _ := io.ReadAll(response4.Body)
	require.Contains(string(bodyBytes4), "result")

	// multiple web3 req
	request5, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`[{"jsonrpc":"2.0","method":"eth_mining","params":[],"id":1}, {"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":2}]`))
	response5 := getServerResp(svr, request5)
	bodyBytes5, _ := io.ReadAll(response5.Body)
	require.True(gjson.Valid(string(bodyBytes5)))
	require.Equal(2, len(gjson.Parse(string(bodyBytes5)).Array()))

	// multiple web3 req with big batch
	apitypes.MaxResponseSize = 1024 // fake max response size
	request8, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`[`+strings.Repeat(`{"jsonrpc":"2.0","method":"eth_mining","params":[],"id":1},`, 90)+`{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":2}]`))
	response8 := getServerResp(svr, request8)
	bodyBytes8, _ := io.ReadAll(response8.Body)
	require.Equal(len(bodyBytes8), 0)

	// multiple web3 req2
	request6, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`[{"jsonrpc":"2.0","method":"eth_mining","params":[],"id":1}]`))
	response6 := getServerResp(svr, request6)
	bodyBytes6, _ := io.ReadAll(response6.Body)
	require.True(gjson.Valid(string(bodyBytes6)))
	require.Equal(1, len(gjson.Parse(string(bodyBytes6)).Array()))

	// web3 req without params
	request7, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_clientVersion","id":67}`))
	core.EXPECT().ServerMeta().Return("mock str1", "mock str2", "mock str3", "mock str4", "mock str5")
	response7 := getServerResp(svr, request7)
	bodyBytes7, _ := io.ReadAll(response7.Body)
	require.Contains(string(bodyBytes7), "result")

	// multiple web3 req with big batch
	apitypes.MaxResponseSize = 1024 * 1024 * 100 // fake max response size
	request9, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`[`+strings.Repeat(`{"jsonrpc":"2.0","method":"eth_mining","params":[],"id":1},`, 102)+`{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":2}]`))
	response9 := getServerResp(svr, request9)
	bodyBytes9, _ := io.ReadAll(response9.Body)
	require.True(gjson.Valid(string(bodyBytes9)))
	require.Contains(string(bodyBytes9), errMsgBatchTooLarge.Error())
}

func TestGasPrice(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}
	core.EXPECT().SuggestGasPrice().Return(uint64(1), nil)
	ret, err := web3svr.gasPrice()
	require.NoError(err)
	require.Equal("0x1", ret.(string))

	core.EXPECT().SuggestGasPrice().Return(uint64(0), errors.New("mock gas price error"))
	_, err = web3svr.gasPrice()
	require.Equal("mock gas price error", err.Error())
}

func TestGetChainID(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}
	core.EXPECT().EVMNetworkID().Return(uint32(1))
	ret, err := web3svr.getChainID()
	require.NoError(err)
	require.Equal("0x1", ret.(string))
}

func TestGetBlockNumber(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}
	core.EXPECT().TipHeight().Return(uint64(1))
	ret, err := web3svr.getBlockNumber()
	require.NoError(err)
	require.Equal("0x1", ret.(string))
}

func TestGetBlockByNumber(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	tsf, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsfhash, err := tsf.Hash()
	require.NoError(err)
	receipts := []*action.Receipt{
		{BlockHeight: 1, ActionHash: tsfhash},
		{BlockHeight: 2, ActionHash: tsfhash},
	}
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(111).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(time.Now()).
		SetReceipts(receipts).
		AddActions(tsf).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(err)
	core.EXPECT().BlockByHeight(gomock.Any()).Return(&apitypes.BlockWithReceipts{
		Block:    &blk,
		Receipts: receipts,
	}, nil)
	core.EXPECT().EVMNetworkID().Return(uint32(0))

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		_, err := web3svr.getBlockByNumber(&inNil)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("nil params", func(t *testing.T) {
		in := gjson.Parse(`{"params":["1", true]}`)
		ret, err := web3svr.getBlockByNumber(&in)
		require.NoError(err)
		rlt, ok := ret.(*getBlockResult)
		require.True(ok)
		require.Equal(blk.Header, rlt.blk.Header)
		require.Equal(receipts, rlt.blk.Receipts)
		require.Len(rlt.transactions, 1)
		tsrlt, ok := rlt.transactions[0].(*getTransactionResult)
		require.True(ok)
		require.Equal(receipts[0], tsrlt.receipt)
	})
}

func TestGetBalance(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}
	balance := "111111111111111111"
	core.EXPECT().WithHeight(gomock.Any()).Return(core).Times(1)
	core.EXPECT().Account(gomock.Any()).Return(&iotextypes.AccountMeta{Balance: balance}, nil, nil)

	in := gjson.Parse(`{"params":["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", "0x1"]}`)
	ret, err := web3svr.getBalance(&in)
	require.NoError(err)
	ans, ok := new(big.Int).SetString(balance, 10)
	require.True(ok)
	require.Equal("0x"+fmt.Sprintf("%x", ans), ret.(string))
}

func TestGetTransactionCount(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}
	core.EXPECT().PendingNonce(gomock.Any()).Return(uint64(2), nil)

	inNil := gjson.Parse(`{"params":[]}`)
	_, err := web3svr.getTransactionCount(&inNil)
	require.EqualError(err, errInvalidFormat.Error())

	in := gjson.Parse(`{"params":["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", 1]}`)
	ret, err := web3svr.getTransactionCount(&in)
	require.NoError(err)
	require.Equal("0x2", ret.(string))
}

func TestCall(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	t.Run("to is StakingProtocol addr", func(t *testing.T) {
		meta := &iotextypes.AccountMeta{
			Address: "io000000000000000000000000stakingprotocol",
			Balance: "100000000000000000000",
		}
		metaBytes, _ := proto.Marshal(meta)
		core.EXPECT().ReadState(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&iotexapi.ReadStateResponse{
			Data: metaBytes,
		}, nil)
		in := gjson.Parse(`{"params":[{
			"from":     "",
			"to":       "0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			"gas":      "0x4e20",
			"gasPrice": "0xe8d4a51000",
			"value":    "0x1",
			"data":     "d201114a"
		   }, "0x1"]}`)
		ret, err := web3svr.call(context.Background(), &in)
		require.NoError(err)
		require.Equal("0x0000000000000000000000000000000000000000000000056bc75e2d63100000", ret.(string))
	})

	t.Run("to is RewardingProtocol addr", func(t *testing.T) {
		amount := big.NewInt(10000)
		core.EXPECT().ReadState(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&iotexapi.ReadStateResponse{
			Data: []byte(amount.String()),
		}, nil)
		in := gjson.Parse(`{"params":[{
			"from":     "",
			"to":       "0xA576C141e5659137ddDa4223d209d4744b2106BE",
			"gas":      "0x4e20",
			"gasPrice": "0xe8d4a51000",
			"value":    "0x1",
			"data":     "ad7a672f"
		   }, "0x1"]}`)
		ret, err := web3svr.call(context.Background(), &in)
		require.NoError(err)
		require.Equal("0x0000000000000000000000000000000000000000000000000000000000002710", ret.(string))
	})

	t.Run("to is contract addr", func(t *testing.T) {
		coreWithHeight := NewMockCoreServiceReaderWithHeight(ctrl)
		core.EXPECT().WithHeight(gomock.Any()).Return(coreWithHeight).Times(1)
		coreWithHeight.EXPECT().ReadContract(gomock.Any(), gomock.Any(), gomock.Any()).Return("111111", nil, nil)
		in := gjson.Parse(`{"params":[{
			"from":     "",
			"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
			"gas":      "0x4e20",
			"gasPrice": "0xe8d4a51000",
			"value":    "0x1",
			"data":     "0x1"
		   }, "0x1"]}`)
		ret, err := web3svr.call(context.Background(), &in)
		require.NoError(err)
		require.Equal("0x111111", ret.(string))
	})

	t.Run("revert call", func(t *testing.T) {
		receipt := &iotextypes.Receipt{
			Status:             uint64(iotextypes.ReceiptStatus_ErrExecutionReverted),
			BlkHeight:          0,
			ActHash:            nil,
			GasConsumed:        0,
			ContractAddress:    "",
			Logs:               nil,
			ExecutionRevertMsg: "revert call",
			TxIndex:            0,
		}
		coreWithHeight := NewMockCoreServiceReaderWithHeight(ctrl)
		core.EXPECT().WithHeight(gomock.Any()).Return(coreWithHeight).Times(1)
		coreWithHeight.EXPECT().ReadContract(gomock.Any(), gomock.Any(), gomock.Any()).Return("", receipt, nil)
		in := gjson.Parse(`{"params":[{
			"from":     "",
			"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
			"gas":      "0x4e20",
			"gasPrice": "0xe8d4a51000",
			"value":    "0x1",
			"data":     "0x1"
		   }, "0x1"]}`)
		_, err := web3svr.call(context.Background(), &in)
		require.EqualError(err, "rpc error: code = InvalidArgument desc = execution reverted: "+receipt.GetExecutionRevertMsg())
	})
}

func TestEstimateGas(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}
	core.EXPECT().ChainID().Return(uint32(1)).Times(2)
	core.EXPECT().EVMNetworkID().Return(uint32(0)).Times(2)

	t.Run("estimate execution", func(t *testing.T) {
		core.EXPECT().Account(gomock.Any()).Return(&iotextypes.AccountMeta{IsContract: true}, nil, nil)
		core.EXPECT().EstimateExecutionGasConsumption(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(11000), nil, nil)

		in := gjson.Parse(`{"params":[{
			"from":     "",
			"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
			"gas":      "0x4e20",
			"gasPrice": "0xe8d4a51000",
			"value":    "0x1",
			"data":     "0x6d4ce63c"
		   }, "0x1"]}`)
		ret, err := web3svr.estimateGas(context.Background(), &in)
		require.NoError(err)
		require.Equal(uint64ToHex(uint64(21000)), ret.(string))
	})

	t.Run("estimate nonexecution", func(t *testing.T) {
		core.EXPECT().Account(gomock.Any()).Return(&iotextypes.AccountMeta{IsContract: false}, nil, nil)
		core.EXPECT().EstimateGasForNonExecution(gomock.Any()).Return(uint64(36000), nil)

		in := gjson.Parse(`{"params":[{
			"from":     "",
			"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
			"gas":      "0x4e20",
			"gasPrice": "0xe8d4a51000",
			"value":    "0x1",
			"data":     "0x1123123c"
		   }, "0x1"]}`)
		ret, err := web3svr.estimateGas(context.Background(), &in)
		require.NoError(err)
		require.Equal(uint64ToHex(uint64(36000)), ret.(string))
	})
}

func TestSendRawTransaction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}
	core.EXPECT().Genesis().Return(genesis.TestDefault())
	core.EXPECT().TipHeight().Return(uint64(0))
	core.EXPECT().EVMNetworkID().Return(uint32(1))
	core.EXPECT().ChainID().Return(uint32(1))
	core.EXPECT().Account(gomock.Any()).Return(&iotextypes.AccountMeta{IsContract: true}, nil, nil)
	core.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return("111111111111111", nil)

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		_, err := web3svr.sendRawTransaction(context.Background(), &inNil)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("send tx", func(t *testing.T) {
		in := gjson.Parse(`{"params":["f8600180830186a09412745fec82b585f239c01090882eb40702c32b04808025a0b0e1aab5b64d744ae01fc9f1c3e9919844a799e90c23129d611f7efe6aec8a29a0195e28d22d9b280e00d501ff63525bb76f5c87b8646c89d5d9c5485edcb1b498"]}`)
		ret, err := web3svr.sendRawTransaction(context.Background(), &in)
		require.NoError(err)
		require.Equal("0x111111111111111", ret.(string))
	})
}

func TestGetCode(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}
	code := "608060405234801561001057600080fd5b50610150806100206contractbytecode"
	data, _ := hex.DecodeString(code)
	core.EXPECT().Account(gomock.Any()).Return(&iotextypes.AccountMeta{ContractByteCode: data}, nil, nil)

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		_, err := web3svr.getCode(&inNil)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("get code", func(t *testing.T) {
		in := gjson.Parse(`{"params":["0x7c13866F9253DEf79e20034eDD011e1d69E67fe5"]}`)
		ret, err := web3svr.getCode(&in)
		require.NoError(err)
		require.Contains(code, util.Remove0xPrefix(ret.(string)))
	})
}

func TestGetNodeInfo(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}
	core.EXPECT().ServerMeta().Return("111", "", "", "222", "")
	ret, err := web3svr.getNodeInfo()
	require.NoError(err)
	require.Equal("111/222", ret.(string))
}

func TestGetNetworkID(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}
	core.EXPECT().EVMNetworkID().Return(uint32(123))
	ret, err := web3svr.getNetworkID()
	require.NoError(err)
	require.Equal("123", ret.(string))
}

func TestIsSyncing(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}
	core.EXPECT().SyncingProgress().Return(uint64(1), uint64(2), uint64(3))
	ret, err := web3svr.isSyncing()
	require.NoError(err)
	rlt, ok := ret.(*getSyncingResult)
	require.True(ok)
	require.Equal("0x1", rlt.StartingBlock)
	require.Equal("0x2", rlt.CurrentBlock)
	require.Equal("0x3", rlt.HighestBlock)
}

func TestGetBlockTransactionCountByHash(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	tsf, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(111).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(time.Now()).
		AddActions(tsf).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(err)
	core.EXPECT().BlockByHash(gomock.Any()).Return(&apitypes.BlockWithReceipts{
		Block: &blk,
	}, nil)

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		_, err := web3svr.getBlockTransactionCountByHash(&inNil)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("get block", func(t *testing.T) {
		blkHash := blk.HashBlock()
		in := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", true]}`, hex.EncodeToString(blkHash[:])))
		ret, err := web3svr.getBlockTransactionCountByHash(&in)
		require.NoError(err)
		require.Equal("0x1", ret.(string))
	})
}

func TestGetBlockByHash(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	tsf, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsfhash, err := tsf.Hash()
	require.NoError(err)
	receipts := []*action.Receipt{
		{BlockHeight: 1, ActionHash: tsfhash},
		{BlockHeight: 2, ActionHash: tsfhash},
	}
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(111).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(time.Now()).
		SetReceipts(receipts).
		AddActions(tsf).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(err)
	core.EXPECT().BlockByHash(gomock.Any()).Return(&apitypes.BlockWithReceipts{
		Block:    &blk,
		Receipts: receipts,
	}, nil)
	core.EXPECT().EVMNetworkID().Return(uint32(0))

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		_, err := web3svr.getBlockByHash(&inNil)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("get block", func(t *testing.T) {
		blkHash := blk.HashBlock()
		in := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", true]}`, hex.EncodeToString(blkHash[:])))
		ret, err := web3svr.getBlockByHash(&in)
		require.NoError(err)
		rlt, ok := ret.(*getBlockResult)
		require.True(ok)
		require.Equal(blk.Header, rlt.blk.Header)
		require.Equal(receipts, rlt.blk.Receipts)
		require.Len(rlt.transactions, 1)
		tsrlt, ok := rlt.transactions[0].(*getTransactionResult)
		require.True(ok)
		require.Equal(receipts[0], tsrlt.receipt)
	})
}

func TestGetTransactionByHash(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	selp, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	txHash, err := selp.Hash()
	require.NoError(err)
	receipt := &action.Receipt{
		Status:          1,
		BlockHeight:     1,
		ActionHash:      txHash,
		GasConsumed:     1,
		ContractAddress: "test",
		TxIndex:         1,
	}
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(111).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(time.Now()).
		AddActions(selp).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(err)
	core.EXPECT().ActionByActionHash(gomock.Any()).Return(selp, &blk, uint32(0), nil)
	core.EXPECT().ReceiptByActionHash(gomock.Any()).Return(receipt, nil)
	core.EXPECT().EVMNetworkID().Return(uint32(0))

	inNil := gjson.Parse(`{"params":[]}`)
	ret, err := web3svr.getTransactionByHash(&inNil)
	require.EqualError(err, errInvalidFormat.Error())

	in := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", true]}`, hex.EncodeToString(txHash[:])))
	ret, err = web3svr.getTransactionByHash(&in)
	require.NoError(err)
	rlt, ok := ret.(*getTransactionResult)
	require.True(ok)
	require.Equal(receipt, rlt.receipt)

	// get pending transaction
	core.EXPECT().ActionByActionHash(gomock.Any()).Return(nil, nil, uint32(0), ErrNotFound)
	core.EXPECT().PendingActionByActionHash(gomock.Any()).Return(selp, nil)
	core.EXPECT().EVMNetworkID().Return(uint32(0))
	ret, err = web3svr.getTransactionByHash(&in)
	require.NoError(err)
	rlt, ok = ret.(*getTransactionResult)
	require.True(ok)
	require.Nil(rlt.blockHash)
	require.Nil(rlt.receipt)

	// get pending contract deploy transaction
	selp, err = action.SignedExecution("", identityset.PrivateKey(27), 1, big.NewInt(1), 10000, big.NewInt(0), []byte("test"))
	require.NoError(err)
	txHash, err = selp.Hash()
	require.NoError(err)
	core.EXPECT().ActionByActionHash(gomock.Any()).Return(nil, nil, uint32(0), ErrNotFound)
	core.EXPECT().PendingActionByActionHash(gomock.Any()).Return(selp, nil)
	core.EXPECT().EVMNetworkID().Return(uint32(0))
	ret, err = web3svr.getTransactionByHash(&in)
	require.NoError(err)
	rlt, ok = ret.(*getTransactionResult)
	require.True(ok)
	require.Nil(rlt.blockHash)
	require.Nil(rlt.receipt)
	require.Nil(rlt.to)
}

func TestGetLogs(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	logs := []*action.Log{
		{
			Address:     "_topic1",
			BlockHeight: 1,
			Topics: []hash.Hash256{
				hash.Hash256b([]byte("_topic1")),
				hash.Hash256b([]byte("_topic11")),
			},
		},
		{
			Address:     "_topic2",
			BlockHeight: 2,
			Topics: []hash.Hash256{
				hash.Hash256b([]byte("_topic2")),
				hash.Hash256b([]byte("_topic22")),
			},
		},
	}
	blkHash1 := hash.Hash256b([]byte("_block1"))
	blkHash2 := hash.Hash256b([]byte("_block2"))
	hashes := []hash.Hash256{
		blkHash1,
		blkHash2,
	}
	core.EXPECT().LogsInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(logs, hashes, nil)

	ret, err := web3svr.getLogs(&filterObject{
		FromBlock: "1",
		ToBlock:   "2",
		Address:   []string{"0x0000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000002"},
		Topics:    [][]string{{byteToHex([]byte("_topic1")), byteToHex([]byte("_topic2")), byteToHex([]byte("_topic3"))}, {byteToHex([]byte("_topic4"))}},
	})
	require.NoError(err)
	rlt, ok := ret.([]*getLogsResult)
	require.True(ok)
	require.Len(rlt, 2)
	require.Equal("_topic1", rlt[0].log.Address)
	require.Equal(blkHash1, rlt[0].blockHash)
	require.Equal("_topic2", rlt[1].log.Address)
	require.Equal(blkHash2, rlt[1].blockHash)
}

func TestGetTransactionReceipt(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	selp, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	txHash, err := selp.Hash()
	require.NoError(err)
	receipt := &action.Receipt{
		Status:          1,
		BlockHeight:     1,
		ActionHash:      txHash,
		GasConsumed:     1,
		ContractAddress: "test",
		TxIndex:         1,
	}
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(111).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(time.Now()).
		AddActions(selp).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(err)
	core.EXPECT().ActionByActionHash(gomock.Any()).Return(selp, &blk, uint32(0), nil)
	core.EXPECT().ReceiptByActionHash(gomock.Any()).Return(receipt, nil)

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		_, err := web3svr.getTransactionReceipt(&inNil)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("get tx", func(t *testing.T) {
		in := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", true]}`, hex.EncodeToString(txHash[:])))
		ret, err := web3svr.getTransactionReceipt(&in)
		require.NoError(err)
		rlt, ok := ret.(*getReceiptResult)
		require.True(ok)
		require.Equal(receipt, rlt.receipt)
		require.Equal("", rlt.logsBloom)
		require.Nil(blk.Header.LogsBloomfilter())
	})
}

func TestGetBlockTransactionCountByNumber(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	tsf, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(111).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(time.Now()).
		AddActions(tsf, tsf).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(err)
	core.EXPECT().BlockByHeight(gomock.Any()).Return(&apitypes.BlockWithReceipts{
		Block: &blk,
	}, nil)

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		_, err := web3svr.getBlockTransactionCountByNumber(&inNil)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("get block", func(t *testing.T) {
		in := gjson.Parse(`{"params":["0x1"]}`)
		ret, err := web3svr.getBlockTransactionCountByNumber(&in)
		require.NoError(err)
		require.Equal("0x2", ret.(string))
	})
}

func TestGetTransactionByBlockHashAndIndex(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	tsf, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsfhash, err := tsf.Hash()
	require.NoError(err)
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(111).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(time.Now()).
		AddActions(tsf).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(err)
	receipts := []*action.Receipt{
		{BlockHeight: 1, ActionHash: tsfhash},
		{BlockHeight: 2, ActionHash: tsfhash},
	}
	core.EXPECT().BlockByHash(gomock.Any()).Return(&apitypes.BlockWithReceipts{
		Block:    &blk,
		Receipts: receipts,
	}, nil)
	core.EXPECT().EVMNetworkID().Return(uint32(0))

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		_, err := web3svr.getTransactionByBlockHashAndIndex(&inNil)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("get tx", func(t *testing.T) {
		blkHash := blk.HashBlock()
		in := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", "0"]}`, hex.EncodeToString(blkHash[:])))
		ret, err := web3svr.getTransactionByBlockHashAndIndex(&in)
		require.NoError(err)
		rlt, ok := ret.(*getTransactionResult)
		require.True(ok)
		require.Equal(receipts[0], rlt.receipt)
		require.Equal(blkHash, *rlt.blockHash)
	})
}

func TestGetTransactionByBlockNumberAndIndex(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	tsf, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsfhash, err := tsf.Hash()
	require.NoError(err)
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(111).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(time.Now()).
		AddActions(tsf).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(err)
	receipts := []*action.Receipt{
		{BlockHeight: 1, ActionHash: tsfhash},
		{BlockHeight: 2, ActionHash: tsfhash},
	}
	core.EXPECT().BlockByHeight(gomock.Any()).Return(&apitypes.BlockWithReceipts{
		Block:    &blk,
		Receipts: receipts,
	}, nil)
	core.EXPECT().EVMNetworkID().Return(uint32(0))

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		_, err := web3svr.getTransactionByBlockNumberAndIndex(&inNil)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("get tx", func(t *testing.T) {
		in := gjson.Parse(`{"params":["0x1", "0"]}`)
		ret, err := web3svr.getTransactionByBlockNumberAndIndex(&in)
		require.NoError(err)
		rlt, ok := ret.(*getTransactionResult)
		require.True(ok)
		require.Equal(receipts[0], rlt.receipt)
		require.Equal(blk.HashBlock(), *rlt.blockHash)
	})
}

func TestGetStorageAt(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}
	val := []byte("test")
	core.EXPECT().ReadContractStorage(gomock.Any(), gomock.Any(), gomock.Any()).Return(val, nil)

	in := gjson.Parse(`{"params":["0x123456789abc", "0"]}`)
	ret, err := web3svr.getStorageAt(&in)
	require.NoError(err)
	require.Equal("0x"+hex.EncodeToString(val), ret.(string))
}

func TestNewfilter(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, newAPICache(1*time.Second, ""), _defaultBatchRequestLimit}

	ret, err := web3svr.newFilter(&filterObject{
		FromBlock: "1",
		ToBlock:   "2",
		Address:   []string{"0x0000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000002"},
		Topics:    [][]string{{byteToHex([]byte("_topic1")), byteToHex([]byte("_topic2")), byteToHex([]byte("_topic3"))}, {byteToHex([]byte("_topic4"))}},
	})
	require.NoError(err)
	require.Equal("0x6e86c450ba48d23a459b74581736ca033ed60ef2a3d5ae09c316f77f67d7fad7", ret.(string))
}

func TestNewBlockFilter(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, newAPICache(1*time.Second, ""), _defaultBatchRequestLimit}
	core.EXPECT().TipHeight().Return(uint64(123))

	ret, err := web3svr.newBlockFilter()
	require.NoError(err)
	require.Equal("0x4c6ace15a9c5b9d3c89e786b7b6dfaf1bdc5807b8d7da0292db94d473f349101", ret.(string))
}

func TestUninstallFilter(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, newAPICache(1*time.Second, ""), _defaultBatchRequestLimit}

	require.NoError(web3svr.cache.Set("123456789abc", []byte("test")))

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		_, err := web3svr.uninstallFilter(&inNil)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("uninstall filter", func(t *testing.T) {
		in := gjson.Parse(`{"params":["0x123456789abc"]}`)
		ret, err := web3svr.uninstallFilter(&in)
		require.NoError(err)
		require.True(ret.(bool))
	})
}

func TestGetFilterChanges(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, newAPICache(1*time.Second, ""), _defaultBatchRequestLimit}
	core.EXPECT().TipHeight().Return(uint64(0)).Times(3)

	t.Run("log filterType", func(t *testing.T) {
		logs := []*action.Log{
			{
				Address:     "_topic1",
				BlockHeight: 1,
				Topics: []hash.Hash256{
					hash.Hash256b([]byte("_topic1")),
					hash.Hash256b([]byte("_topic11")),
				},
			},
			{
				Address:     "_topic2",
				BlockHeight: 2,
				Topics: []hash.Hash256{
					hash.Hash256b([]byte("_topic2")),
					hash.Hash256b([]byte("_topic22")),
				},
			},
		}
		blkHash1 := hash.Hash256b([]byte("_block1"))
		blkHash2 := hash.Hash256b([]byte("_block2"))
		hashes := []hash.Hash256{
			blkHash1,
			blkHash2,
		}
		core.EXPECT().LogsInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(logs, hashes, nil)

		require.NoError(web3svr.cache.Set("123456789abc", []byte(`{"logHeight":0,"filterType":"log","fromBlock":"0x1"}`)))
		in := gjson.Parse(`{"params":["0x123456789abc"]}`)
		ret, err := web3svr.getFilterChanges(&in)
		require.NoError(err)
		rlt, ok := ret.([]*getLogsResult)
		require.True(ok)
		require.Len(rlt, 2)
		require.Equal("_topic1", rlt[0].log.Address)
		require.Equal(blkHash1, rlt[0].blockHash)
		require.Equal("_topic2", rlt[1].log.Address)
		require.Equal(blkHash2, rlt[1].blockHash)
	})

	t.Run("block filterType", func(t *testing.T) {
		tsf, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		tsfhash, err := tsf.Hash()
		require.NoError(err)
		blk, err := block.NewTestingBuilder().
			SetHeight(1).
			SetVersion(111).
			SetPrevBlockHash(hash.ZeroHash256).
			SetTimeStamp(time.Now()).
			AddActions(tsf).
			SignAndBuild(identityset.PrivateKey(0))
		require.NoError(err)
		receipts := []*action.Receipt{
			{BlockHeight: 1, ActionHash: tsfhash},
			{BlockHeight: 2, ActionHash: tsfhash},
		}
		core.EXPECT().BlockByHeightRange(gomock.Any(), gomock.Any()).Return(
			[]*apitypes.BlockWithReceipts{
				{
					Block:    &blk,
					Receipts: receipts,
				},
			}, nil)

		require.NoError(web3svr.cache.Set("123456789abc", []byte(`{"logHeight":0,"filterType":"block","fromBlock":"0x1"}`)))
		in := gjson.Parse(`{"params":["0x123456789abc"]}`)
		ret, err := web3svr.getFilterChanges(&in)
		require.NoError(err)
		rlt, ok := ret.([]string)
		require.True(ok)
		require.Len(rlt, 1)
		blkHash := blk.HashBlock()
		require.Equal("0x"+hex.EncodeToString(blkHash[:]), rlt[0])
	})
}

func TestGetFilterLogs(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, newAPICache(1*time.Second, ""), _defaultBatchRequestLimit}

	logs := []*action.Log{
		{
			Address:     "_topic1",
			BlockHeight: 1,
			Topics: []hash.Hash256{
				hash.Hash256b([]byte("_topic1")),
				hash.Hash256b([]byte("_topic11")),
			},
		},
		{
			Address:     "_topic2",
			BlockHeight: 2,
			Topics: []hash.Hash256{
				hash.Hash256b([]byte("_topic2")),
				hash.Hash256b([]byte("_topic22")),
			},
		},
	}
	blkHash1 := hash.Hash256b([]byte("_block1"))
	blkHash2 := hash.Hash256b([]byte("_block2"))
	hashes := []hash.Hash256{
		blkHash1,
		blkHash2,
	}
	core.EXPECT().TipHeight().Return(uint64(0))
	core.EXPECT().LogsInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(logs, hashes, nil)

	require.NoError(web3svr.cache.Set("123456789abc", []byte(`{"logHeight":0,"filterType":"log","fromBlock":"0x1"}`)))

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		_, err := web3svr.getFilterLogs(&inNil)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("get filter log", func(t *testing.T) {
		in := gjson.Parse(`{"params":["0x123456789abc"]}`)
		ret, err := web3svr.getFilterLogs(&in)
		require.NoError(err)
		rlt, ok := ret.([]*getLogsResult)
		require.True(ok)
		require.Len(rlt, 2)
		require.Equal("_topic1", rlt[0].log.Address)
		require.Equal(blkHash1, rlt[0].blockHash)
		require.Equal("_topic2", rlt[1].log.Address)
		require.Equal(blkHash2, rlt[1].blockHash)
	})
}

func TestSubscribe(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	listener := mock_apitypes.NewMockListener(ctrl)
	listener.EXPECT().AddResponder(gomock.Any()).Return("streamid_1", nil).Times(3)
	core.EXPECT().ChainListener().Return(listener).Times(3)
	writer := mock_apitypes.NewMockWeb3ResponseWriter(ctrl)

	t.Run("newHeads subscription", func(t *testing.T) {
		in := gjson.Parse(`{"params":["newHeads"]}`)
		sc, _ := StreamFromContext(WithStreamContext(context.Background()))
		ret, err := web3svr.subscribe(sc, &in, writer)
		require.NoError(err)
		require.Equal("streamid_1", ret.(string))
	})

	t.Run("logs subscription", func(t *testing.T) {
		in := gjson.Parse(`{"params":["logs",{"fromBlock":"1","fromBlock":"2","address":["0x0000000000000000000000000000000000000001"],"topics":[["0x5f746f70696331"]]}]}`)
		sc, _ := StreamFromContext(WithStreamContext(context.Background()))
		ret, err := web3svr.subscribe(sc, &in, writer)
		require.NoError(err)
		require.Equal("streamid_1", ret.(string))
	})

	t.Run("logs topic not array", func(t *testing.T) {
		in := gjson.Parse(`{"params":["logs",{"fromBlock":"1","fromBlock":"2","address":["0x0000000000000000000000000000000000000001"],"topics":["0x5f746f70696331"]}]}`)
		sc, _ := StreamFromContext(WithStreamContext(context.Background()))
		ret, err := web3svr.subscribe(sc, &in, writer)
		require.NoError(err)
		require.Equal("streamid_1", ret.(string))
	})

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		sc, _ := StreamFromContext(WithStreamContext(context.Background()))
		_, err := web3svr.subscribe(sc, &inNil, writer)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("nil logs", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":["logs"]}`)
		sc, _ := StreamFromContext(WithStreamContext(context.Background()))
		_, err := web3svr.subscribe(sc, &inNil, writer)
		require.EqualError(err, errInvalidFormat.Error())
	})
}

func TestUnsubscribe(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	listener := mock_apitypes.NewMockListener(ctrl)
	listener.EXPECT().RemoveResponder(gomock.Any()).Return(true, nil)
	core.EXPECT().ChainListener().Return(listener)

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		_, err := web3svr.unsubscribe(&inNil)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("unsubscribe", func(t *testing.T) {
		in := gjson.Parse(`{"params":["0x123456789abc"]}`)
		ret, err := web3svr.unsubscribe(&in)
		require.NoError(err)
		require.True(ret.(bool))
	})
}

func TestLocalAPICache(t *testing.T) {
	require := require.New(t)
	testKey, testData := strconv.Itoa(rand.Int()), []byte(strconv.Itoa(rand.Int()))
	cacheLocal := newAPICache(1*time.Second, "")
	_, exist := cacheLocal.Get(testKey)
	require.False(exist)
	err := cacheLocal.Set(testKey, testData)
	require.NoError(err)
	data, _ := cacheLocal.Get(testKey)
	require.Equal(testData, data)
	cacheLocal.Del(testKey)
	_, exist = cacheLocal.Get(testKey)
	require.False(exist)
}

func TestDebugTraceTransaction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	ctx := context.Background()
	tsf, err := action.SignedExecution(identityset.Address(29).String(),
		identityset.PrivateKey(29), 1, big.NewInt(0), testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64), []byte{})
	require.NoError(err)
	tsfhash, err := tsf.Hash()
	require.NoError(err)
	receipt := &action.Receipt{Status: 1, BlockHeight: 1, ActionHash: tsfhash, GasConsumed: 100000}
	structLogger := &logger.StructLogger{}

	core.EXPECT().TraceTransaction(ctx, gomock.Any(), gomock.Any()).AnyTimes().Return([]byte{0x01}, receipt, structLogger, nil)

	t.Run("nil params", func(t *testing.T) {
		inNil := gjson.Parse(`{"params":[]}`)
		_, err := web3svr.traceTransaction(ctx, &inNil)
		require.EqualError(err, errInvalidFormat.Error())
	})

	t.Run("trace tx", func(t *testing.T) {
		in := gjson.Parse(`{"params":["` + hex.EncodeToString(tsfhash[:]) + `"]}`)
		ret, err := web3svr.traceTransaction(ctx, &in)
		require.NoError(err)
		rlt, ok := ret.(*debugTraceTransactionResult)
		require.True(ok)
		require.Equal("0x01", rlt.ReturnValue)
		require.False(rlt.Failed)
		require.Equal(uint64(100000), rlt.Gas)
		require.Empty(rlt.Revert)
		require.Equal(0, len(rlt.StructLogs))
	})
}

func TestDebugTraceCall(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	ctx := context.Background()
	tsf, err := action.SignedExecution(identityset.Address(29).String(),
		identityset.PrivateKey(29), 1, big.NewInt(0), testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64), []byte{})
	require.NoError(err)
	tsfhash, err := tsf.Hash()
	require.NoError(err)
	receipt := &action.Receipt{Status: 1, BlockHeight: 1, ActionHash: tsfhash, GasConsumed: 100000}
	structLogger := &logger.StructLogger{}

	core.EXPECT().TraceCall(ctx, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return([]byte{0x01}, receipt, structLogger, nil)

	in := gjson.Parse(`{"method":"debug_traceCall","params":[{"from":null,"to":"0x6b175474e89094c44da98b954eedeac495271d0f","data":"0x70a082310000000000000000000000006E0d01A76C3Cf4288372a29124A26D4353EE51BE"}],"id":1,"jsonrpc":"2.0"}`)
	ret, err := web3svr.traceCall(ctx, &in)
	require.NoError(err)
	rlt, ok := ret.(*debugTraceTransactionResult)
	require.True(ok)
	require.Equal("0x01", rlt.ReturnValue)
	require.False(rlt.Failed)
	require.Equal(uint64(100000), rlt.Gas)
	require.Empty(rlt.Revert)
	require.Equal(0, len(rlt.StructLogs))
}

func TestResponseIDMatchTypeWithRequest(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	core.EXPECT().TipHeight().Return(uint64(1)).AnyTimes()
	core.EXPECT().Track(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
	svr := newHTTPHandler(NewWeb3Handler(core, "", _defaultBatchRequestLimit))
	getServerResp := func(svr *hTTPHandler, req *http.Request) *httptest.ResponseRecorder {
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		svr.ServeHTTP(resp, req)
		return resp
	}
	tests := []struct {
		req string
		sub string
	}{
		{`{"jsonrpc":"2.0","method":"eth_blockNumber","id":1}`, `"id":1`},
		{`{"jsonrpc":"2.0","method":"eth_blockNumber","id":"1"}`, `"id":"1"`},
		{`{"jsonrpc":"2.0","method":"eth_blockNumber","id":"0x32"}`, `"id":"0x32"`},
		{`{"jsonrpc":"2.0","method":"eth_blockNumber","id":[]}`, `error`},
		{`{"jsonrpc":"2.0","method":"eth_blockNumber","id":[1]}`, `error`},
		{`{"jsonrpc":"2.0","method":"eth_blockNumber","id":0x32}`, `error`},
		{`{"jsonrpc":"2.0","method":"eth_blockNumber","id":{1}}`, `error`},
	}
	for _, tt := range tests {
		request, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(tt.req))
		response := getServerResp(svr, request)
		bodyBytes, _ := io.ReadAll(response.Body)
		require.Contains(string(bodyBytes), tt.sub)
	}
}
