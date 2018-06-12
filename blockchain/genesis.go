// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"encoding/hex"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/blockchain/trx"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

// Genesis defines the Genesis default settings
type Genesis struct {
	InitDelegatesPubKey []string
	ChainID             uint32
	TotalSupply         uint64
	BlockReward         uint64
	Timestamp           uint64
	ParentHash          common.Hash32B
	GenesisCoinbaseData string
}

// initDelegatePK hardcodes initial 21 candidates that enter candidate pool
var initDelegatePK = []string{
	"86fa928962c85b47671b11a10e1771cff778392be52b5a272cbbf552fc44a7c66a41600562e6b3988db574dc79a9ebab097d516c6b24315dbcb98f4b267c60a6c8d162e2f54f6d00",
	"f46ff44f7a1ac9ed5f87597abcae0684c618ba7bcef9f7fd3100cb60cef95fa7db0fed0432b0d45a5d0b5dbd2e3129f44f39f1a2503135a1f9e58718d5c4d45aee61adae75e0c206",
	"74da096d6028e21fb12b826ff949a5f87d6f5059f14b35bc21dbb5ea889594fb2a5c16052e6285019ae5a9ba580e2283aa1dc9d0a97aadce34d1db056e9587e49076a5fc7f7d0b05",
	"e15fb9dd58fa2bbd10f7b683e442fb08cd213a240d0df6e82bd384bf9969d93616afd402c129a043df2d9dfdd15f4bcaee9f479aae9818f4a7ca32ac53ee8778f0798277f81e7706",
	"ee34ae11f1d40b81148cbb3689150f1443ea837fa8a921eef499691d6b945eeab072120310e53ba4b03fd1ce014dd1acf75b1b72baa5d2d5c9b29a3aaa8452d2ab28fd6ba2831f04",
	"1ba986e56130c6c4d57be3bb58551d627fc229c7e0ac070c054585f48fe905b0d9981906bbac80380e21c28d66a191abd41c017166b50190c1affb124e057f41fc2431c35d093900",
	"51fe83bd671bd55508bec913ab1fc035b64d14b772d0dda48a8ebe177ed3bae31191240741df51016896073b7d1a64e92190ea82301b5d25daf272a261f6482562a830bda5f4ea06",
	"f3b83b344c22e389b67a2ac6fefbec59872777dcb342608e55de651259f04f0cdfc35c008e38179f9cea7d790b17e74fe8897a92d15972221078e6e8333297bc5b6635fb354c2c07",
	"a034dd57f39a3ffb3ee37a3b3306144829664a8a3619b2f30425a952374a89d91077c200b696eadf66e4bfef0f742f13d42437ce23f3dafe8f22162b8f258c92143dc9db64d1d101",
	"62f871513abf7a048d33312b5092e5e7c3f10849b842e3bf3c5a2e3186165356f7300401561b31e3d30a9e86836319b2d775a61de1ee11565e68ea330b5cc42548b78f35b3debe00",
	"c8f6f08cc5e90f19638b406b4fd1df635fe5146a4758bb15759fb8c93e522db1794c2f06e11c48258584a71abfb426c3db9544f6b8a2ace8f93f4a9cc217f637b0d86ad5e37c8805",
	"c96b38921d96e7d2ffd6466ddcf4daf4997d8ff18e2e8989f5953b9c39ecdc9b8fca7500c8574e563ca5017e59e706c5f18cf910966176032a3a227d5e4f61cf89d7224b4ef45a03",
	"ed7a30c4d9cc96579fed12f4bb76b0f61687258703cea689a2204eae1290b0aa688e78032ae757283d44b766419b203da508e59f7a072866d23ba64886a900a7af7c98926f203804",
	"9093d8a565bf50e5c15ca864fa0c8af344dceb1a18655e4561066a7f72c1ff50684b8a06c55e8c184bc7f18d8e59f0dc4a13c274382e75c51422e717e043a2b08eb892457b4a6a06",
	"91d68798f80655d29dde24a0c8e8c3820f3187ebc3fcae59edab09bff653445285e6a30173b5d0b1720383bdf067926ca6750b1685efbdeb25e6d994fb59f8be7f0b3cba54320403",
	"7d8e2a93f5f1c2b77ddadb27d611693ccaf386dc9cbb722f6f3dfe0af3b46df2cbded5037992e4d3ca826a2f5c1edcfc217dbbb759eefc3047af424d6b87ef8d030d415f49d56f04",
	"021239ea718bb781cc99f1e5124c962161584153b4f079039e9aa821a95e508b285ef20688abf763346a38b798da5fdcfcb80352df9e61b0ce3a6692e3dd57f1ffa066c0d99e6106",
	"38b5ef1ad196d28390484aefab3a3104b3e8db2d39c4fdb79b49cfe3ec07acb320132306658f274b8414cad3fcf15ef806e168cdd20e57e2b5a70aaf1a6b91cfd55e29284b64fe02",
	"e29a91600e0ab60b489c4d98be554f160f3cc233033005ea99ddab52e6e82084967977009bf6301ba6df9909fbca748a66779ea27c996c90721b56db7ac74d46ba782701b0c9c707",
	"98d2daa55e41d79073201f3cdac74ab303b3de07c1d3a896140f227c34f7c99a842503060167c1daebdfae687a4fe1f1ff3e718f649206d8e1aaedbfc2cc69ff164ce0a5c1f02205",
	"f07250311320c730e272a32a748a44d6c31811b9aff84dbaa2f8485984ac3adf57147d0759a1f3051f3e17b9dc71dbe6161f546b5c4c4fae48507974d51da1c22d519fb025fe2203",
}

