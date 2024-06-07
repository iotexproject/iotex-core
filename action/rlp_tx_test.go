package action

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	iotexcrypto "github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	. "github.com/iotexproject/iotex-core/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/pkg/version"
)

func TestGenerateRlp(t *testing.T) {
	require := require.New(t)

	ab := AbstractAction{
		version:  1,
		nonce:    2,
		gasLimit: 1000,
		gasPrice: new(big.Int),
	}
	rlpTsf := &Transfer{
		AbstractAction: ab,
		recipient:      "io1x9qa70ewgs24xwak66lz5dgm9ku7ap80vw3071",
	}
	rlpTsf1 := &Transfer{
		AbstractAction: ab,
		amount:         big.NewInt(100),
		recipient:      "",
	}
	hT1, _ := hex.DecodeString("87e39e819193ae46472eb1320739b34c4c3b38ea321c7cc503432bdcfd0cbf15")
	rlpTsf2 := &Transfer{
		AbstractAction: ab,
		recipient:      "io1x9qa70ewgs24xwak66lz5dgm9ku7ap80vw3070",
	}
	hT2, _ := hex.DecodeString("eaaf38a552809a9bdb1509c8093bd2c74eb07baff862dae692c1d2b865478b14")
	rlpExec := &Execution{
		AbstractAction: ab,
		amount:         big.NewInt(100),
		data:           _signByte,
	}
	hE1, _ := hex.DecodeString("fcdd0c3d07f438d6e67ea852b40e5dc256d75f5e1fa9ac3ca96030efeb634150")
	rlpExec1 := &Execution{
		AbstractAction: ab,
		contract:       "io1x9qa70ewgs24xwak66lz5dgm9ku7ap80vw3070",
		amount:         big.NewInt(100),
		data:           _signByte,
	}
	hE2, _ := hex.DecodeString("fee3db88ee7d7defa9eded672d08fc8641f760f3a11d404a53276ad6f412b8a5")
	rlpTests := []struct {
		act  Action
		sig  []byte
		err  string
		hash hash.Hash256
	}{
		{nil, _validSig, ErrNilAction.Error(), hash.ZeroHash256},
		{rlpTsf, _validSig, address.ErrInvalidAddr.Error(), hash.ZeroHash256},
		{rlpTsf1, _signByte, "address length = 0, expecting 41", hash.ZeroHash256},
		{rlpTsf1, _validSig, "", hash.BytesToHash256(hT1)},
		{rlpTsf2, _validSig, "", hash.BytesToHash256(hT2)},
		{rlpExec, _validSig, "", hash.BytesToHash256(hE1)},
		{rlpExec1, _validSig, "", hash.BytesToHash256(hE2)},
	}

	for _, v := range rlpTests {
		act, ok := v.act.(EthCompatibleAction)
		if !ok {
			require.Contains(v.err, ErrNilAction.Error())
			continue
		}
		tx, err := act.ToEthTx(0)
		if err != nil {
			require.Contains(err.Error(), v.err)
			continue
		}
		signer, err := NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, _evmNetworkID)
		require.NoError(err)
		h, err := rlpSignedHash(tx, signer, v.sig)
		if err != nil {
			require.Contains(err.Error(), v.err)
		}
		require.Equal(v.hash, h)
	}
}

