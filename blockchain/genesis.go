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
	"2450b21ecd74b10a7a33ba7b132af2db3ae97f0a831ea10d40ec03b47652127ee62d1e0346a902eac583295bd0023e08b1f27279927d5959d5826671474de4f7646758b6b6b5ff03",
	"f487cdb2481632cba991f8f8a8748b10ec0e51210c79fbbb4c67baddc1f08da709627405ea008017f4a371c6edcbdeb0a92716fde574aa96eab05d6c6e46f6dd9663ee8ae7c4e501",
	"ca29fa51cf4da6534652497facf274451eb5bd8b4851b410d5061b7609d4817ce6351c010e37c27220477f027ac2c05a8c9a6052289c7aaa0f4c6635949211ea9a07f209b48be903",
	"99ae3505d5363b3af777d2057cf9987c21e8bdacf39d65bfffae36bd2e8b563ba2988601ecb66c4e89d4dbefd521a9400ba0c570ab3417bba1d9b2950fbba83380bec7cdf7cfeb04",
	"db43c524d5b3a705202ca9ce7b4060d56b88871902799b662b03e38b339b81b509b6b4013883badb2877110b7a8a4487b5283d0b3a251d5a87b7d3b9c22c063a658f7a131ddf8e05",
	"02cb2a2fb698dd542ba8562109ef08c59d42945b51445ead50875d1f284d0b72850f7200a3b06bc66cbf63cc09b9442087cef3b3689eae1241c07d36d0246b85e83745e9bede8702",
	"b11a92181026df13dfb4fc8d3bd5cc27cd872d20f578697268c8479aa53efed0c6b2810118c208912d338a3ea5b7023dbc968fae5bdeba409bc0c3b56ad50720d1238187b9cead06",
	"d6639908f068a181397e0c934422de153eee8da43b38fa4986220d503225ef038d331e0161300749cf08b01eecd37eadd5ac7d3b3a6ef9346dbce07779ac41a8faceb523cd6dab07",
	"55ee08599d1b1c89cf4b205dd312627bd5e718a2d228584b882e944e14a8d22bdfc91b019a6ac7003474bc2bae936136c598065b3bad2f87776f26daf4b1cdd50af75f8edf2f1103",
	"902bdc0b555eb2648a9187e109e14e5675e662a73a417f93416ef17e3c6979109b231002b08c506839c2988ff592236c1fd9fb11a00d771fb3696752b599e9777fd1c7e9fd3de001",
	"182f1f249f9d77a6c7464e5479bb3075b985dadf7fcc969db0f31d3a38ba19b62f0e99044cbb091281b7d2cbb17f779a547cee6a4cc1fbf395b768fc5a1773f322cc3932af33fe01",
	"411011a284f646d62ab4ca885ba964eaba4e5172b12a18427af1400bf9771cf8238a7b060b886944f9e3c446927af0f616031c181839c9e863a081b6ce64dc89af62dbe15c3fc706",
	"3463fb35b5d49b205cf6a0fce61cebba4b9cf17505ff4d4a7e2a77d1a155c94a4e7b64005e859c8777ca8f09291a579d9e2bfb3ec5465e3a771287254bc9e913f74a0192db491d04",
	"480574e03fca43fee6fbffede5f37d0e677e4e5ed3b35397834b1cfe369c91b693429f008b459f50373588b322a34f9a0c61425c9669c60b4f24516cbfa8affcd5bc0da3257d9f06",
	"45272fc7acd408bdebad966ba41b22bb600ec5e5ea27675b40e08cbe15dcd438c78085032a82f9cf2258940145604aebca99b0379c388d1676b96777adfe8bcf120338cc94481904",
	"4b6c8bb3f19f2bd50363018a886a8c031f2ec6df8783639e6bb3f1a78c17dd1faae14e066838287b76911051bb3a19d9f937d030f3b78f75867c19722d5ef5bfca55104651274705",
	"03ab40d41ed5dde5398996cf628da79b28df2435a129eeb0a03e449d1c2ef1ad5adc71004c5756e0e4406ad53cce193885385eab64a19408f9b48246d97cb5d819f715a02aa6f602",
	"fc8846cbcb1d64ceb1abe50e02cf00ff7befb8d933c72c09d561097b13179a9ee003d305fc291923fcdf9aae94385c51e5eb5df2653a91a1e0773363afa87abfd7bd15f5e5969706",
	"5e01409e8f63bb0b6490627196edcf0dd926ffae9d3bc24f661a03135e5f68f0751fbd02c811f3a1587b8a0cdff0cc2b01972e97ffd2588c37ac6cf205ea64bc940fd408cfd2f303",
	"99719c65313b389f927dc1c3021bfa310fedd85836d4e572e02fc7bf26af7c9ef2925103ac5635b4c069ced4e30aac7726070151349cbe1e597948531f6c0e4c6c31a38604de0d02",
	"4ce3a5279c682fc79dd879bfaa725e48b8c83f4d164cf2c08a5dc661b37daedac953cc04504a924444b9a1f294da374b8f82acdfce16f692d2da131a4413dd9e9cf29552fd0fc707",
}