var initVotesSignature = []string{
	"2501fb97870867c963bd6b310abcf0926616a3500064ead2547635a4cfbaf0479ba019015fa1e7e1c52c2c48a033045f214c0bc26dc13d480bad21d1ef08f87a3202004367ba2701",
	"57bc589c77895fb9fe2db6b8ca5157720e7c1ff147a8987c744d86766e9701b176044201201d7c7226784b9ab98a3691475f1b3572ee60646ef6ee0603d36be7daf1e0a41f68ae01",
	"8251b62a1b13c6093dc844d294bfeab9de6cd985e7f97f3434009948611d63fe9af03601bd2154a83df9b1c8f9b7763676f56f20d875a26971e4cd8262c4dab22b2f8091d7df1001",
	"2747744095bd89842bafcdbedd026ffadd121f909b65b874b45f9ab8566440382f6e0201c11599fd06c51b0caf74c07d76734623c75f42575b72f918945b7baa7c6dacae8de1fe01",
	"6ae1f1948ef33b1c1cb72bca281dfd4d306d6c1eb8ba801eb799f54711e0d72f7d4dc4002fc690e7ef0a6c616b5ff409645fd44012d8e47a1e807e1cabc49f0d48087c9fecb64800",
	"26ec44cd5be30760421e4c44e8e56ee4716f1a0072e1922f7a0052954bd9fd1dc3ae5b01d9dafa71d1bc86547a78e5fc1a6238daa3b7877ad3063deae5a5e61bb1646366f489bb01",
	"b8ec084180d1fb016374417ed319bfb400b71e4512940f533912d4e62d61562cafbad7001b84b0d4e99330c585537ddb245e8df31d8e3efaba69060b4bff084695a0a6a833e06400",
	"d1b33069b9abe61aafe370621fd3d8b1c2aa49def7da6d3daa24d52fab1c967b2d6ff000c5fd7ac75d9027be1a777f869ea4db7aef74193660481049b79adb8398889bf3fa245500",
	"8089f28b29d75d70ef0a2eb09e4bd5a255a20309bbcbf6ed2543a90b3bb7218c9c3ed201145cf20883afe816bb2df2372a2be76e84768f292886bcbf5bfd43eb8eba793262f8b301",
	"aa645977cbc44adaf13cfef306b2f81ec9e9481423318ab37074c078afd07325b1dd2000b5f704ec682e9412486d455217e17210dabddf26f553cdc20c77f31e2d290e63f11dd501",
	"64cc1715dc5731ee2d0056d8e20e0d5e8db49015f397c62faa8f65118509b026ec3d930053241c8705b64b91c255dc9c832ee2afc3b52a3231e406a7ca2be27327d9d7ba7dcaea00",
	"3276a6c785bb233667a39b71d3e03ef6256705da9b2fed4b6e23fdccbf5c6aa883d742008ead51f85347ae1ba16bf0afa91704f301a6133bda836d6d4cd55712bd14d3a310ab9300",
	"135123610d6302b8d6f48d3c186754dcd6e4d95b25780e354f5fd4d43d712cb70a74e6005cf51c23ff176ce7c52b5d3bba52e08c1340f4c92228dbe982120a375eb58a8f2be28a01",
	"46f8530862d8cb227ca042ca5ad6ef7157ed54872219ae4e7a67991b576b448a4fd3ae0065aa3ec0eb2291bf274a6160e7ce24093e364d0cf2f342e6bb67c5bf01245a1a49b78701",
	"524d1befff199164faed888ced0877f2f15be7afdca6309f3c34f8e98f6333a60f40cf012a71088b225ddd0e173e9819ffa2cfeb6348dd1a4f0b9b6011f39a6e811a0187c8e5d501",
	"6da59c15313412192e53b86babdc5952bdfa74a22feb5ec69af5ec8cf04e05d2580c1a001270ff8c7f682dd1810acdbf9f3f6b6be402f2b57fbca4e0a5f6d240e2b197ebd0ae1500",
	"d7c55ac60b5f01fea46082fe1457ccf5027da53e888a9df3a337a0e22ad67e6679272901109326f1925b5c2bfa32e4ab85f6b41769269afdb1b2c8169c6e4717c96d240c34874601",
	"42e131a8d31492f794509b1b109d73b4f6b3c186be2dece0f8062d49a8778223c68302004d7fd95045c803d64340d2a5b3d1762483102acff4dbe03637a8f7eff916b796c8553701",
	"020f01cf384319b5bd847b135a3f91600449ded431ae1d3d11ab6f8fa963b7e54e10dc00249a3220911119e6c79fdb5180620c8cf23b604d5185e3b24da1754527a261e3a798a300",
	"ff0d558ef940061fd39e8fdd6720d221a744f8c5bf249ac23dbece80ae23939e4aaa10004f12387f23b4317efaac816581acab35d6c558b5ed88f6c7788a573e3aa561cd580b7f01",
	"298479cf8a824c945c1f9255bdb90e2742a719ddad35e5b5c1eb98764fa70a323bc4030112b363d125ddbb0e5d221701cd288d16ccc3873b495b7e63ab0fcc17794bc17107010900",
}

