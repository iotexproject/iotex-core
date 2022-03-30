package api

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	_testContractIoAddr  = "io1ryygckqjw06720cg9j6tkwtprxu4jgcag4w6vn"
	_testSenderIoAddr, _ = address.FromString("io154mvzs09vkgn0hw6gg3ayzw5w39jzp47f8py9v")
	_testBlkHash, _      = hash.HexStringToHash256("c4aace64c1f4d7c0b6ebe74ba01e00e27c7ff4b2552c36ef617f38f0f2b1ebb3")
	_testTxHash, _       = hash.HexStringToHash256("25bef7a7e20402a625973613b19bbc1793ed3a38cad270abf623222120a10fd0")

	_testTopic1, _ = hash.HexStringToHash256("ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
	_testTopic2, _ = hash.HexStringToHash256("0000000000000000000000008a68e01add9adc8b887025dc54c36cfa91432f58")
	_testTopic3, _ = hash.HexStringToHash256("000000000000000000000000567ff65f8b4bec33b9925cad6f7ec3c45ac79b26")

	_testPubKey, _ = crypto.HexStringToPublicKey("04e9f906040bf6f1df25d6fff3f36f6aa135060ff54acf96564a2a298e469ac7162c78564903d4cf39d976493b44906a2bb553997e12b747439d173adcd02d6552")
)

func TestWeb3ResponseMarshal(t *testing.T) {
	require := require.New(t)

	t.Run("Result", func(t *testing.T) {
		res, err := json.Marshal(&web3Response{
			id:     1,
			result: false,
			err:    nil,
		})
		require.NoError(err)
		require.JSONEq(`
		{
			"jsonrpc":"2.0",
			"id":1,
			"result":false
		 }
		`, string(res))
	})

	t.Run("Error", func(t *testing.T) {
		res, err := json.Marshal(&web3Response{
			id:     1,
			result: nil,
			err:    errInvalidBlock,
		})
		require.NoError(err)
		require.JSONEq(`
		{
			"jsonrpc":"2.0",
			"id":1,
			"error":{
			   "code":-32603,
			   "message":"invalid block"
			}
		 }
		`, string(res))
	})
}

