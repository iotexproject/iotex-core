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

// GenConfig defines the Genesis Configuration
type GenConfig struct {
	ChainID uint32
}

// Genesis defines the Genesis default settings
type Genesis struct {
	InitDelegatesPubKey []string
	GenConfig           GenConfig
	TotalSupply         uint64
	BlockReward         uint64
	Timestamp           uint64
	ParentHash          common.Hash32B
	GenesisCoinbaseData string
}

// PubKey hardcodes first 21 candidates to put into candidate pool
var initDelegatePK = []string{
	"8a15551c4b27214bd564bda7a91cfcc29e1e7777774dbd8c8de151de9ed8f57cc007aa01fad104f9793188f2ca2b8482f3773ffb6fa8c8a644942540d710b5a6fc0f32a51823c007",
	"e3e5546eb1c0c4d11cf15b02b36c0bea2ec7707c9fe24dd62a519209daa09537eba6aa0467a64de07fcb624658ed8446333c6f25610516b7616bb2e4ec7e6cba7df8912d72133307",
	"cc8725215d18ce14e37ffdd5ced5ee1006975dd06a1b74408240db4df03d8b425f0c6d0504fb16239baffefd8ab18a1930dbef1274e8ada72401f730df2bece8e8ebfc0b1755d006",
	"5f7bf9305adfb20243a5e909b8fe480fe00b46ceda6f4074589425278ec70edea2a0e7023af256900bc652c7b4627019ccedb234e4c412f69b0b85fea59bc998edbf59d9f4896605",
	"69af66d143a3245b83ea4db2fd12cc891dcd1e86175aaf1181af63421fed7a3e12f97302be631afda17857e4fa5ce3076ad02fb53c64607b5e484bc43621002a403b0934f0ee3405",
	"c8d9b6bf72df3209eb6bea6762f22aa6273b4f4c6b7b5a49b48eea1cacb497c78d5b290663f89ef374fdd9cc3a3acd28daf457367f5c116e3a37cea196d7152c05a3943338a9c800",
	"e68730c04d7e3a91b5b26c84db6fcc57d0e0370c9991512436acb655c661dc44f5074600672d8a6ef9bc4f265634d6282fdbd257d6d18d641bd9293f9142290c6a439f3572d54202",
	"951852cf47370b14b7c1f32049bef9cfa91abdcb56f93faae693f75f095920f54b98e70013a17be8eac79bcfac17b959ab971733033883677fcba73136b981b133bf975e185d6a07",
	"50954738369da46d2e7628a3dc7db74a538a7a1459e2a26d3c6e25a4fd392474b91b450207242baf200d2f514282b987674a980b59e55e07c8c5f03ff966cdaf8a93e366163aa902",
	"7b8e70298f55037e90d03e06b3ed21bd82569d238d3e06fa0effa79ae40162a4f3e7260169501590a27e5b9f989987a996525e8ee83ed8305a1b654bf6fb270d6617618aa1accf01",
	"557e306c363d7e61c7e7cf19d395ebb6889e02df9c9c8084753764f0c1ae807486d35d07586c9f20fcf2b0214e135fdb09ded972c947490ba2b699550fe0785ebd043f205e3f2a01",
	"0a0e3b1e0f6303f4639edf9f102963f398f7a8c6d77120bc3aadae7dce374502515f5200b5607d020adf749dc10ed8186806dece2f569a75df87403bd2a6feb812855c881707ca04",
	"5e785c8dce54c9c6ce9fd18e653e59abf9e4b3d8970adeee41b982949670bdc4ed66510521c4bbfe425f0f160bd49e7e1328d17a11c8292187e0b2ef22b7b121a51460da2d5e4d00",
	"3fe891269da9ecca4ab5183327a6e96eb26abcbd59070d76a56ec988b00449d055e2f10422e075ffcb8afa772628d941f84778760e3a300efd4cd6713eaa8e528af8a99e861f5400",
	"058e46f934d216cec5a3defe523585dac4998ac268b34385f9f4d3160aa1734040fc280166595277a3afbbee35c86d1b667ed9f73a8bdc4e8b71ff042fcc95ad7f2e033d33cf7202",
	"167bb838a4454edcc0ad55b8fee7cbc81583cc70464040fe2ce6d7307812bbf908611e0379c7bf849f5351e617d04a90be4ea8d4718f104a77e5feb8eed3cbf91f7aeb8fe9edb201",
	"ce08a683caf9f9b3e12f8239b6b165a6ccac745cd2cc7002f7c7b5d3ac78b2dcef1cb2034fefcf3e6c693d850cf4913912ba9eb3ab129f44bbb23d3edde68397ee27f7eae3bd8a01",
	"82147c7e3d30f1c7873f214e5300012f24c69c233eda5a36bd87c5943747dfd99d3fe603279addce5b6ef34b6a0a352f0a0fb4903413554c242bc88b7671fa5d7c0b2af589656103",
	"300a4587b07bba5fe58a2ebaef0a34e2b6b53dc3d6b71874091b5954eb4f6df544374500c118d9928acd6e5f52677e0ebd547bc9c39de863a59888d7ee291d0fc5834e95f5b84a00",
	"9d085995544f962115263531aefd9783a50cc9bc7b63dccec5a1c9945553b4d7aeecd306cdb034a0f87231e51bc2f3263444bb977fcc65381f4035aebb26a2af34b1593fa3e44903",
	"b2e13934d36c737d441f8013260f7c8fa53638796b61c25a6b80dc09f1e64178af68460181a8416a974468c6cce2bb5a433ae2d025ab3c47d3eb20ac8d068903e4e27a5eb6ba9e06",
}

// Gen hardcodes genesis default settings
var Gen = &Genesis{
	InitDelegatesPubKey: initDelegatePK,
	GenConfig:           GenConfig{uint32(1)},
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
	for _, pk := range initDelegatePK {
		pubk, err := hex.DecodeString(pk)
		if err != nil {
			panic(err)
		}
		address, err := iotxaddress.GetAddress(pubk, false, []byte{0x01, 0x02, 0x03, 0x04})
		if err != nil {
			panic(err)
		}
		vote := action.NewVote(0, address.PublicKey, address.PublicKey)
		votes = append(votes, vote)
	}

	block := &Block{
		Header: &BlockHeader{
			version:       Version,
			chainID:       Gen.GenConfig.ChainID,
			height:        uint64(0),
			timestamp:     Gen.Timestamp,
			prevBlockHash: Gen.ParentHash,
			txRoot:        common.ZeroHash32B,
			stateRoot:     common.ZeroHash32B,
			trnxNumber:    uint32(1),
			trnxDataSize:  0,
			blockSig:      []byte{},
		},
		Tranxs: []*trx.Tx{cbtx},
		Votes:  votes,
	}

	block.Header.txRoot = block.TxRoot()

	for _, tx := range block.Tranxs {
		// add up trnx size
		block.Header.trnxDataSize += tx.TotalSize()
	}

	return block
}
