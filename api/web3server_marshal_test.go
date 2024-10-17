package api

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
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

	var (
		receiptRoot, _       = hex.DecodeString("0c26064b778ca775ed2f4220882ce20ced34f806ceb5edf67a7fb4cdb7b1a5dc")
		deltaStateDigest, _  = hex.DecodeString("900d80ab3bb6d12a98ae177268610b28a14c9ef84fb891a9c809d6d863d79cd3")
		previousBlockHash, _ = hex.DecodeString("1f20ad92a25748c2459aa6820a4fe5a25a1c57702045f4ab910dc88df5a04fce")
	)
	tsf := action.NewExecution(action.EmptyAddress,
		unit.ConvertIotxToRau(1000),
		[]byte{})
	evlp := (&action.EnvelopeBuilder{}).SetNonce(2).SetGasPrice(unit.ConvertIotxToRau(1)).
		SetGasLimit(21000).SetAction(tsf).Build()
	sevlp, err := action.Sign(evlp, identityset.PrivateKey(24))
	require.NoError(err)
	ra := (&block.RunnableActionsBuilder{}).AddActions([]*action.SealedEnvelope{sevlp}...).Build()
	blk, err := block.NewBuilder(ra).
		SetHeight(uint64(1)).
		SetTimestamp(time.Date(2011, 1, 26, 0, 0, 0, 0, time.UTC)).
		SetVersion(1).
		SetReceiptRoot(hash.BytesToHash256(receiptRoot)).
		SetDeltaStateDigest(hash.BytesToHash256(deltaStateDigest)).
		SetPrevBlockHash(hash.BytesToHash256(previousBlockHash)).
		SetBaseFee(big.NewInt(10)).
		SetBlobGasUsed(20).SetExcessBlobGas(100).
		SignAndBuild(identityset.PrivateKey(28))
	require.NoError(err)
	txHash, err := sevlp.Hash()
	require.NoError(err)
	blk.Receipts = []*action.Receipt{{
		Status:          1,
		BlockHeight:     1,
		ActionHash:      txHash,
		GasConsumed:     21000,
		ContractAddress: _testContractIoAddr,
		TxIndex:         1,
	}}

	t.Run("BlockWithoutDetail", func(t *testing.T) {
		res, err := json.Marshal(&getBlockResult{
			blk:          &blk,
			transactions: []interface{}{string("0x2133ee7ff4562535166e3f16fd7407c19e5ed1acd036f78d3528a5a40e40ad42")},
		})
		require.NoError(err)
		require.JSONEq(`
		{
			"author":"0x1e14d5373E1AF9Cc77F0032aD2cd0FBA8be5Ea2e",
			"number":"0x1",
			"hash":"0xe234481999d8eb5731b5a22b9991d50e08ca46e9768145e2529c35c39e8e53d3",
			"parentHash":"0x1f20ad92a25748c2459aa6820a4fe5a25a1c57702045f4ab910dc88df5a04fce",
			"sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
			"logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			"transactionsRoot":"0x8e281592f4d4585e195c1d6c499b891f856c55e77aa2880e61d27755fe09186f",
			"stateRoot":"0x900d80ab3bb6d12a98ae177268610b28a14c9ef84fb891a9c809d6d863d79cd3",
			"receiptsRoot":"0x0c26064b778ca775ed2f4220882ce20ced34f806ceb5edf67a7fb4cdb7b1a5dc",
			"miner":"0x1e14d5373E1AF9Cc77F0032aD2cd0FBA8be5Ea2e",
			"difficulty":"0xfffffffffffffffffffffffffffffffe",
			"totalDifficulty":"0xff14700000000000000000000000486001d72",
			"extraData":"0x",
			"size":"0x1",
			"gasLimit":"0x5208",
			"gasUsed":"0x5208",
			"timestamp":"0x4d3f6400",
			"transactions":[
			   "0x2133ee7ff4562535166e3f16fd7407c19e5ed1acd036f78d3528a5a40e40ad42"
			],
			"step":"373422302",
			"uncles":[

			],
			"baseFeePerGas": "0xa",
			"blobGasUsed": "0x14",
			"excessBlobGas": "0x64"
		 }
		`, string(res))
	})

	t.Run("BlockWithDetail", func(t *testing.T) {
		raw := types.NewTx(&types.LegacyTx{
			Nonce:    2,
			Value:    unit.ConvertIotxToRau(1000),
			Gas:      21000,
			GasPrice: unit.ConvertIotxToRau(1),
			Data:     []byte{},
		})
		signer, err := action.NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, 0)
		require.NoError(err)
		ethTx, err := action.RawTxToSignedTx(raw, signer, sevlp.Signature())
		require.NoError(err)
		tx := &getTransactionResult{
			blockHash: &_testBlkHash,
			to:        nil,
			ethTx:     ethTx,
			receipt:   blk.Receipts[0],
			pubkey:    sevlp.SrcPubkey(),
		}
		res, err := json.Marshal(&getBlockResult{
			blk:          &blk,
			transactions: []interface{}{tx},
		})
		require.NoError(err)
		require.JSONEq(`
		{
			"author":"0x1e14d5373E1AF9Cc77F0032aD2cd0FBA8be5Ea2e",
			"number":"0x1",
			"hash":"0xe234481999d8eb5731b5a22b9991d50e08ca46e9768145e2529c35c39e8e53d3",
			"parentHash":"0x1f20ad92a25748c2459aa6820a4fe5a25a1c57702045f4ab910dc88df5a04fce",
			"sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
			"logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			"transactionsRoot":"0x8e281592f4d4585e195c1d6c499b891f856c55e77aa2880e61d27755fe09186f",
			"stateRoot":"0x900d80ab3bb6d12a98ae177268610b28a14c9ef84fb891a9c809d6d863d79cd3",
			"receiptsRoot":"0x0c26064b778ca775ed2f4220882ce20ced34f806ceb5edf67a7fb4cdb7b1a5dc",
			"miner":"0x1e14d5373E1AF9Cc77F0032aD2cd0FBA8be5Ea2e",
			"difficulty":"0xfffffffffffffffffffffffffffffffe",
			"totalDifficulty":"0xff14700000000000000000000000486001d72",
			"extraData":"0x",
			"size":"0x1",
			"gasLimit":"0x5208",
			"gasUsed":"0x5208",
			"timestamp":"0x4d3f6400",
			"transactions":[
			   {
				  "hash":"0x8e281592f4d4585e195c1d6c499b891f856c55e77aa2880e61d27755fe09186f",
				  "nonce":"0x2",
				  "blockHash":"0xc4aace64c1f4d7c0b6ebe74ba01e00e27c7ff4b2552c36ef617f38f0f2b1ebb3",
				  "blockNumber":"0x1",
				  "transactionIndex":"0x1",
				  "from":"0x2b5e18f6f541dce2b7d6c19203f886cf93319c61",
				  "to":null,
				  "value":"0x3635c9adc5dea00000",
				  "gasPrice":"0xde0b6b3a7640000",
				  "gas":"0x5208",
				  "input":"0x",
				  "r":"0xd7b08f7b37c7fac89a2d0b819225f459cfab6d6d14307893ce711269c26c1c67",
				  "s":"0x5f9b182b55d50734447f0859b7261701b0195357048d902a723dcb40616ebdda",
				  "v":"0x1c",
				  "type": "0x0"
			   }
			],
			"step":"373422302",
			"uncles":[

			],
			"baseFeePerGas": "0xa",
			"blobGasUsed": "0x14",
			"excessBlobGas": "0x64"
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
		raw := types.NewTx(&types.LegacyTx{
			Nonce:    1,
			Value:    big.NewInt(10),
			Gas:      21000,
			GasPrice: big.NewInt(0),
			Data:     []byte{},
		})
		sig, _ := hex.DecodeString("363964383961306166323764636161363766316236326133383335393464393735393961616464326237623136346362343131326161386464666434326638391b")
		signer, err := action.NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, 4690)
		require.NoError(err)
		tx, err := action.RawTxToSignedTx(raw, signer, sig)
		require.NoError(err)
		res, err := json.Marshal(&getTransactionResult{
			blockHash: &_testBlkHash,
			to:        nil,
			ethTx:     tx,
			receipt:   receipt,
			pubkey:    _testPubKey,
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
			"v":"0x24c7",
			"chainId":"0x1252",
			"type":"0x0"
		 }
		`, string(res))
	})
	t.Run("ISSUE3932", func(t *testing.T) {
		sig, _ := hex.DecodeString("00d50c3169003e7c9c9392069a1c9e5251c8f558120cceb9aa312f0fa46247701f4cad47a5540cdda5459f7a46c62df752ca588e0b73722ec767ff08994134a41b")
		blkHash, _ := hash.HexStringToHash256("bdcf722cbc0dc9b9eb5b46ad4af9e15944a886310b1cb2227129070c0e4b7de5")
		contract := "0xc3527348de07d591c9d567ce1998efa2031b8675"
		data, _ := hex.DecodeString("1fad948c0000000000000000000000000000000000000000000000000000000000000040000000000000000000000000ec4daee51c4bf81bd00af165f8ca66823ee3b12a000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000200000000000000000000000003e24491a4f2a946e30baf624fda6b4484d106c12000000000000000000000000000000000000000000000000000000000000005b000000000000000000000000000000000000000000000000000000000000016000000000000000000000000000000000000000000000000000000000000001800000000000000000000000000000000000000000000000000000000000009fd90000000000000000000000000000000000000000000000000000000000011da4000000000000000000000000000000000000000000000000000000000000db26000000000000000000000000000000000000000000000000000000e8d4a51000000000000000000000000000000000000000000000000000000000e8d4a510000000000000000000000000000000000000000000000000000000000000000240000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000084b61d27f6000000000000000000000000065e1164818487818e6ba714e8d80b91718ad75800000000000000000000000000000000000000000000000000038d7ea4c6800000000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000095ccc7012efb2e65aa31752f3ac01e23817c08a47500000000000000000000000000000000000000000000000000000000650af9f8000000000000000000000000000000000000000000000000000000006509a878b5acba7277159ae6fa661ed1988cc10ac2c96c58dc332bde2a6dc0d8531ea3924d9d04cda681c271411250ae7d9e9aea47661dba67a66f08d19804a255e45c561b0000000000000000000000000000000000000000000000000000000000000000000000000000000000004160daa88165299ca7e585d5d286cee98b54397b57ac704b74331a48d67651195322ef3884c7d60023333f2542a07936f34edc9efa3cbd19e8cd0f8972c54171a21b00000000000000000000000000000000000000000000000000000000000000")
		pubkey, _ := crypto.HexStringToPublicKey("04806b217cb0b6a675974689fd99549e525d967287eee9a62dc4e598eea981b8158acfe026da7bf58397108abd0607672832c28ef3bc7b5855077f6e67ab5fc096")
		actHash, _ := hash.HexStringToHash256("cbc2560d986d79a46bfd96a08d18c6045b29f97352c1360289e371d9cffd6b6a")
		raw := types.NewTx(&types.LegacyTx{
			Nonce:    305,
			Value:    big.NewInt(0),
			Gas:      297131,
			GasPrice: big.NewInt(1000000000000),
			Data:     data,
		})
		signer, err := action.NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, 0)
		require.NoError(err)
		tx, err := action.RawTxToSignedTx(raw, signer, sig)
		require.NoError(err)
		res, err := json.Marshal(&getTransactionResult{
			blockHash: &blkHash,
			to:        &contract,
			ethTx:     tx,
			receipt: &action.Receipt{
				Status:          1,
				BlockHeight:     22354907,
				ActionHash:      actHash,
				GasConsumed:     196223,
				ContractAddress: "",
				TxIndex:         0,
			},
			pubkey: pubkey,
		})
		require.NoError(err)
		require.JSONEq(`
		{
			"hash": "0xcbc2560d986d79a46bfd96a08d18c6045b29f97352c1360289e371d9cffd6b6a",
			"nonce": "0x131",
			"blockHash": "0xbdcf722cbc0dc9b9eb5b46ad4af9e15944a886310b1cb2227129070c0e4b7de5",
			"blockNumber": "0x1551bdb",
			"transactionIndex": "0x0",
			"from": "0xec4daee51c4bf81bd00af165f8ca66823ee3b12a",
			"to": "0xc3527348de07d591c9d567ce1998efa2031b8675",
			"value": "0x0",
			"gasPrice": "0xe8d4a51000",
			"gas": "0x488ab",
			"input": "0x1fad948c0000000000000000000000000000000000000000000000000000000000000040000000000000000000000000ec4daee51c4bf81bd00af165f8ca66823ee3b12a000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000200000000000000000000000003e24491a4f2a946e30baf624fda6b4484d106c12000000000000000000000000000000000000000000000000000000000000005b000000000000000000000000000000000000000000000000000000000000016000000000000000000000000000000000000000000000000000000000000001800000000000000000000000000000000000000000000000000000000000009fd90000000000000000000000000000000000000000000000000000000000011da4000000000000000000000000000000000000000000000000000000000000db26000000000000000000000000000000000000000000000000000000e8d4a51000000000000000000000000000000000000000000000000000000000e8d4a510000000000000000000000000000000000000000000000000000000000000000240000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000084b61d27f6000000000000000000000000065e1164818487818e6ba714e8d80b91718ad75800000000000000000000000000000000000000000000000000038d7ea4c6800000000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000095ccc7012efb2e65aa31752f3ac01e23817c08a47500000000000000000000000000000000000000000000000000000000650af9f8000000000000000000000000000000000000000000000000000000006509a878b5acba7277159ae6fa661ed1988cc10ac2c96c58dc332bde2a6dc0d8531ea3924d9d04cda681c271411250ae7d9e9aea47661dba67a66f08d19804a255e45c561b0000000000000000000000000000000000000000000000000000000000000000000000000000000000004160daa88165299ca7e585d5d286cee98b54397b57ac704b74331a48d67651195322ef3884c7d60023333f2542a07936f34edc9efa3cbd19e8cd0f8972c54171a21b00000000000000000000000000000000000000000000000000000000000000",
			"r": "0xd50c3169003e7c9c9392069a1c9e5251c8f558120cceb9aa312f0fa4624770",
			"s": "0x1f4cad47a5540cdda5459f7a46c62df752ca588e0b73722ec767ff08994134a4",
			"v": "0x1b",
			"type": "0x0"
		}
		`, string(res))
	})
	t.Run("PendingTransaction", func(t *testing.T) {
		sig, _ := hex.DecodeString("00d50c3169003e7c9c9392069a1c9e5251c8f558120cceb9aa312f0fa46247701f4cad47a5540cdda5459f7a46c62df752ca588e0b73722ec767ff08994134a41b")
		contract := "0xc3527348de07d591c9d567ce1998efa2031b8675"
		data, _ := hex.DecodeString("1fad948c0000000000000000000000000000000000000000000000000000000000000040000000000000000000000000ec4daee51c4bf81bd00af165f8ca66823ee3b12a000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000200000000000000000000000003e24491a4f2a946e30baf624fda6b4484d106c12000000000000000000000000000000000000000000000000000000000000005b000000000000000000000000000000000000000000000000000000000000016000000000000000000000000000000000000000000000000000000000000001800000000000000000000000000000000000000000000000000000000000009fd90000000000000000000000000000000000000000000000000000000000011da4000000000000000000000000000000000000000000000000000000000000db26000000000000000000000000000000000000000000000000000000e8d4a51000000000000000000000000000000000000000000000000000000000e8d4a510000000000000000000000000000000000000000000000000000000000000000240000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000084b61d27f6000000000000000000000000065e1164818487818e6ba714e8d80b91718ad75800000000000000000000000000000000000000000000000000038d7ea4c6800000000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000095ccc7012efb2e65aa31752f3ac01e23817c08a47500000000000000000000000000000000000000000000000000000000650af9f8000000000000000000000000000000000000000000000000000000006509a878b5acba7277159ae6fa661ed1988cc10ac2c96c58dc332bde2a6dc0d8531ea3924d9d04cda681c271411250ae7d9e9aea47661dba67a66f08d19804a255e45c561b0000000000000000000000000000000000000000000000000000000000000000000000000000000000004160daa88165299ca7e585d5d286cee98b54397b57ac704b74331a48d67651195322ef3884c7d60023333f2542a07936f34edc9efa3cbd19e8cd0f8972c54171a21b00000000000000000000000000000000000000000000000000000000000000")
		pubkey, _ := crypto.HexStringToPublicKey("04806b217cb0b6a675974689fd99549e525d967287eee9a62dc4e598eea981b8158acfe026da7bf58397108abd0607672832c28ef3bc7b5855077f6e67ab5fc096")
		raw := types.NewTx(&types.LegacyTx{
			Nonce:    305,
			Value:    big.NewInt(0),
			Gas:      297131,
			GasPrice: big.NewInt(1000000000000),
			Data:     data,
		})
		signer, err := action.NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, 0)
		require.NoError(err)
		tx, err := action.RawTxToSignedTx(raw, signer, sig)
		require.NoError(err)
		res, err := json.Marshal(&getTransactionResult{
			blockHash: nil,
			to:        &contract,
			ethTx:     tx,
			receipt:   nil,
			pubkey:    pubkey,
		})
		require.NoError(err)
		require.JSONEq(`
		{
			"hash": "0xd5de34d026608b4453218f5125f8e22afd6f56710ad9da75d1ac5df127118add",
			"nonce": "0x131",
			"blockHash": null,
			"blockNumber": null,
			"transactionIndex": null,
			"from": "0xec4daee51c4bf81bd00af165f8ca66823ee3b12a",
			"to": "0xc3527348de07d591c9d567ce1998efa2031b8675",
			"value": "0x0",
			"gasPrice": "0xe8d4a51000",
			"gas": "0x488ab",
			"input": "0x1fad948c0000000000000000000000000000000000000000000000000000000000000040000000000000000000000000ec4daee51c4bf81bd00af165f8ca66823ee3b12a000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000200000000000000000000000003e24491a4f2a946e30baf624fda6b4484d106c12000000000000000000000000000000000000000000000000000000000000005b000000000000000000000000000000000000000000000000000000000000016000000000000000000000000000000000000000000000000000000000000001800000000000000000000000000000000000000000000000000000000000009fd90000000000000000000000000000000000000000000000000000000000011da4000000000000000000000000000000000000000000000000000000000000db26000000000000000000000000000000000000000000000000000000e8d4a51000000000000000000000000000000000000000000000000000000000e8d4a510000000000000000000000000000000000000000000000000000000000000000240000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000084b61d27f6000000000000000000000000065e1164818487818e6ba714e8d80b91718ad75800000000000000000000000000000000000000000000000000038d7ea4c6800000000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000095ccc7012efb2e65aa31752f3ac01e23817c08a47500000000000000000000000000000000000000000000000000000000650af9f8000000000000000000000000000000000000000000000000000000006509a878b5acba7277159ae6fa661ed1988cc10ac2c96c58dc332bde2a6dc0d8531ea3924d9d04cda681c271411250ae7d9e9aea47661dba67a66f08d19804a255e45c561b0000000000000000000000000000000000000000000000000000000000000000000000000000000000004160daa88165299ca7e585d5d286cee98b54397b57ac704b74331a48d67651195322ef3884c7d60023333f2542a07936f34edc9efa3cbd19e8cd0f8972c54171a21b00000000000000000000000000000000000000000000000000000000000000",
			"r": "0xd50c3169003e7c9c9392069a1c9e5251c8f558120cceb9aa312f0fa4624770",
			"s": "0x1f4cad47a5540cdda5459f7a46c62df752ca588e0b73722ec767ff08994134a4",
			"v": "0x1b",
			"type": "0x0"
		}
		`, string(res))
	})
	t.Run("AccessListTx", func(t *testing.T) {
		to := common.BigToAddress(big.NewInt(2))
		toStr := to.String()
		raw := types.NewTx(&types.AccessListTx{
			ChainID:  big.NewInt(4690),
			Nonce:    1,
			Value:    big.NewInt(10),
			Gas:      21000,
			GasPrice: big.NewInt(0),
			Data:     []byte{},
			To:       &to,
			AccessList: types.AccessList{
				{Address: common.BigToAddress(big.NewInt(1)), StorageKeys: []common.Hash{common.BigToHash(big.NewInt(1))}},
			},
		})

		signer, err := action.NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, 4690)
		require.NoError(err)
		tx, err := types.SignTx(raw, signer, identityset.PrivateKey(1).EcdsaPrivateKey().(*ecdsa.PrivateKey))
		require.NoError(err)
		res, err := json.Marshal(&getTransactionResult{
			blockHash: &_testBlkHash,
			to:        &toStr,
			ethTx:     tx,
			receipt:   receipt,
			pubkey:    _testPubKey,
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
			"to":"0x0000000000000000000000000000000000000002",
			"value":"0xa",
			"gasPrice":"0x0",
			"gas":"0x5208",
			"input":"0x",
			"r":"0xa5cc935e61c499b8ee016f44eb77400cd42fa44e54c37370c9b002ad661b2485",
			"s":"0x1a295c82f4c2c9b96477ea450c256f5544afdde6e712c515a213fc8888879d96",
			"v":"0x0",
			"chainId":"0x1252",
			"type":"0x1",
			"yParity": "0x0",
			"accessList": [
				{
					"address": "0x0000000000000000000000000000000000000001",
					"storageKeys": ["0x0000000000000000000000000000000000000000000000000000000000000001"]
				}
			]
		 }
		`, string(res))
	})
	t.Run("DynamicFeeTx", func(t *testing.T) {
		to := common.BigToAddress(big.NewInt(2))
		toStr := to.String()
		raw := types.NewTx(&types.DynamicFeeTx{
			ChainID:   big.NewInt(4690),
			Nonce:     1,
			Value:     big.NewInt(10),
			Gas:       21000,
			GasTipCap: big.NewInt(20),
			GasFeeCap: big.NewInt(30),
			Data:      []byte{},
			To:        &to,
			AccessList: types.AccessList{
				{Address: common.BigToAddress(big.NewInt(1)), StorageKeys: []common.Hash{common.BigToHash(big.NewInt(1))}},
			},
		})

		signer, err := action.NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, 4690)
		require.NoError(err)
		tx, err := types.SignTx(raw, signer, identityset.PrivateKey(1).EcdsaPrivateKey().(*ecdsa.PrivateKey))
		require.NoError(err)
		receipt := &action.Receipt{
			Status:            1,
			BlockHeight:       16,
			ActionHash:        _testTxHash,
			GasConsumed:       21000,
			ContractAddress:   _testContractIoAddr,
			TxIndex:           1,
			EffectiveGasPrice: big.NewInt(25),
		}
		res, err := json.Marshal(&getTransactionResult{
			blockHash: &_testBlkHash,
			to:        &toStr,
			ethTx:     tx,
			receipt:   receipt,
			pubkey:    _testPubKey,
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
			"to":"0x0000000000000000000000000000000000000002",
			"value":"0xa",
			"gasPrice":"0x19",
			"gas":"0x5208",
			"input":"0x",
			"r":"0xc5d7508b27bc4a675ab72d3cae3caec37727de81d6d05ab608e8a013a98dabe4",
			"s":"0x6149528d7a506af04fd8e825bdf79a8a3619f9e1b4f5272ba8c76e07807d0ab1",
			"v":"0x0",
			"chainId":"0x1252",
			"type":"0x2",
			"yParity": "0x0",
			"accessList": [
				{
					"address": "0x0000000000000000000000000000000000000001",
					"storageKeys": ["0x0000000000000000000000000000000000000000000000000000000000000001"]
				}
			],
			"maxFeePerGas": "0x1e",
			"maxPriorityFeePerGas": "0x14"
		 }
		`, string(res))
	})
	t.Run("BlobTx", func(t *testing.T) {
		to := common.BigToAddress(big.NewInt(2))
		toStr := to.String()
		var (
			testBlob       = kzg4844.Blob{1, 2, 3, 4}
			testBlobCommit = assertions.MustNoErrorV(kzg4844.BlobToCommitment(testBlob))
			testBlobProof  = assertions.MustNoErrorV(kzg4844.ComputeBlobProof(testBlob, testBlobCommit))
		)
		sidecar := &types.BlobTxSidecar{
			Blobs:       []kzg4844.Blob{testBlob},
			Commitments: []kzg4844.Commitment{testBlobCommit},
			Proofs:      []kzg4844.Proof{testBlobProof},
		}
		raw := types.NewTx(&types.BlobTx{
			ChainID:   uint256.MustFromBig(big.NewInt(4690)),
			Nonce:     1,
			Value:     uint256.MustFromBig(big.NewInt(10)),
			Gas:       21000,
			GasTipCap: uint256.MustFromBig(big.NewInt(20)),
			GasFeeCap: uint256.MustFromBig(big.NewInt(30)),
			Data:      []byte{},
			To:        to,
			AccessList: types.AccessList{
				{Address: common.BigToAddress(big.NewInt(1)), StorageKeys: []common.Hash{common.BigToHash(big.NewInt(1))}},
			},
			BlobFeeCap: uint256.MustFromBig(big.NewInt(10)),
			BlobHashes: sidecar.BlobHashes(),
			Sidecar:    sidecar,
		})

		signer, err := action.NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, 4690)
		require.NoError(err)
		tx, err := types.SignTx(raw, signer, identityset.PrivateKey(1).EcdsaPrivateKey().(*ecdsa.PrivateKey))
		require.NoError(err)
		receipt := &action.Receipt{
			Status:            1,
			BlockHeight:       16,
			ActionHash:        _testTxHash,
			GasConsumed:       21000,
			ContractAddress:   _testContractIoAddr,
			TxIndex:           1,
			EffectiveGasPrice: big.NewInt(25),
			BlobGasUsed:       1024,
			BlobGasPrice:      big.NewInt(10),
		}
		res, err := json.Marshal(&getTransactionResult{
			blockHash: &_testBlkHash,
			to:        &toStr,
			ethTx:     tx,
			receipt:   receipt,
			pubkey:    _testPubKey,
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
			"to":"0x0000000000000000000000000000000000000002",
			"value":"0xa",
			"gasPrice":"0x19",
			"gas":"0x5208",
			"input":"0x",
			"r":"0xdc27308cb102ad623f48c8f71a1eb407a3070d5185a99be4b8af5a88b49dcc5b",
			"s":"0x776660bf014e4c25751c08e047be0d7f7490e564c96a3de88ca65e53147c87bf",
			"v":"0x1",
			"chainId":"0x1252",
			"type":"0x3",
			"yParity": "0x1",
			"accessList": [
				{
					"address": "0x0000000000000000000000000000000000000001",
					"storageKeys": ["0x0000000000000000000000000000000000000000000000000000000000000001"]
				}
			],
			"maxFeePerGas": "0x1e",
			"maxPriorityFeePerGas": "0x14",
			"blobVersionedHashes": ["0x016ba00cade4653a609ffbfb20363fbc5ec4d09f58f3598fc96677a2aaad2837"],
			"maxFeePerBlobGas": "0xa"
		 }
		`, string(res))
	})
}

func TestReceiptObjectMarshal(t *testing.T) {
	require := require.New(t)

	receipt := &action.Receipt{
		Status:            1,
		BlockHeight:       16,
		ActionHash:        _testTxHash,
		GasConsumed:       21000,
		ContractAddress:   _testContractIoAddr,
		TxIndex:           1,
		BlobGasUsed:       2,
		BlobGasPrice:      big.NewInt(10),
		EffectiveGasPrice: big.NewInt(25),
	}

	t.Run("ContractCreation", func(t *testing.T) {
		contractEthaddr, _ := ioAddrToEthAddr(_testContractIoAddr)
		res, err := json.Marshal(&getReceiptResult{
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
			"logsBloom":"0x00008000000100000400000000000040000000000000000000000000000000000000000001000200000400000000000000000000001000000000000000001000000000001000000000200000004000000000000000000101000000000000000008000008000208000000000000400000000000000000000000000000000000000000080010000000000200010000000000000500000000000000000000000000004080000000000000001000000800020000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000400000000000000000000000000080000400010200000",
			"logs":[
			   
			],
			"status":"0x1",
			"blobGasPrice": "0xa",
			"blobGasUsed": "0x2",
			"effectiveGasPrice": "0x19",
			"type": "0x0"
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
		res, err := json.Marshal(&getReceiptResult{
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
			"logsBloom":"0x00008000000100000400000000000040000000000000000000000000000000000000000001000200000400000000000000000000001000000000000000001000000000001000000000200000004000000000000000000101000000000000000008000008000208000000000000400000000000000000000000000000000000000000080010000000000200010000000000000500000000000000000000000000004080000000000000001000000800020000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000400000000000000000000000000080000400010200000",
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
			"status":"0x1",
			"blobGasPrice": "0xa",
			"blobGasUsed": "0x2",
			"effectiveGasPrice": "0x19",
			"type": "0x0"
		 }
		`, string(res))
	})
}