var (
	// deterministic deployment: https://github.com/Arachnid/deterministic-deployment-proxy
	// see example at https://etherscan.io/tx/0xeddf9e61fb9d8f5111840daef55e5fde0041f5702856532cdbb5a02998033d26
	deterministicDeploymentTx = "0xf8a58085174876e800830186a08080b853604580600e600039806000f350fe7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf31ba02222222222222222222222222222222222222222222222222222222222222222a02222222222222222222222222222222222222222222222222222222222222222"

	// example access list tx, created with a random chainID value = 0x7a69 = 31337
	accessListTx = "0x01f8a0827a690184ee6b280082520894a0ee7a142d267c1f36714e4a8f75612f20a797206480f838f794a0ee7a142d267c1f36714e4a8f75612f20a79720e1a0000000000000000000000000000000000000000000000000000000000000000001a0eb211dfd353d76d43ea31a130ff36ac4eb7b379eae4d49fa2376741daf32f9ffa07ab673241d75e103f81ddd4aa34dd6849faf2f0f593eebe61a68fed74490a348"

	rlpTests = []struct {
		actType  string
		raw      string
		nonce    uint64
		limit    uint64
		price    string
		amount   string
		to       string
		chainID  uint32
		encoding iotextypes.Encoding
		dataLen  int
		hash     string
		pubkey   string
		pkhash   string
	}{
		{
			"transfer",
			"f86e8085e8d4a51000825208943141df3f2e4415533bb6d6be2a351b2db9ee84ef88016345785d8a0000808224c6a0204d25fc0d7d8b3fdf162c6ee820f888f5533b1c382d79d5cbc8ec1d9091a9a8a016f1a58d7e0d0fd24be800f64a2d6433c5fcb31e3fc7562b7fbe62bc382a95bb",
			0,
			21000,
			"1000000000000",
			"100000000000000000",
			"0x3141df3f2e4415533bb6d6be2A351B2db9ee84EF",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			0,
			"eead45fe6b510db9ed6dce9187280791c04bbaadd90c54a7f4b1f75ced382ff1",
			"041ba784140be115e8fa8698933e9318558a895c75c7943100f0677e4d84ff2763ff68720a0d22c12d093a2d692d1e8292c3b7672fccf3b3db46a6e0bdad93be17",
			"87eea07540789af85b64947aea21a3f00400b597",
		},
		{
			"execution",
			"f8ab0d85e8d4a5100082520894ac7ac39de679b19aae042c0ce19facb86e0a411780b844a9059cbb0000000000000000000000003141df3f2e4415533bb6d6be2a351b2db9ee84ef000000000000000000000000000000000000000000000000000000003b9aca008224c5a0fac4e25db03c99fec618b74a962d322a334234696eb62c7e5b9889132ff4f4d7a02c88e451572ca36b6f690ce23ff9d6695dd71e888521fa706a8fc8c279099a61",
			13,
			21000,
			"1000000000000",
			"0",
			"0xAC7AC39De679b19AaE042c0cE19fAcB86e0A4117",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			68,
			"7467dd6ccd4f3d7b6dc0002b26a45ad0b75a1793da4e3557cf6ff2582cbe25c9",
			"041ba784140be115e8fa8698933e9318558a895c75c7943100f0677e4d84ff2763ff68720a0d22c12d093a2d692d1e8292c3b7672fccf3b3db46a6e0bdad93be17",
			"87eea07540789af85b64947aea21a3f00400b597",
		},
		{
			"execution",
			"f9024f2e830f42408381b3208080b901fc608060405234801561001057600080fd5b50336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555061019c806100606000396000f3fe608060405234801561001057600080fd5b50600436106100415760003560e01c8063445df0ac146100465780638da5cb5b14610064578063fdacd576146100ae575b600080fd5b61004e6100dc565b6040518082815260200191505060405180910390f35b61006c6100e2565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b6100da600480360360208110156100c457600080fd5b8101908080359060200190929190505050610107565b005b60015481565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141561016457806001819055505b5056fea265627a7a72315820e54fe55a78b9d8bec22b4d3e6b94b7e59799daee3940423eb1aa30fe643eeb9a64736f6c634300051000328224c5a0439310c2d5509fc42486171b910cf8107542c86e23202a3a8ba43129cabcdbfea038966d36b41916f619c64bdc8c3ddcb021b35ea95d44875eb8201e9422fd98f0",
			46,
			8500000,
			"1000000",
			"0",
			EmptyAddress,
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			508,
			"b676128dae841742e3ab6e518acb30badc6b26230fe870821d1de08c85823067",
			"049c6567f527f8fc98c0875d3d80097fcb4d5b7bfe037fc9dd5dbeaf563d58d7ff17a4f2b85df9734ecdb276622738e28f0b7cf224909ab7b128c5ca748729b0d2",
			"1904bfcb93edc9bf961eead2e5c0de81dcc1d37d",
		},
		{
			"stakeCreate",
			"f901670980828ca09404c22afae6a03438b8fed74cb1cf441168df3f1280b90104a3d374c400000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000000000000000000007000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000000000000000000004746573740000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008224c6a02d359322fb1eb0ef44008b587d28160fb237ae716da2735aef9ce2702af52151a03518f334c585c31cec1d9c0ec81fd4c4f70d3ab661502a5aeb1f6eb07bc01854",
			9,
			36000,
			"0",
			"0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			260,
			"f59e5f9ba10ec50fdd1ebb41c75c6d54cfc634428620930b6ba6300847127241",
			"04dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b",
			"1e14d5373e1af9cc77f0032ad2cd0fba8be5ea2e",
		},
		{
			"stakeAddDeposit",
			"f8e60980825aa09404c22afae6a03438b8fed74cb1cf441168df3f1280b88434e8e14500000000000000000000000000000000000000000000000000000000000000070000000000000000000000000000000000000000000000000000000000000064000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000008224c6a0b4af40981b992eab4f15afe1bab1d4c498069cae8fa4ea3e0800d732f282fadea073de47b280905d5ccd4fc329e9c1f10b0bdefe1f4a3f4f8b00918680428ae7dd",
			9,
			23200,
			"0",
			"0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			132,
			"8823b599b46cd907c4691aa71b5668b835be76a8358fa9beb866610e27598592",
			"04dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b",
			"1e14d5373e1af9cc77f0032ad2cd0fba8be5ea2e",
		},
		{
			"changeCandidate",
			"f9012609808273a09404c22afae6a03438b8fed74cb1cf441168df3f1280b8c4fb3d51380000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000700000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000004746573740000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008224c6a06087932dec4df781917c12d31feb80c8e61b2317e3a0820401f3c095746a765da0549df65b7d94fdec196bc70e6dda056fec873664c064036e55fd3bc7e766a595",
			9,
			29600,
			"0",
			"0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			196,
			"23f29aebf4b193b02dd78866d56ea7a7b1cdbf27604d34868bb82993c216b4ec",
			"04dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b",
			"1e14d5373e1af9cc77f0032ad2cd0fba8be5ea2e",
		},
		{
			"unstake",
			"f8c609808252089404c22afae6a03438b8fed74cb1cf441168df3f1280b8642bde151d0000000000000000000000000000000000000000000000000000000000000007000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000008224c6a0bd261cf9c6a2412272c660742a902965611d838176b2ad22b39219dadeedb312a02beef231cc90e4fbd6b928b28aea6ca1ccaf4f793f9d05bc0186e0e09b920a0a",
			9,
			21000,
			"0",
			"0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			100,
			"0c8560b135d325573e6aad484d71e2887835acce7fd4a78eddcb24efe6071516",
			"04dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b",
			"1e14d5373e1af9cc77f0032ad2cd0fba8be5ea2e",
		},
		{
			"withdrawStake",
			"f8c609808252089404c22afae6a03438b8fed74cb1cf441168df3f1280b864d179ffb50000000000000000000000000000000000000000000000000000000000000007000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000008224c5a0526ec8247ce5ae1c8542776cf28a85c7bc2f17e1abc3de5cfbfd20fd85aeeca3a07a6db0a7ae08b6b6aeeae4a1446dcc6f090589e7835d63ebc0a1db783c5e2c89",
			9,
			21000,
			"0",
			"0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			100,
			"49cc2e14f3d1c03d7e36686d962995ea0f30f65f948d5a59181a5504bc58c102",
			"04dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b",
			"1e14d5373e1af9cc77f0032ad2cd0fba8be5ea2e",
		},
		{
			"restake",
			"f9010609808267209404c22afae6a03438b8fed74cb1cf441168df3f1280b8a44c4fee4b000000000000000000000000000000000000000000000000000000000000000700000000000000000000000000000000000000000000000000000000000000070000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000008224c5a0078638f1f00ab532522558e2e4f394f13bfe134a8687ac4639ebead60f63e0dba019dc91f08c1f422de44aa7e6b25ada0632f29bc22242900b3a60f7f371301f02",
			9,
			26400,
			"0",
			"0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			164,
			"c3d30b0fccf9d59ece79419d329e50082a8b6d86dee1b9f424f8852e154713d1",
			"04dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b",
			"1e14d5373e1af9cc77f0032ad2cd0fba8be5ea2e",
		},
		{
			"transferStake",
			"f8e60980825aa09404c22afae6a03438b8fed74cb1cf441168df3f1280b8849cb560bb0000000000000000000000003041a575c7a70021e3082929798c8c3fdaa9d8240000000000000000000000000000000000000000000000000000000000000007000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000008224c5a07fd84321b04de059cbfdf12dcd383d3e554c569705f32035bc435b8d996c8bdba00df629ef4d9135d73bb60d49815761bc0a6ee7eb66770106dc9e809cd105d570",
			9,
			23200,
			"0",
			"0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			132,
			"a60b2546839e889a0ef89be4f224fb70dab3e4ddb6f65391ff708b01116593c1",
			"04dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b",
			"1e14d5373e1af9cc77f0032ad2cd0fba8be5ea2e",
		},
		{
			"candidateRegister",
			"f901c709808252089404c22afae6a03438b8fed74cb1cf441168df3f1280b90164bee5f7b700000000000000000000000000000000000000000000000000000000000001000000000000000000000000003041a575c7a70021e3082929798c8c3fdaa9d8240000000000000000000000003041a575c7a70021e3082929798c8c3fdaa9d8240000000000000000000000003041a575c7a70021e3082929798c8c3fdaa9d82400000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000000000000000000007000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001400000000000000000000000000000000000000000000000000000000000000004746573740000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008224c5a07b4fd9921e47c13bb52fc5e0df47f4dc0bffcf6e7877e13c7e559b4a7d3b9825a0073a3a029822aa43506834bef8b1ed30d6bb436d758d7cf583deb48a4efefd6a",
			9,
			21000,
			"0",
			"0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			356,
			"9aa470cbdc3b3fd8f51aae4770d6a58cf4016be18201f0efbae6d83d0b2aa096",
			"04dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b",
			"1e14d5373e1af9cc77f0032ad2cd0fba8be5ea2e",
		},
		{
			"candidateUpdate",
			"f9010609808252089404c22afae6a03438b8fed74cb1cf441168df3f1280b8a4435f9f2200000000000000000000000000000000000000000000000000000000000000600000000000000000000000003041a575c7a70021e3082929798c8c3fdaa9d8240000000000000000000000003041a575c7a70021e3082929798c8c3fdaa9d824000000000000000000000000000000000000000000000000000000000000000474657374000000000000000000000000000000000000000000000000000000008224c6a04df6545da871debef84476198e76167d4dfe6fb83098a5a49dbab734352f20f1a0220686494f8b09751bbeb5c63729e7ecc5c504f952c67e926461db98588854b6",
			9,
			21000,
			"0",
			"0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			164,
			"7a8d96d35c939bf1634d587b1f471c0f3f96ba750d64d43289c9eef267718ef0",
			"04dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b",
			"1e14d5373e1af9cc77f0032ad2cd0fba8be5ea2e",
		},
		//{
		//	"rewardingClaim",
		//	"f8c6806482520894a576c141e5659137ddda4223d209d4744b2106be80b8642df163ef0000000000000000000000000000000000000000000000000000000000000065000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000008224c6a03d62943431b8ca0e8ea0a9c79dc8f64df14c965ca5486da124804013778ed566a065956b185116b65cec0ec9bcf9539e924784a4b448190b0aa9d017f1719be921",
		//	0,
		//	21000,
		//	"100",
		//	"0",
		//	"0xA576C141e5659137ddDa4223d209d4744b2106BE",
		//	_evmNetworkID,
		//	iotextypes.Encoding_ETHEREUM_EIP155,
		//	100,
		//	"4d26d0736ebd9e69bd5994f3730b05a2d48c810b3bb54818be65d02004cf4ff4",
		//	"04830579b50e01602c2015c24e72fbc48bca1cca1e601b119ca73abe2e0b5bd61fcb7874567e091030d6b644f927445d80e00b3f9ca0c566c21c30615e94c343da",
		//	"8d38efe45794d7fceea10b2262c23c12245959db",
		//},
		{
			"rewardingDeposit",
			"f8c6016482520894a576c141e5659137ddda4223d209d4744b2106be80b86427852a6b0000000000000000000000000000000000000000000000000000000000000065000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000008224c6a013b7679dbabcb0f97b93942436f5072cca3c7fe43451a8fedcdf3c84c1344e1da02af4cc67594c0200b59f4e30ba149af15e546acbfc69fa31f14e8788ab063d85",
			1,
			21000,
			"100",
			"0",
			"0xA576C141e5659137ddDa4223d209d4744b2106BE",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			100,
			"842fea9a292c0bc7a1e05a14a8255ed8dc945fdae7c01a6b70f5213ebe35583b",
			"04830579b50e01602c2015c24e72fbc48bca1cca1e601b119ca73abe2e0b5bd61fcb7874567e091030d6b644f927445d80e00b3f9ca0c566c21c30615e94c343da",
			"8d38efe45794d7fceea10b2262c23c12245959db",
		},
		{
			"candidateActivate",
			"f88a7885e8d4a510008252089404c22afae6a03438b8fed74cb1cf441168df3f1280a4ef68b1a400000000000000000000000000000000000000000000000000000000000000018224c6a05d21ebb92203797ce49a95febe4430cfdfd32deb9284c80fee5d600124a77791a05ac4871b1b1200433f197aee068007b52f2b096e27fe473874ceab89661fad2a",
			120,
			21000,
			"1000000000000",
			"0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			36,
			"db61959a804781ea9da78d6d4ce6054e1ea940ae4a2cac3853f5262728a9369e",
			"049ea260dc05a824a8a7e92d3b87e47ce06384e1eee15d446494a20d299c1bd11900edc2df5697aa00b2256c286193239f83740d079b032b9fbe085b791fa10950",
			"065e1164818487818e6ba714e8d80b91718ad758",
		},
		{
			"candidateEndorsement",
			"f8ab7885e8d4a510008252089404c22afae6a03438b8fed74cb1cf441168df3f1280b8448a8a5d51000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000018224c5a0cb0c5d443925b98776645793fe8bfdeb34ca1d364d1c389521f5a4c715a42a91a07c1bb620307a78fb463bb8204a43d4bf32acd6efed0cb6b7c00eb7ae13da1ad6",
			120,
			21000,
			"1000000000000",
			"0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			68,
			"008d08ae3327ae395fdc0b16497e181c4eb6633cf796c174e8819cbb2a29194c",
			"049ea260dc05a824a8a7e92d3b87e47ce06384e1eee15d446494a20d299c1bd11900edc2df5697aa00b2256c286193239f83740d079b032b9fbe085b791fa10950",
			"065e1164818487818e6ba714e8d80b91718ad758",
		},
		{
			"candidateTransferOwnership",
			"f8c9018203e8830f42409404c22afae6a03438b8fed74cb1cf441168df3f1280b8643bf030350000000000000000000000007f54538b6260d754701e2f4a9a9bc0cb654293d9000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000008224c6a04ed770debd083f4a6e897fc262a179a17af2db7fb0524124f02da0f4469cd40ea02ef832202857f657b417e2c7aa2d8fb12093849ac4e2b7f130e4c6fec6b1b199",
			1,
			1000000,
			"1000",
			"0",
			"0x04C22AfaE6a03438b8FED74cb1Cf441168DF3F12",
			_evmNetworkID,
			iotextypes.Encoding_ETHEREUM_EIP155,
			100,
			"a8eec664ab3df5dd23980c83c6e6c2dd2fe32c02383de89174bf2b7740d22e39",
			"04dc4c548c3a478278a6a09ffa8b5c4b384368e49654b35a6961ee8288fc889cdc39e9f8194e41abdbfac248ef9dc3f37b131a36ee2c052d974c21c1d2cd56730b",
			"1e14d5373e1af9cc77f0032ad2cd0fba8be5ea2e",
		},
		{
			"unprotected",
			deterministicDeploymentTx,
			0,
			100000,
			"100000000000",
			"0",
			EmptyAddress,
			0,
			iotextypes.Encoding_ETHEREUM_UNPROTECTED,
			83,
			"eddf9e61fb9d8f5111840daef55e5fde0041f5702856532cdbb5a02998033d26",
			"040a98b1acb38ed9cd8d0e8f1f03b1588bae140586f8a8049197b65013a3c17690151ae422e3fdfb26be2e6a4465b1f9cf5c26a5635109929a0d0a11734124d50a",
			"3fab184622dc19b6109349b94811493bf2a45362",
		},
	}
)

