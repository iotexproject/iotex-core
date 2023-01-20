package api

import (
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

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/go-pkgs/util"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	apitypes "github.com/iotexproject/iotex-core/api/types"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_apicoreservice"
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
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	svr := newHTTPHandler(NewWeb3Handler(core, ""))
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
}

func TestGasPrice(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}
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
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}
	core.EXPECT().EVMNetworkID().Return(uint32(1))
	ret, err := web3svr.getChainID()
	require.NoError(err)
	require.Equal("0x1", ret.(string))
}

func TestGetBlockNumber(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}
	core.EXPECT().TipHeight().Return(uint64(1))
	ret, err := web3svr.getBlockNumber()
	require.NoError(err)
	require.Equal("0x1", ret.(string))
}

func TestGetBlockByNumber(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}

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

	in := gjson.Parse(`{"params":["1", true]}`)
	ret, err := web3svr.getBlockByNumber(&in)
	require.NoError(err)
	rlt, ok := ret.(*getBlockResult)
	require.True(ok)
	require.Equal(rlt.blk.Header, blk.Header)
	require.Equal(rlt.blk.Receipts, receipts)
	require.Len(rlt.transactions, 1)
	tsrlt, ok := rlt.transactions[0].(*getTransactionResult)
	require.True(ok)
	require.Equal(tsrlt.receipt, receipts[0])
}

func TestGetBalance(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}
	balance := "111111111111111111"
	core.EXPECT().Account(gomock.Any()).Return(&iotextypes.AccountMeta{Balance: balance}, nil, nil)

	in := gjson.Parse(`{"params":["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", 1]}`)
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
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}
	core.EXPECT().PendingNonce(gomock.Any()).Return(uint64(2), nil)
	in := gjson.Parse(`{"params":["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", 1]}`)
	ret, err := web3svr.getTransactionCount(&in)
	require.NoError(err)
	require.Equal("0x2", ret.(string))
}

func TestCall(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}

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
		   },
		   1]}`)
		ret, err := web3svr.call(&in)
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
		   },
		   1]}`)
		ret, err := web3svr.call(&in)
		require.NoError(err)
		require.Equal("0x0000000000000000000000000000000000000000000000000000000000002710", ret.(string))
	})

	t.Run("to is contract addr", func(t *testing.T) {
		core.EXPECT().ReadContract(gomock.Any(), gomock.Any(), gomock.Any()).Return("111111", nil, nil)
		in := gjson.Parse(`{"params":[{
			"from":     "",
			"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
			"gas":      "0x4e20",
			"gasPrice": "0xe8d4a51000",
			"value":    "0x1",
			"data":     "0x1"
		   },
		   1]}`)
		ret, err := web3svr.call(&in)
		require.NoError(err)
		require.Equal("0x111111", ret.(string))
	})
}

func TestEstimateGas(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}
	core.EXPECT().ChainID().Return(uint32(1)).Times(2)

	t.Run("estimate execution", func(t *testing.T) {
		core.EXPECT().Account(gomock.Any()).Return(&iotextypes.AccountMeta{IsContract: true}, nil, nil)
		core.EXPECT().EstimateExecutionGasConsumption(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(11000), nil)

		in := gjson.Parse(`{"params":[{
			"from":     "",
			"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
			"gas":      "0x4e20",
			"gasPrice": "0xe8d4a51000",
			"value":    "0x1",
			"data":     "0x6d4ce63c"
		   },
		   1]}`)
		ret, err := web3svr.estimateGas(&in)
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
		   },
		   1]}`)
		ret, err := web3svr.estimateGas(&in)
		require.NoError(err)
		require.Equal(uint64ToHex(uint64(36000)), ret.(string))
	})
}

func TestSendRawTransaction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}
	core.EXPECT().EVMNetworkID().Return(uint32(1))
	core.EXPECT().ChainID().Return(uint32(1))
	core.EXPECT().Account(gomock.Any()).Return(&iotextypes.AccountMeta{IsContract: true}, nil, nil)
	core.EXPECT().SendAction(gomock.Any(), gomock.Any()).Return("111111111111111", nil)

	in := gjson.Parse(`{"params":["f8600180830186a09412745fec82b585f239c01090882eb40702c32b04808025a0b0e1aab5b64d744ae01fc9f1c3e9919844a799e90c23129d611f7efe6aec8a29a0195e28d22d9b280e00d501ff63525bb76f5c87b8646c89d5d9c5485edcb1b498"]}`)
	ret, err := web3svr.sendRawTransaction(&in)
	require.NoError(err)
	require.Equal("0x111111111111111", ret.(string))
}

func TestGetCode(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}
	code := "608060405234801561001057600080fd5b50610150806100206contractbytecode"
	data, _ := hex.DecodeString(code)
	core.EXPECT().Account(gomock.Any()).Return(&iotextypes.AccountMeta{ContractByteCode: data}, nil, nil)
	in := gjson.Parse(`{"params":["0x7c13866F9253DEf79e20034eDD011e1d69E67fe5"]}`)
	ret, err := web3svr.getCode(&in)
	require.NoError(err)
	require.Contains(code, util.Remove0xPrefix(ret.(string)))
}

func TestGetNodeInfo(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}
	core.EXPECT().ServerMeta().Return("111", "", "", "222", "")
	ret, err := web3svr.getNodeInfo()
	require.NoError(err)
	require.Equal("111/222", ret.(string))
}

func TestGetBlockTransactionCountByHash(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}

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

	blkHash := blk.HashBlock()
	in := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", true]}`, hex.EncodeToString(blkHash[:])))
	ret, err := web3svr.getBlockTransactionCountByHash(&in)
	require.NoError(err)
	require.Equal("0x1", ret.(string))
}

