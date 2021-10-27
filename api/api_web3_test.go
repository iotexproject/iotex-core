package api

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestGasPrice(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	ret, _ := gasPrice(svr, nil)
	require.Equal(ret, uint64ToHex(1000000000000))
}

func TestGetChainID(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	ret, _ := getChainID(svr, nil)
	require.Equal(ret, uint64ToHex(1))
}

func TestGetBlockNumber(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	ret, _ := getBlockNumber(svr, nil)
	require.Equal(ret, uint64ToHex(4))
}

func TestGetBlockByNumber(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{"1", false}
	ret, _ := getBlockByNumber(svr, testData)
	v, _ := ret.(blockObject)
	require.Equal(len(v.Transactions), 2)
}

func TestGetBalance(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{"0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", 1}
	ret, _ := getBalance(svr, testData)
	ans, _ := big.NewInt(0).SetString("9999999999999999999999999991", 10)
	require.Equal(ret, "0x"+fmt.Sprintf("%x", ans))
}

func TestGetTransactionCount(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{"0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", 1}
	ret, _ := getTransactionCount(svr, testData)
	require.Equal(ret, uint64ToHex(2))
}

func TestCall(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{
		map[string]interface{}{
			"from":     "",
			"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
			"gas":      "0x4e20",
			"gasPrice": "0xe8d4a51000",
			"value":    "0x1",
			"data":     "0x1"},
		1}
	_, err := call(svr, testData)
	require.NoError(err)

	testData2 := []interface{}{
		map[string]interface{}{
			"from":     "",
			"to":       "0xb1f8e55c7f64d203c1400b9d8555d050f94adf39",
			"gas":      "0x4e20",
			"gasPrice": "0xe8d4a51000",
			"value":    "0x1",
			"data":     "0x1"},
		1}
	ret, _ := call(svr, testData2)
	require.Nil(ret)
}

func TestEstimateGas(t *testing.T) {
	var web3Util web3Utils
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContract(svr, identityset.PrivateKey(13), 1, svr.bc.TipHeight(), contractCode)

	testData := []struct {
		input  []interface{}
		result uint64
	}{
		{
			input: []interface{}{
				map[string]interface{}{
					"from":     web3Util.ioAddrToEthAddr(identityset.Address(0).String()),
					"to":       web3Util.ioAddrToEthAddr(identityset.Address(28).String()),
					"gas":      "0x0",
					"gasPrice": "0x0",
					"value":    "0x0",
					"data":     "0x1123123c"},
				1},
			result: 21000,
		},
		{
			input: []interface{}{
				map[string]interface{}{
					"from":     web3Util.ioAddrToEthAddr(identityset.Address(0).String()),
					"to":       web3Util.ioAddrToEthAddr(identityset.Address(28).String()),
					"gas":      "0x0",
					"gasPrice": "0x0",
					"value":    "0x0",
					"data":     "344933be000000000000000000000000000000000000000000000000000be497a92e9f3300000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000f8be4046fd89199906ca348bcd3822c4b250e246000000000000000000000000000000000000000000000000000000006173a15400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000a00744882684c3e4747faefd68d283ea44099d030000000000000000000000000258866edaf84d6081df17660357ab20a07d0c80"},
				1},
			result: 36000,
		},
		{
			input: []interface{}{
				map[string]interface{}{
					"from":     web3Util.ioAddrToEthAddr(identityset.Address(0).String()),
					"to":       web3Util.ioAddrToEthAddr(contract),
					"gas":      "0x0",
					"gasPrice": "0x0",
					"value":    "0x0",
					"data":     "0x6d4ce63c"},
				1},
			result: 21000,
		},
	}

	for _, v := range testData {
		ret, err := estimateGas(svr, v.input)
		require.NoError(err)
		require.Equal(ret, uint64ToHex(v.result))
	}
}

func TestSendRawTransaction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{"f8600180830186a09412745fec82b585f239c01090882eb40702c32b04808025a0b0e1aab5b64d744ae01fc9f1c3e9919844a799e90c23129d611f7efe6aec8a29a0195e28d22d9b280e00d501ff63525bb76f5c87b8646c89d5d9c5485edcb1b498"}
	res, _ := sendRawTransaction(svr, testData)
	require.Equal(res, "0x778fd5a054e74e9055bf68ef5f9d559fa306e8ba7dee608d0a3624cca0b63b3e")
}

func TestGetCode(t *testing.T) {
	var web3Util web3Utils
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()
	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContract(svr, identityset.PrivateKey(13), 1, svr.bc.TipHeight(), contractCode)

	testData := []interface{}{web3Util.ioAddrToEthAddr(contract), 1}
	ret, _ := getCode(svr, testData)
	require.Contains(contractCode, removeHexPrefix(ret.(string)))
}

func TestGetNodeInfo(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	_, err := getNodeInfo(svr, nil)
	require.NoError(err)
}