func TestBlockObjectMarshal(t *testing.T) {
	require := require.New(t)

	blkMeta := &iotextypes.BlockMeta{
		Hash:              "a52101ae81a5cf4e054709456adb5e8fcbb0c707f02e5f292d0bd22ddd817076",
		Height:            1,
		Timestamp:         timestamppb.New(time.Date(2011, 1, 26, 0, 0, 0, 0, time.UTC)),
		NumActions:        2,
		ProducerAddress:   "io1juvx5g063eu4ts832nukp4vgcwk2gnc5cu9ayd",
		TransferAmount:    "10",
		TxRoot:            "2a4e3b26aa302bf1974b850397e4ec24aa77517e4ea367e3f7f764b2c59f1eb5",
		ReceiptRoot:       "0c26064b778ca775ed2f4220882ce20ced34f806ceb5edf67a7fb4cdb7b1a5dc",
		DeltaStateDigest:  "900d80ab3bb6d12a98ae177268610b28a14c9ef84fb891a9c809d6d863d79cd3",
		LogsBloom:         _zeroLogsBloom,
		PreviousBlockHash: "1f20ad92a25748c2459aa6820a4fe5a25a1c57702045f4ab910dc88df5a04fce",
		GasLimit:          20000,
		GasUsed:           10000,
	}

	t.Run("BlockWithoutDetail", func(t *testing.T) {
		res, err := json.Marshal(&blockObject{
			blkMeta:      blkMeta,
			logsBloom:    _zeroLogsBloom,
			transactions: []interface{}{string("0x2133ee7ff4562535166e3f16fd7407c19e5ed1acd036f78d3528a5a40e40ad42")},
		})
		require.NoError(err)
		require.JSONEq(`
		{
			"author":"0x97186A21fA8E7955C0f154f960d588C3ACA44f14",
			"number":"0x1",
			"hash":"0xa52101ae81a5cf4e054709456adb5e8fcbb0c707f02e5f292d0bd22ddd817076",
			"parentHash":"0x1f20ad92a25748c2459aa6820a4fe5a25a1c57702045f4ab910dc88df5a04fce",
			"sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
			"logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			"transactionsRoot":"0x2a4e3b26aa302bf1974b850397e4ec24aa77517e4ea367e3f7f764b2c59f1eb5",
			"stateRoot":"0x900d80ab3bb6d12a98ae177268610b28a14c9ef84fb891a9c809d6d863d79cd3",
			"receiptsRoot":"0x0c26064b778ca775ed2f4220882ce20ced34f806ceb5edf67a7fb4cdb7b1a5dc",
			"miner":"0x97186A21fA8E7955C0f154f960d588C3ACA44f14",
			"difficulty":"0xfffffffffffffffffffffffffffffffe",
			"totalDifficulty":"0xff14700000000000000000000000486001d72",
			"extraData":"0x",
			"size":"0x2",
			"gasLimit":"0x4e20",
			"gasUsed":"0x2710",
			"timestamp":"0x4d3f6400",
			"transactions":[
			   "0x2133ee7ff4562535166e3f16fd7407c19e5ed1acd036f78d3528a5a40e40ad42"
			],
			"step":"373422302",
			"uncles":[
			   
			]
		 }
		`, string(res))
	})

	t.Run("BlockWithDetail", func(t *testing.T) {
		receipt := &action.Receipt{
			Status:          1,
			BlockHeight:     16,
			ActionHash:      _testTxHash,
			GasConsumed:     21000,
			ContractAddress: _testContractIoAddr,
			TxIndex:         1,
		}
		tx := &transactionObject{
			blockHash: _testBlkHash,
			to:        nil,
			ethTx:     types.NewContractCreation(1, big.NewInt(10), 21000, big.NewInt(0), []byte{}),
			receipt:   receipt,
			pubkey:    _testPubKey,
			signature: []byte("69d89a0af27dcaa67f1b62a383594d97599aadd2b7b164cb4112aa8ddfd42f895649075cae1b7216c43a491c5e9be68d1d9a27b863d71155ecdd7c95dab5394f01"),
		}
		res, err := json.Marshal(&blockObject{
			blkMeta:      blkMeta,
			logsBloom:    _zeroLogsBloom,
			transactions: []interface{}{tx},
		})
		require.NoError(err)
		require.JSONEq(`
		{
			"author":"0x97186A21fA8E7955C0f154f960d588C3ACA44f14",
			"number":"0x1",
			"hash":"0xa52101ae81a5cf4e054709456adb5e8fcbb0c707f02e5f292d0bd22ddd817076",
			"parentHash":"0x1f20ad92a25748c2459aa6820a4fe5a25a1c57702045f4ab910dc88df5a04fce",
			"sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
			"logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			"transactionsRoot":"0x2a4e3b26aa302bf1974b850397e4ec24aa77517e4ea367e3f7f764b2c59f1eb5",
			"stateRoot":"0x900d80ab3bb6d12a98ae177268610b28a14c9ef84fb891a9c809d6d863d79cd3",
			"receiptsRoot":"0x0c26064b778ca775ed2f4220882ce20ced34f806ceb5edf67a7fb4cdb7b1a5dc",
			"miner":"0x97186A21fA8E7955C0f154f960d588C3ACA44f14",
			"difficulty":"0xfffffffffffffffffffffffffffffffe",
			"totalDifficulty":"0xff14700000000000000000000000486001d72",
			"extraData":"0x",
			"size":"0x2",
			"gasLimit":"0x4e20",
			"gasUsed":"0x2710",
			"timestamp":"0x4d3f6400",
			"transactions":[
			   {
				  "hash":"0x25bef7a7e20402a625973613b19bbc1793ed3a38cad270abf623222120a10fd0",
				  "nonce":"0x1",
				  "blockHash":"0xc4aace64c1f4d7c0b6ebe74ba01e00e27c7ff4b2552c36ef617f38f0f2b1ebb3",
				  "blockNumber":"0x10",
				  "transactionIndex":"0x1",
				  "from":"0x0666dba65b0ef88d11cdcbe857ffb6618310dcfa",
				  "to":null,
				  "value":"0xa",
				  "gasPrice":"0x0",
				  "gas":"0x5208",
				  "input":"0x",
				  "r":"0x3639643839613061663237646361613637663162363261333833353934643937",
				  "s":"0x3539396161646432623762313634636234313132616138646466643432663839",
				  "v":"0x35"
			   }
			],
			"step":"373422302",
			"uncles":[
			   
			]
		 }
		`, string(res))
	})
}