func TestNewEthSignerError(t *testing.T) {
	require := require.New(t)
	singer, err := NewEthSigner(iotextypes.Encoding_ETHEREUM_ACCESSLIST, 1)
	require.ErrorIs(err, ErrInvalidAct)
	require.Nil(singer)

	tx := types.NewTx(&types.DynamicFeeTx{
		To:        nil,
		Nonce:     4,
		Value:     big.NewInt(4),
		Gas:       4,
		GasTipCap: big.NewInt(44),
		GasFeeCap: big.NewInt(1045),
	})
	_, _, _, err = ExtractTypeSigPubkey(tx)
	require.ErrorIs(err, ErrNotSupported)
}

func TestEthTxDecodeVerify(t *testing.T) {
	require := require.New(t)

	for _, v := range rlpTests {
		// decode received RLP tx
		tx, err := DecodeEtherTx(v.raw)
		require.NoError(err)
		encoding, sig, pubkey, err := ExtractTypeSigPubkey(tx)
		require.Equal(v.encoding, encoding)
		V, _, _ := tx.RawSignatureValues()
		recID := V.Uint64()
		if encoding == iotextypes.Encoding_ETHEREUM_EIP155 {
			require.EqualValues(types.LegacyTxType, tx.Type())
			require.True(tx.Protected())
			require.Less(uint64(28), recID)
		} else if encoding == iotextypes.Encoding_ETHEREUM_UNPROTECTED {
			require.EqualValues(types.LegacyTxType, tx.Type())
			require.False(tx.Protected())
			require.Zero(tx.ChainId().Uint64())
			// unprotected tx has V = 27, 28
			require.True(27 == recID || 28 == recID)
			require.True(27 == sig[64] || 28 == sig[64])
		}
		require.EqualValues(v.chainID, tx.ChainId().Uint64())
		require.Equal(v.pubkey, pubkey.HexString())
		require.Equal(v.pkhash, hex.EncodeToString(pubkey.Hash()))

		// convert to our Execution
		pb := &iotextypes.Action{
			Encoding: encoding,
		}
		pb.Core = convertToNativeProto(tx, v.actType)
		pb.SenderPubKey = pubkey.Bytes()
		pb.Signature = sig

		// send on wire
		bs, err := proto.Marshal(pb)
		require.NoError(err)

		// receive from API
		proto.Unmarshal(bs, pb)
		selp, err := (&Deserializer{}).SetEvmNetworkID(v.chainID).ActionToSealedEnvelope(pb)
		require.NoError(err)
		act, ok := selp.Action().(EthCompatibleAction)
		require.True(ok)
		rlpTx, err := act.ToEthTx(uint32(tx.ChainId().Uint64()))
		require.NoError(err)

		// verify against original tx
		require.Equal(v.nonce, rlpTx.Nonce())
		require.Equal(v.price, rlpTx.GasPrice().String())
		require.Equal(v.limit, rlpTx.Gas())
		if v.to == "" {
			require.Nil(rlpTx.To())
		} else {
			require.Equal(v.to, rlpTx.To().Hex())
		}
		require.Equal(v.amount, rlpTx.Value().String())
		require.Equal(v.dataLen, len(rlpTx.Data()))
		h, err := selp.Hash()
		require.NoError(err)
		require.Equal(v.hash, hex.EncodeToString(h[:]))
		require.Equal(pubkey, selp.SrcPubkey())
		require.True(bytes.Equal(sig, selp.signature))
		raw, err := selp.envelopeHash()
		require.NoError(err)
		signer, err := NewEthSigner(encoding, uint32(tx.ChainId().Uint64()))
		require.NoError(err)
		rawHash := signer.Hash(tx)
		require.True(bytes.Equal(rawHash[:], raw[:]))
		require.NotEqual(raw, h)
		require.NoError(selp.VerifySignature())
	}
}