var initVotesSignature = []string{
	"df99073512d885af9ae2355d2912e454748ed9b8de7a2836aed5d80ab20bd0bde6d2ee01bdfa5817790ed8c075b2914b0f5b817285bcabe45d0d021a705a4334a9ec607cadabe101",
	"2dd4c9b8d4e4fb8108bac96a950a661dcbbf1fd4bdc6a03bdada5291ecd8bd0572f35d001d4aa2525778c841d414a722178a333695b6bac11ed652e2d3b13d34ba6706c06268c501",
	"82a76b9d0a5759cee447a7a8d81a6cda512fe97ff7f2b3f2aacfd1722a8daec3d617f50100163157021d5da11d9e0a1b6dffab6e56cc044d959b4e1230a2cd4f12787a3382828201",
	"77f79cd6b6ede28d7dc0150c59337d9984155c19a9011a564a74a88ba122693a858484007502fb95ad7762b8849a093aa35aad7ff561a5778ea823e1a9b1dc4c04bb718649d53801",
	"a05e68906a5d9c3bf47fe47b3f6d726be92e1282554dca35fa4e41fa40f9ca91ef2c9e01cf3814cb08a4a6b972b47cdb35e6356eb478631db7d28be3e4269b27014a6406b9d48800",
	"a960e403181e55b00084a38ba79548d6f2b7143a51b5e875c1671f3bb922fe8bd3d5ed012a6b88af49cd89da2d634c957460efdd384bac20ec8ef3347f3bdc6812fc0298f096c201",
	"5f3956c21f85a5974709a264cf4cb8b34d225c0a81e5753fabd96cd2e550fe562401ad0083476c7ba6dd57d58eeac2418614fd58ae4e1a9cba5f5521be4d2670eeccef0afad2c500",
	"498f928a62bc80354e2f66609f0b1bd364a9804eaf5bd15e71b276186d54698e78c1a80104c26fa9baf2dfd8401b03d29ce86f10bb56b9dfd04f6de85061c9e575107abf46c64501",
	"1afbe70dc1a04a98414fa1168db1764d1aba290fc8322f2ebbc6f7faed57832085ff9a01b429b9c9e02b69f395d298a903a9582a5f13cf9c8c9c6b9aa10b066592696417cbacee00",
	"2e7a382add476640bf2fdd27920413ce881622eebac33be88ee2e4dc5356de0dee39900083b28a9146e5b5eae22a13155d02c10fc25884fd0487b636ed6cd228bb6bd8c05b3ef100",
	"c9f497d2de58c69a9a9573d95d41188668b86b572bc16b8786e7d29aa8f645e56a1c9101fc04bb2fdef7eb38e8724bb5e4fb3cec517fb3a1e1f230fb0886cfc0b3984dee42e53f00",
	"c4b210b212ace78eb09e723490e68617ce5f36fde4fc62e5af1dcd82f6a0486a60eaa6013ff32d090aa9f6b86856ac17904e5d4f471d3d3bffda3885e68c8d490a4729d3ba0a2f00",
	"b40a32bb847089c52c06d6545155bb86160dad2b883150842a0a458cc4b0c2421c4812009c24ebf0a69d376b4eb3ca35a14fd20176df0e491b690458c578ca56e67a9e35f9f98901",
	"feeaf7e160418add6b9de7854a6fc57a6da1fe473d047c360b2a6d6bc3424d602442c200135f93bcc373b79e108c9246340a26ac628e9234f670291f00bf0e62f44c322ed6770300",
	"62400e0822ddf17ead2d26cf99cdd91530e399ee2ed201d50a1d5c33fed3705a6dcfcb0115f3f24e3faf99ed51cc9d6fef0ea07020d0d889e6650bba04492581e085a508908e3b00",
	"feefda129e7a323fc6db684026fc90f82fa4e8b69acb3d4c4adc60a822578e6e157e610002759d03c706b224a6639cd3d2d874bc69f560452d5a428c25b9a8c2a3b12fc488df5a01",
	"a3c759e20843a50405179bbff80b9d927abfd1cd405404396b43c04239eab0aa57d4980102da39bd380d604f889fd7b57ecfc9760e50a4af2a25ceb2630e6c88173112403a418900",
	"09bc2385aa7f780083248c9cc1e10599b198ee425af58986bea2a1ac90ec8e21c969720001fa1ef4285ae9a19cc17c91018177ff1e07008cdcfea9db88ac2ef3c90e465a04fe5900",
	"e8004fdcc2078bd82a5d7c8c820ab89fbf939bd62feef9e16c55f9f0482a1602f67da10112584a9ef951a8d01d614c4596f749a0e8933e795404edf14968dfe603fc006f483ffd00",
	"af60b29d9a6df984a99ff3ef1a971c293c3e002d457517029ebe29aca272f0bec75f4a019a059a539c50a74425dc71c62da9487520fc7bafdb99abf0b7f0a2d70487ac1f1c2e5701",
	"3e2a0a8fee2202690e5a85ee3250611a8eb76aae47c9b5102dd386e00139d046d3296a01bad3ec561256defe97c85e7559f94978475cfaeee29e45540bd8a4a8c4dc3ca4fbd37200",
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
		vote := action.NewVote(1, address.PublicKey, address.PublicKey)
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
		Tranxs:    []*trx.Tx{},
		Transfers: []*action.Transfer{},
		Votes:     votes,
	}

	block.Header.txRoot = block.TxRoot()
	return block
}