func TestGetBlockTransactionCountByHash(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	header, _ := svr.bc.BlockHeaderByHeight(1)
	blkHash := header.HashBlock()

	testData := []interface{}{"0x" + hex.EncodeToString(blkHash[:]), 1}
	ret, err := getBlockTransactionCountByHash(svr, testData)
	require.NoError(err)
	require.Equal(ret, uint64ToHex(2))
}

func TestGetBlockByHash(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	header, _ := svr.bc.BlockHeaderByHeight(1)
	blkHash := header.HashBlock()

	testData := []interface{}{"0x" + hex.EncodeToString(blkHash[:]), false}
	ret, err := getBlockByHash(svr, testData)
	require.NoError(err)
	ans, _ := ret.(blockObject)
	require.Equal(ans.Hash, "0x"+hex.EncodeToString(blkHash[:]))
	require.Equal(len(ans.Transactions), 2)
}

func TestGetTransactionByHash(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{"0x" + hex.EncodeToString(transferHash1[:]), false}
	ret, err := getTransactionByHash(svr, testData)
	require.NoError(err)
	require.Equal(ret.(transactionObject).Hash, "0x"+hex.EncodeToString(transferHash1[:]))
}

func TestGetLogs(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{
		map[string]interface{}{
			"fromBlock": "0x1",
		}}
	ret, err := getLogs(svr, testData)
	require.NoError(err)
	require.Equal(len(ret.([]logsObject)), 4)
}

func TestGetTransactionReceipt(t *testing.T) {
	var web3Util web3Utils
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{"0x" + hex.EncodeToString(transferHash1[:]), 1}
	ret, err := getTransactionReceipt(svr, testData)
	require.NoError(err)
	ans := ret.(receiptObject)
	require.Equal(ans.TransactionHash, "0x"+hex.EncodeToString(transferHash1[:]))
	require.Equal(ans.From, web3Util.ioAddrToEthAddr(identityset.Address(27).String()))
	require.Equal(*ans.To, web3Util.ioAddrToEthAddr(identityset.Address(30).String()))
}

func TestGetTransactionByBlockHashAndIndex(t *testing.T) {
	var web3Util web3Utils
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	header, _ := svr.bc.BlockHeaderByHeight(1)
	blkHash := header.HashBlock()

	testData := []interface{}{"0x" + hex.EncodeToString(blkHash[:]), "0x0"}
	ret, err := getTransactionByBlockHashAndIndex(svr, testData)
	ans := ret.(transactionObject)
	require.NoError(err)
	require.Equal(ans.Hash, "0x"+hex.EncodeToString(transferHash1[:]))
	require.Equal(ans.From, web3Util.ioAddrToEthAddr(identityset.Address(27).String()))
	require.Equal(*ans.To, web3Util.ioAddrToEthAddr(identityset.Address(30).String()))
	require.Equal(ans.Gas, uint64ToHex(20000))
	require.Equal(ans.GasPrice, uint64ToHex(0))
}

func TestGetTransactionByBlockNumberAndIndex(t *testing.T) {
	var web3Util web3Utils
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{"0x1", "0x0"}
	ret, err := getTransactionByBlockNumberAndIndex(svr, testData)
	ans := ret.(transactionObject)
	require.NoError(err)
	require.Equal(ans.Hash, "0x"+hex.EncodeToString(transferHash1[:]))
	require.Equal(ans.From, web3Util.ioAddrToEthAddr(identityset.Address(27).String()))
	require.Equal(*ans.To, web3Util.ioAddrToEthAddr(identityset.Address(30).String()))
	require.Equal(ans.Gas, uint64ToHex(20000))
	require.Equal(ans.GasPrice, uint64ToHex(0))
}

func TestNewfilter(t *testing.T) {
	require := require.New(t)
	testData := []interface{}{
		map[string]interface{}{
			"fromBlock": "0x1",
		}}
	ret, err := newFilter(nil, testData)
	require.NoError(err)
	require.Equal(ret, "0xd0b3b2603922858ffca89c131c43e47d5ff435cc")
}

func TestNewBlockFilter(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	ret, err := newBlockFilter(svr, nil)
	require.NoError(err)
	require.Equal(ret, "0x287f37d1471029509609447495afbf75714df258")
}

func TestGetFilterChanges(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	filterReq := []interface{}{
		map[string]interface{}{
			"fromBlock": "0x1",
		}}
	filterID1, _ := newFilter(nil, filterReq)
	ret, err := getFilterChanges(svr, []interface{}{filterID1})
	require.NoError(err)
	require.Equal(len(ret.([]logsObject)), 4)

	filterID2, _ := newBlockFilter(svr, nil)
	ret2, err := getFilterChanges(svr, []interface{}{filterID2})
	require.NoError(err)
	require.Equal(len(ret2.([]interface{})), 0)
}

func TestGetFilterLogs(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServer(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	filterReq := []interface{}{
		map[string]interface{}{
			"fromBlock": "0x1",
		}}
	filterID, _ := newFilter(nil, filterReq)
	testData := []interface{}{filterID}
	ret, err := getFilterLogs(svr, testData)
	require.NoError(err)
	require.Equal(len(ret.([]logsObject)), 4)
}