func TestEthTxDecodeVerifyV2(t *testing.T) {
	var (
		r        = require.New(t)
		sk, _    = iotexcrypto.HexStringToPrivateKey("708361c6460c93f027df0823eb440f3270ee2937944f2de933456a3900d6dd1a")
		nonce    = uint64(100)
		amount   = big.NewInt(1)
		to       = "io1c04r4jc8q6u57phxpla639u5h5v2t3s062hyxa"
		addrto   = common.BytesToAddress(MustNoErrorV(address.FromString(to)).Bytes())
		data     = []byte("any")
		gasLimit = uint64(21000)
		gasPrice = big.NewInt(101)
		chainID  = uint32(4689)
	)
	r.NotNil(sk)

	elpbuilder := (&EnvelopeBuilder{}).
		SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetChainID(chainID)

	tests := []struct {
		name     string                                     // case name
		encoding iotextypes.Encoding                        // assign and check signer's type
		txto     string                                     // assertion of tx.To()
		txamount *big.Int                                   // assertion of tx.Value()
		txdata   []byte                                     // assertion of tx.Data()
		action   EthCompatibleAction                        // eth compatible action
		builder  func(*types.Transaction) (Envelope, error) // envelope builder
	}{
		{
			name:     "Transfer",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     to,
			txamount: amount,
			txdata:   data,
			action:   MustNoErrorV(NewTransfer(nonce, amount, to, data, gasLimit, gasPrice)),
			builder:  elpbuilder.BuildTransfer,
		},
		{
			name:     "Execution",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     to,
			txamount: amount,
			txdata:   data,
			action:   MustNoErrorV(NewExecution(to, nonce, amount, gasLimit, gasPrice, data)),
			builder:  elpbuilder.BuildExecution,
		},
		{
			name:     "ExecutionEmptyTo",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     "",
			txamount: amount,
			txdata:   data,
			action:   MustNoErrorV(NewExecution("", nonce, amount, gasLimit, gasPrice, data)),
			builder:  elpbuilder.BuildExecution,
		},
		{
			name:     "CreateStake",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_createStakeMethod.ID,
				MustNoErrorV(_createStakeMethod.Inputs.Pack("name", amount, uint32(86400), true, data))...,
			),
			action:  MustNoErrorV(NewCreateStake(nonce, "name", amount.String(), 86400, true, data, gasLimit, gasPrice)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "DepositToStake",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_depositToStakeMethod.ID,
				MustNoErrorV(_depositToStakeMethod.Inputs.Pack(uint64(10), amount, data))...,
			),
			action:  MustNoErrorV(NewDepositToStake(nonce, 10, amount.String(), data, gasLimit, gasPrice)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "ChangeStakeCandidate",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_changeCandidateMethod.ID,
				MustNoErrorV(_changeCandidateMethod.Inputs.Pack("name", uint64(11), data))...,
			),
			action:  MustNoErrorV(NewChangeCandidate(nonce, "name", 11, data, gasLimit, gasPrice)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "UnStake",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_unstakeMethod.ID,
				MustNoErrorV(_unstakeMethod.Inputs.Pack(uint64(12), data))...,
			),
			action:  MustNoErrorV(NewUnstake(nonce, 12, data, gasLimit, gasPrice)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "StakeWithdraw",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_withdrawStakeMethod.ID,
				MustNoErrorV(_withdrawStakeMethod.Inputs.Pack(uint64(13), data))...,
			),
			action:  MustNoErrorV(NewWithdrawStake(nonce, 13, data, gasLimit, gasPrice)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "ReStake",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_restakeMethod.ID,
				MustNoErrorV(_restakeMethod.Inputs.Pack(uint64(14), uint32(7200), false, data))...,
			),
			action:  MustNoErrorV(NewRestake(nonce, 14, 7200, false, data, gasLimit, gasPrice)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "TransferStakeOwnership",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_transferStakeMethod.ID,
				MustNoErrorV(_transferStakeMethod.Inputs.Pack(
					common.BytesToAddress(MustNoErrorV(address.FromString(to)).Bytes()),
					uint64(15), data),
				)...,
			),
			action:  MustNoErrorV(NewTransferStake(nonce, to, 15, data, gasLimit, gasPrice)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "RegisterStakeCandidate",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_candidateRegisterMethod.ID,
				MustNoErrorV(_candidateRegisterMethod.Inputs.Pack(
					"name", addrto, addrto, addrto, amount, uint32(6400), false, data,
				))...,
			),
			action: MustNoErrorV(NewCandidateRegister(
				nonce, "name", to, to, to, amount.String(),
				6400, false, data, gasLimit, gasPrice,
			)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "UpdateStakeCandidate",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_candidateUpdateMethod.ID,
				MustNoErrorV(_candidateUpdateMethod.Inputs.Pack("name", addrto, addrto))...,
			),
			action:  MustNoErrorV(NewCandidateUpdate(nonce, "name", to, to, gasLimit, gasPrice)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "StakeCandidateActivate",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				candidateActivateMethod.ID,
				MustNoErrorV(candidateActivateMethod.Inputs.Pack(uint64(16)))...,
			),
			action: &CandidateActivate{
				AbstractAction: AbstractAction{
					version:  version.ProtocolVersion,
					chainID:  chainID,
					nonce:    nonce,
					gasLimit: gasLimit,
					gasPrice: gasPrice,
				},
				bucketID: 16,
			},
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "StakeCandidateEndorsement",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				candidateEndorsementMethod.ID,
				MustNoErrorV(candidateEndorsementMethod.Inputs.Pack(uint64(17), false))...,
			),
			action: &CandidateEndorsement{
				AbstractAction: AbstractAction{
					version:  version.ProtocolVersion,
					chainID:  chainID,
					nonce:    nonce,
					gasLimit: gasLimit,
					gasPrice: gasPrice,
				},
				bucketIndex: 17,
			},
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "StakeCandidateTransferOwnership",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.StakingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_candidateTransferOwnershipMethod.ID,
				MustNoErrorV(_candidateTransferOwnershipMethod.Inputs.Pack(addrto, data))...,
			),
			action:  MustNoErrorV(NewCandidateTransferOwnership(nonce, gasLimit, gasPrice, to, data)),
			builder: elpbuilder.BuildStakingAction,
		},
		{
			name:     "ClaimFromRewardingFund",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.RewardingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_claimRewardingMethod.ID,
				MustNoErrorV(_claimRewardingMethod.Inputs.Pack(amount, data, to))...,
			),
			action: &ClaimFromRewardingFund{
				AbstractAction: AbstractAction{
					version:  version.ProtocolVersion,
					chainID:  chainID,
					nonce:    nonce,
					gasLimit: gasLimit,
					gasPrice: gasPrice,
				},
				amount:  amount,
				address: to,
				data:    data,
			},
			builder: elpbuilder.BuildRewardingAction,
		},
		{
			name:     "DepositToRewardingFund",
			encoding: iotextypes.Encoding_ETHEREUM_EIP155,
			txto:     MustNoErrorV(address.FromBytes(address.RewardingProtocolAddrHash[:])).String(),
			txamount: big.NewInt(0),
			txdata: append(
				_depositRewardMethod.ID,
				MustNoErrorV(_depositRewardMethod.Inputs.Pack(amount, data))...,
			),
			action: &DepositToRewardingFund{
				AbstractAction: AbstractAction{
					version:  version.ProtocolVersion,
					chainID:  chainID,
					nonce:    nonce,
					gasLimit: gasLimit,
					gasPrice: gasPrice,
				},
				amount: amount,
				data:   data,
			},
			builder: elpbuilder.BuildRewardingAction,
		},
		{
			name:     "UnprotectedExecution",
			encoding: iotextypes.Encoding_ETHEREUM_UNPROTECTED,
			txto:     to,
			txamount: amount,
			txdata:   data,
			action:   MustNoErrorV(NewExecution(to, nonce, amount, gasLimit, gasPrice, data)),
			builder:  elpbuilder.BuildExecution,
		},
	}

	verifytx := func(tx *types.Transaction, enc iotextypes.Encoding, amount *big.Int, data []byte, to string, sigv *big.Int) {
		r.Equal(tx.Nonce(), nonce)
		r.Equal(tx.Gas(), gasLimit)
		r.Equal(tx.GasPrice(), gasPrice)
		r.Equal(tx.Type(), uint8(types.LegacyTxType))
		r.Equal(tx.Value(), amount)
		r.Equal(tx.Data(), data)
		if to == "" {
			r.Nil(tx.To())
		} else {
			r.Equal(to, strings.ToLower(tx.To().String()))
		}
		if enc == iotextypes.Encoding_ETHEREUM_UNPROTECTED {
			r.False(tx.Protected())
			r.Zero(tx.ChainId().Uint64())
			r.True(sigv.Uint64() == 27 || sigv.Uint64() == 28)
		} else if enc == iotextypes.Encoding_ETHEREUM_EIP155 {
			r.True(tx.Protected())
			r.Equal(tx.ChainId().Uint64(), uint64(chainID))
			r.True(sigv.Uint64() > 28)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toaddr := ""
			if tt.txto != "" {
				toaddr = "0x" + hex.EncodeToString(MustNoErrorV(address.FromString(tt.txto)).Bytes())
			}
			// build eth tx from test case
			var (
				tx        = MustNoErrorV(tt.action.ToEthTx(chainID))
				signer    = MustNoErrorV(NewEthSigner(tt.encoding, chainID))
				signature = MustNoErrorV(sk.Sign(tx.Hash().Bytes()))
				builttx   = MustNoErrorV(RawTxToSignedTx(tx, signer, signature))
				raw       = MustNoErrorV(builttx.MarshalBinary())
			)

			sv, _, _ := builttx.RawSignatureValues()
			verifytx(builttx, tt.encoding, tt.txamount, tt.txdata, toaddr, sv)

			// decode eth tx from builttx hex raw
			decodedtx, err := DecodeEtherTx(hex.EncodeToString(raw))
			r.NoError(err)
			enc, sig, pk, err := ExtractTypeSigPubkey(decodedtx)
			r.NoError(err)
			r.Equal(enc, tt.encoding)
			r.Equal(sig[:64], signature[:64])
			sv, _, _ = decodedtx.RawSignatureValues()
			verifytx(decodedtx, tt.encoding, tt.txamount, tt.txdata, toaddr, sv)

			// build elp from eth tx by native converter
			elp, err := tt.builder(decodedtx)
			r.NoError(err)
			sealed, err := (&Deserializer{}).
				SetEvmNetworkID(chainID).
				ActionToSealedEnvelope(&iotextypes.Action{
					Core:         elp.Proto(),
					SenderPubKey: pk.Bytes(),
					Signature:    sig,
					Encoding:     tt.encoding,
				})
			r.NoError(err)
			action, ok := sealed.Action().(EthCompatibleAction)
			r.True(ok)
			convertedrawtx, err := action.ToEthTx(chainID)
			r.NoError(err)
			r.NoError(sealed.VerifySignature())
			convertedsignedtx, err := RawTxToSignedTx(convertedrawtx, signer, signature)
			r.NoError(err)
			r.Equal(convertedsignedtx.Hash(), decodedtx.Hash())
			sv, _, _ = convertedsignedtx.RawSignatureValues()
			verifytx(convertedsignedtx, tt.encoding, tt.txamount, tt.txdata, toaddr, sv)
		})
	}
}