func TestLogsObjectMarshal(t *testing.T) {
	require := require.New(t)

	res, err := json.Marshal(&getLogsResult{
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

func TestStreamResponseMarshal(t *testing.T) {
	require := require.New(t)

	res, err := json.Marshal(&streamResponse{
		id: "0xcd0c3e8af590364c09d0fa6a1210faf5",
		result: &getLogsResult{
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
		}})
	require.NoError(err)
	require.JSONEq(`
	{
		"jsonrpc":"2.0",
		"method":"eth_subscription",
		"params":{
		   "subscription":"0xcd0c3e8af590364c09d0fa6a1210faf5",
		   "result":{
			  "removed":false,
			  "logIndex":"0x3",
			  "transactionIndex":"0x1",
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
		}
	 }
	`, string(res))
}

func TestBlobSiderCar(t *testing.T) {
	require := require.New(t)

	t.Run("Marshal", func(t *testing.T) {
		res, err := json.Marshal(&apitypes.BlobSidecarResult{
			BlobSidecar: &types.BlobTxSidecar{
				Blobs: []kzg4844.Blob{},
				Commitments: []kzg4844.Commitment{
					kzg4844.Commitment{},
				},
				Proofs: []kzg4844.Proof{
					kzg4844.Proof{},
				},
			},
			BlockNumber: 1,
			BlockHash:   common.BigToHash(big.NewInt(2)),
			TxIndex:     2,
			TxHash:      common.BigToHash(big.NewInt(3)),
		})
		require.NoError(err)
		require.JSONEq(`
		{
			"blobSidecar":{
				"Blobs":[],
				"Commitments":[
					"0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
				],
				"Proofs":[
					"0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
				]
			},
			"blockHeight":1,
			"blockHash":"0x0000000000000000000000000000000000000000000000000000000000000002",
			"txIndex":2,
			"txHash":"0x0000000000000000000000000000000000000000000000000000000000000003"
		}
		`, string(res))
	})
}