func TestGetBlockByHash(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}

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

	blkHash := blk.HashBlock()
	in := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", true]}`, hex.EncodeToString(blkHash[:])))
	ret, err := web3svr.getBlockByHash(&in)
	require.NoError(err)
	rlt, ok := ret.(*getBlockResult)
	require.True(ok)
	require.Equal(rlt.blk.Header, blk.Header)
	require.Equal(rlt.blk.Receipts, receipts)
	require.Len(rlt.transactions, 1)
	tsrlt, ok := rlt.transactions[0].(*getTransactionResult)
	require.True(ok)
	require.Equal(tsrlt.receipt, receipts[0])
}

func TestGetTransactionByHash(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}

	selp, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	txHash, err := selp.Hash()
	require.NoError(err)
	receipt := &action.Receipt{
		Status:          1,
		BlockHeight:     1,
		ActionHash:      hash.Hash256b([]byte("test")),
		GasConsumed:     1,
		ContractAddress: "test",
		TxIndex:         1,
	}
	core.EXPECT().ActionByActionHash(gomock.Any()).Return(selp, hash.Hash256b([]byte("test")), 0, 0, nil)
	core.EXPECT().ReceiptByActionHash(gomock.Any()).Return(receipt, nil)

	in := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", true]}`, hex.EncodeToString(txHash[:])))
	ret, err := web3svr.getTransactionByHash(&in)
	require.NoError(err)
	rlt, ok := ret.(*getTransactionResult)
	require.True(ok)
	require.Equal(rlt.receipt, receipt)
}

func TestGetLogs(t *testing.T) {

}

func TestGetTransactionReceipt(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}

	selp, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	txHash, err := selp.Hash()
	require.NoError(err)
	receipt := &action.Receipt{
		Status:          1,
		BlockHeight:     1,
		ActionHash:      hash.Hash256b([]byte("test")),
		GasConsumed:     1,
		ContractAddress: "test",
		TxIndex:         1,
	}
	core.EXPECT().ActionByActionHash(gomock.Any()).Return(selp, hash.Hash256b([]byte("test")), 0, 0, nil)
	core.EXPECT().ReceiptByActionHash(gomock.Any()).Return(receipt, nil)
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(111).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(time.Now()).
		AddActions(selp).
		SignAndBuild(identityset.PrivateKey(0))
	require.NoError(err)
	core.EXPECT().BlockByHash(gomock.Any()).Return(&apitypes.BlockWithReceipts{
		Block: &blk,
	}, nil)

	in := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", true]}`, hex.EncodeToString(txHash[:])))
	ret, err := web3svr.getTransactionReceipt(&in)
	require.NoError(err)
	rlt, ok := ret.(*getReceiptResult)
	require.True(ok)
	require.Equal(rlt.receipt, receipt)
	bf := blk.Header.LogsBloomfilter()
	topic, _ := hex.DecodeString(rlt.logsBloom)
	require.True(bf.Exist(topic))
}

func TestGetBlockTransactionCountByNumber(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := &web3Handler{core, nil}

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
	core.EXPECT().BlockByHeight(gomock.Any()).Return(&apitypes.BlockWithReceipts{
		Block: &blk,
	}, nil)
	
	in := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", true]}`, hex.EncodeToString(txHash[:])))
	ret, err := web3svr.getBlockTransactionCountByNumber(&in)
	require.NoError(err)
	require.Equal("111/222", ret.(string))
}

func TestGetTransactionByBlockHashAndIndex(t *testing.T) {
}

func TestGetTransactionByBlockNumberAndIndex(t *testing.T) {
}

func TestNewfilter(t *testing.T) {

}

func TestNewBlockFilter(t *testing.T) {

}

func TestGetFilterChanges(t *testing.T) {

}

func TestGetFilterLogs(t *testing.T) {

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
	require.Equal(data, testData)
	cacheLocal.Del(testKey)
	_, exist = cacheLocal.Get(testKey)
	require.False(exist)
}

func TestGetStorageAt(t *testing.T) {

}

func TestGetNetworkID(t *testing.T) {

}

func TestEthAccounts(t *testing.T) {

}