func convertToNativeProto(tx *types.Transaction, actType string) *iotextypes.ActionCore {
	elpBuilder := &EnvelopeBuilder{}
	elpBuilder.SetGasLimit(tx.Gas()).SetGasPrice(tx.GasPrice()).SetNonce(tx.Nonce())
	switch actType {
	case "transfer":
		elp, _ := elpBuilder.BuildTransfer(tx)
		return elp.Proto()
	case "execution", "unprotected", "accesslist":
		elp, _ := elpBuilder.BuildExecution(tx)
		return elp.Proto()
	case "stakeCreate", "stakeAddDeposit", "changeCandidate", "unstake", "withdrawStake", "restake",
		"transferStake", "candidateRegister", "candidateUpdate", "candidateActivate", "candidateEndorsement", "candidateTransferOwnership":
		elp, _ := elpBuilder.BuildStakingAction(tx)
		return elp.Proto()
	case "rewardingClaim", "rewardingDeposit":
		elp, _ := elpBuilder.BuildRewardingAction(tx)
		return elp.Proto()
	default:
		panic("unsupported")
	}
}

func TestIssue3944(t *testing.T) {
	r := require.New(t)
	// the sample tx below is the web3 format tx on the testnet:
	// https://testnet.iotexscan.io/tx/fcaf377ff3cc785d60c58de7e121d6a2e79e1c58c189ea8641f3ea61f7605285
	// or you can get it using ethclient code:
	// {
	//		cli, _ := ethclient.Dial("https://babel-api.testnet.iotex.io")
	// 		tx, _, err := cli.TransactionByHash(context.Background(), common.HexToHash("0xfcaf377ff3cc785d60c58de7e121d6a2e79e1c58c189ea8641f3ea61f7605285"))
	// }
	var (
		hash    = "0xfcaf377ff3cc785d60c58de7e121d6a2e79e1c58c189ea8641f3ea61f7605285"
		data, _ = hex.DecodeString("f3fef3a3000000000000000000000000fdff3eafde9a0cc42d18aab2a7454b1105f19edf00000000000000000000000000000000000000000000002086ac351052600000")
		to      = common.HexToAddress("0xd313b3131e238C635f2fE4a84EaDaD71b3ed25fa")
		sig, _  = hex.DecodeString("adff1da88c93f4e80c27bab0d613147fb7aeeed6e976231695de52cd9ac5aa8a3094e02759b838514f8376e05ceb266badc791ac2e7045ee7c15e58fc626980b1b")
	)
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    83,
		To:       &to,
		Value:    new(big.Int),
		Gas:      39295,
		GasPrice: big.NewInt(1000000000000),
		Data:     data,
		V:        big.NewInt(int64(sig[64])),
		R:        new(big.Int).SetBytes(sig[:32]),
		S:        new(big.Int).SetBytes(sig[32:64]),
	})

	r.EqualValues(types.LegacyTxType, tx.Type())
	r.False(tx.Protected())
	v, q, s := tx.RawSignatureValues()
	r.Equal(sig[:32], q.Bytes())
	r.Equal(sig[32:64], s.Bytes())
	r.Equal("1b", v.Text(16))
	r.NotEqual(hash, tx.Hash().Hex()) // hash does not match with wrong V value in signature

	signer, err := NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, 4690)
	r.NoError(err)
	tx1, err := RawTxToSignedTx(tx, signer, sig)
	r.NoError(err)
	r.True(tx1.Protected())
	r.EqualValues(4690, tx1.ChainId().Uint64())
	v, q, s = tx1.RawSignatureValues()
	r.Equal(sig[:32], q.Bytes())
	r.Equal(sig[32:64], s.Bytes())
	r.Equal("9415", v.String()) // this is the correct V value corresponding to chainID = 4690
	r.Equal(hash, tx1.Hash().Hex())
}