// Gen hardcodes genesis default settings
var Gen = &Genesis{
	InitDelegatesPubKey: initDelegatePK,
	ChainID:             uint32(1),
	TotalSupply:         uint64(10000000000),
	BlockReward:         uint64(5),
	Timestamp:           uint64(1524676419),
	ParentHash:          common.Hash32B{},
	GenesisCoinbaseData: "Connecting the physical world, block by block",
}

// NewGenesisBlock creates a new genesis block
func NewGenesisBlock() *Block {
	cbtx := trx.NewCoinbaseTx(ta.Addrinfo["miner"].RawAddress, Gen.TotalSupply, Gen.GenesisCoinbaseData)
	if cbtx == nil {
		logger.Error().Msg("Cannot create coinbase transaction")
		return nil
	}

	votes := []*action.Vote{}
	for i, pk := range initDelegatePK {
		pubk, err := hex.DecodeString(pk)
		if err != nil {
			panic(err)
		}
		address, err := iotxaddress.GetAddress(pubk, iotxaddress.IsTestnet, iotxaddress.ChainID)
		if err != nil {
			panic(err)
		}
		sign, err := hex.DecodeString(initVotesSignature[i])
		if err != nil {
			panic(err)
		}
		vote := action.NewVote(0, address.PublicKey, address.PublicKey)
		vote.Signature = sign
		votes = append(votes, vote)
	}

	block := &Block{
		Header: &BlockHeader{
			version:       common.ProtocolVersion,
			chainID:       Gen.ChainID,
			height:        uint64(0),
			timestamp:     Gen.Timestamp,
			prevBlockHash: Gen.ParentHash,
			txRoot:        common.ZeroHash32B,
			stateRoot:     common.ZeroHash32B,
			blockSig:      []byte{},
		},
		Tranxs:    []*trx.Tx{cbtx},
		Transfers: []*action.Transfer{},
		Votes:     votes,
	}

	block.Header.txRoot = block.TxRoot()
	return block
}