func TestTransactionObjectMarshal(t *testing.T) {
	require := require.New(t)

	receipt := &action.Receipt{
		Status:          1,
		BlockHeight:     16,
		ActionHash:      _testTxHash,
		GasConsumed:     21000,
		ContractAddress: _testContractIoAddr,
		TxIndex:         1,
	}

	t.Run("ContractCreation", func(t *testing.T) {
		res, err := json.Marshal(&transactionObject{
			blockHash: _testBlkHash,
			to:        nil,
			ethTx:     types.NewContractCreation(1, big.NewInt(10), 21000, big.NewInt(0), []byte{}),
			receipt:   receipt,
			pubkey:    _testPubKey,
			signature: []byte("69d89a0af27dcaa67f1b62a383594d97599aadd2b7b164cb4112aa8ddfd42f895649075cae1b7216c43a491c5e9be68d1d9a27b863d71155ecdd7c95dab5394f01"),
		})
		require.NoError(err)
		require.JSONEq(`
		{
			"hash":"0x25bef7a7e20402a625973613b19bbc1793ed3a38cad270abf623222120a10fd0",
			"nonce":"0x1",
			"blockHash":"0xc4aace64c1f4d7c0b6ebe74ba01e00e27c7ff4b2552c36ef617f38f0f2b1ebb3",
			"blockNumber":"0x10",
			"transactionIndex":"0x1",
			"from":"0x0666dba65b0ef88d11cdcbe857ffb6618310dcfa",
			"to":null,
			"value":"0xa",
			"gasPrice":"0x0",
			"gas":"0x5208",
			"input":"0x",
			"r":"0x3639643839613061663237646361613637663162363261333833353934643937",
			"s":"0x3539396161646432623762313634636234313132616138646466643432663839",
			"v":"0x35"
		 }
		`, string(res))
	})
}

func TestReceiptObjectMarshal(t *testing.T) {
	require := require.New(t)

	receipt := &action.Receipt{
		Status:          1,
		BlockHeight:     16,
		ActionHash:      _testTxHash,
		GasConsumed:     21000,
		ContractAddress: _testContractIoAddr,
		TxIndex:         1,
	}

	t.Run("ContractCreation", func(t *testing.T) {
		contractEthaddr, _ := ioAddrToEthAddr(_testContractIoAddr)
		res, err := json.Marshal(&receiptObject{
			blockHash:       _testBlkHash,
			from:            _testSenderIoAddr,
			to:              nil,
			contractAddress: &contractEthaddr,
			logsBloom:       "00008000000100000400000000000040000000000000000000000000000000000000000001000200000400000000000000000000001000000000000000001000000000001000000000200000004000000000000000000101000000000000000008000008000208000000000000400000000000000000000000000000000000000000080010000000000200010000000000000500000000000000000000000000004080000000000000001000000800020000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000400000000000000000000000000080000400010200000",
			receipt:         receipt,
		})
		require.NoError(err)
		require.JSONEq(`
		{
			"transactionIndex":"0x1",
			"transactionHash":"0x25bef7a7e20402a625973613b19bbc1793ed3a38cad270abf623222120a10fd0",
			"blockHash":"0xc4aace64c1f4d7c0b6ebe74ba01e00e27c7ff4b2552c36ef617f38f0f2b1ebb3",
			"blockNumber":"0x10",
			"from":"0xa576c141e5659137ddda4223d209d4744b2106be",
			"to":null,
			"cumulativeGasUsed":"0x5208",
			"gasUsed":"0x5208",
			"contractAddress":"0x19088c581273F5E53f082CB4BB396119b959231D",
			"logsBloom":"00008000000100000400000000000040000000000000000000000000000000000000000001000200000400000000000000000000001000000000000000001000000000001000000000200000004000000000000000000101000000000000000008000008000208000000000000400000000000000000000000000000000000000000080010000000000200010000000000000500000000000000000000000000004080000000000000001000000800020000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000400000000000000000000000000080000400010200000",
			"logs":[
			   
			],
			"status":"0x1"
		 }
		`, string(res))
	})

	t.Run("ContractExecution", func(t *testing.T) {
		receipt.AddLogs(&action.Log{
			Address:            _testContractIoAddr,
			Topics:             action.Topics{_testTopic1, _testTopic2, _testTopic3},
			Data:               []byte("test"),
			BlockHeight:        16,
			ActionHash:         _testTxHash,
			Index:              3,
			TxIndex:            1,
			NotFixTopicCopyBug: false,
		})
		contractEthaddr, _ := ioAddrToEthAddr(_testContractIoAddr)
		res, err := json.Marshal(&receiptObject{
			blockHash:       _testBlkHash,
			from:            _testSenderIoAddr,
			to:              &contractEthaddr,
			contractAddress: nil,
			logsBloom:       "00008000000100000400000000000040000000000000000000000000000000000000000001000200000400000000000000000000001000000000000000001000000000001000000000200000004000000000000000000101000000000000000008000008000208000000000000400000000000000000000000000000000000000000080010000000000200010000000000000500000000000000000000000000004080000000000000001000000800020000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000400000000000000000000000000080000400010200000",
			receipt:         receipt,
		})
		require.NoError(err)
		require.JSONEq(`
		{
			"transactionIndex":"0x1",
			"transactionHash":"0x25bef7a7e20402a625973613b19bbc1793ed3a38cad270abf623222120a10fd0",
			"blockHash":"0xc4aace64c1f4d7c0b6ebe74ba01e00e27c7ff4b2552c36ef617f38f0f2b1ebb3",
			"blockNumber":"0x10",
			"from":"0xa576c141e5659137ddda4223d209d4744b2106be",
			"to":"0x19088c581273F5E53f082CB4BB396119b959231D",
			"cumulativeGasUsed":"0x5208",
			"gasUsed":"0x5208",
			"contractAddress":null,
			"logsBloom":"00008000000100000400000000000040000000000000000000000000000000000000000001000200000400000000000000000000001000000000000000001000000000001000000000200000004000000000000000000101000000000000000008000008000208000000000000400000000000000000000000000000000000000000080010000000000200010000000000000500000000000000000000000000004080000000000000001000000800020000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000400000000000000000000000000080000400010200000",
			"logs":[
			   {
				  "removed":false,
				  "logIndex":"0x3",
				  "transactionIndex":"0x1",
				  "transactionHash":"0x25bef7a7e20402a625973613b19bbc1793ed3a38cad270abf623222120a10fd0",
				  "blockHash":"0xc4aace64c1f4d7c0b6ebe74ba01e00e27c7ff4b2552c36ef617f38f0f2b1ebb3",
				  "blockNumber":"0x10",
				  "address":"0x19088c581273F5E53f082CB4BB396119b959231D",
				  "data":"0x74657374",
				  "topics":[
					 "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
					 "0x0000000000000000000000008a68e01add9adc8b887025dc54c36cfa91432f58",
					 "0x000000000000000000000000567ff65f8b4bec33b9925cad6f7ec3c45ac79b26"
				  ]
			   }
			],
			"status":"0x1"
		 }
		`, string(res))
	})
}

func TestLogsObjectMarshal(t *testing.T) {
	require := require.New(t)

	res, err := json.Marshal(&logsObjectV2{
		blockHash: _testBlkHash,
		log: &action.Log{
			Address:            _testContractIoAddr,
			Topics:             action.Topics{_testTopic1, _testTopic2, _testTopic3},
			Data:               []byte("test"),
			BlockHeight:        2,
			ActionHash:         _testTxHash,
			Index:              3,
			TxIndex:            1,
			NotFixTopicCopyBug: false,
		},
	})
	require.NoError(err)
	require.JSONEq(`
	{
		"removed":false,
		"transactionIndex":"0x1",
		"logIndex":"0x3",
		"transactionHash":"0x25bef7a7e20402a625973613b19bbc1793ed3a38cad270abf623222120a10fd0",
		"blockHash":"0xc4aace64c1f4d7c0b6ebe74ba01e00e27c7ff4b2552c36ef617f38f0f2b1ebb3",
		"blockNumber":"0x2",
		"address":"0x19088c581273F5E53f082CB4BB396119b959231D",
		"data":"0x74657374",
		"topics":[
		   "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
		   "0x0000000000000000000000008a68e01add9adc8b887025dc54c36cfa91432f58",
		   "0x000000000000000000000000567ff65f8b4bec33b9925cad6f7ec3c45ac79b26"
		]
	 }
	`, string(res))
}
