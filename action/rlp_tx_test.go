package action

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/config"
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
		data:           signByte,
	}
	hE1, _ := hex.DecodeString("fcdd0c3d07f438d6e67ea852b40e5dc256d75f5e1fa9ac3ca96030efeb634150")
	rlpExec1 := &Execution{
		AbstractAction: ab,
		contract:       "io1x9qa70ewgs24xwak66lz5dgm9ku7ap80vw3070",
		amount:         big.NewInt(100),
		data:           signByte,
	}
	hE2, _ := hex.DecodeString("fee3db88ee7d7defa9eded672d08fc8641f760f3a11d404a53276ad6f412b8a5")
	rlpTests := []struct {
		act  rlpTransaction
		sig  []byte
		err  string
		hash hash.Hash256
	}{
		{nil, validSig, "nil action to generate RLP tx", hash.ZeroHash256},
		{rlpTsf, validSig, "invalid recipient address", hash.ZeroHash256},
		{rlpTsf1, signByte, "invalid signature length =", hash.ZeroHash256},
		{rlpTsf1, validSig, "", hash.BytesToHash256(hT1)},
		{rlpTsf2, validSig, "", hash.BytesToHash256(hT2)},
		{rlpExec, validSig, "", hash.BytesToHash256(hE1)},
		{rlpExec1, validSig, "", hash.BytesToHash256(hE2)},
	}

	for _, v := range rlpTests {
		_, err := generateRlpTx(v.act)
		if err != nil {
			require.Contains(err.Error(), v.err)
		}
		h, err := rlpSignedHash(v.act, 4689, v.sig)
		if err != nil {
			require.Contains(err.Error(), v.err)
		}
		require.Equal(v.hash, h)
	}
}

func TestRlpDecodeVerify(t *testing.T) {
	// register the extern chain ID
	config.SetEVMNetworkID(config.Default.Chain.EVMNetworkID)

	require := require.New(t)

	rlpTests := []struct {
		raw     string
		nonce   uint64
		limit   uint64
		price   string
		amount  string
		to      string
		isTsf   bool
		dataLen int
		hash    string
		pubkey  string
		pkhash  string
	}{
		// transfer
		{
			"f86e8085e8d4a51000825208943141df3f2e4415533bb6d6be2a351b2db9ee84ef88016345785d8a0000808224c6a0204d25fc0d7d8b3fdf162c6ee820f888f5533b1c382d79d5cbc8ec1d9091a9a8a016f1a58d7e0d0fd24be800f64a2d6433c5fcb31e3fc7562b7fbe62bc382a95bb",
			0,
			21000,
			"1000000000000",
			"100000000000000000",
			"io1x9qa70ewgs24xwak66lz5dgm9ku7ap80vw3070",
			true,
			0,
			"eead45fe6b510db9ed6dce9187280791c04bbaadd90c54a7f4b1f75ced382ff1",
			"041ba784140be115e8fa8698933e9318558a895c75c7943100f0677e4d84ff2763ff68720a0d22c12d093a2d692d1e8292c3b7672fccf3b3db46a6e0bdad93be17",
			"87eea07540789af85b64947aea21a3f00400b597",
		},
		// execution
		{
			"f8ab0d85e8d4a5100082520894ac7ac39de679b19aae042c0ce19facb86e0a411780b844a9059cbb0000000000000000000000003141df3f2e4415533bb6d6be2a351b2db9ee84ef000000000000000000000000000000000000000000000000000000003b9aca008224c5a0fac4e25db03c99fec618b74a962d322a334234696eb62c7e5b9889132ff4f4d7a02c88e451572ca36b6f690ce23ff9d6695dd71e888521fa706a8fc8c279099a61",
			13,
			21000,
			"1000000000000",
			"0",
			"io143av880x0xce4tsy9sxwr8avhphq5sghum77ct",
			false,
			68,
			"7467dd6ccd4f3d7b6dc0002b26a45ad0b75a1793da4e3557cf6ff2582cbe25c9",
			"041ba784140be115e8fa8698933e9318558a895c75c7943100f0677e4d84ff2763ff68720a0d22c12d093a2d692d1e8292c3b7672fccf3b3db46a6e0bdad93be17",
			"87eea07540789af85b64947aea21a3f00400b597",
		},
		// execution
		{
			"f9024f2e830f42408381b3208080b901fc608060405234801561001057600080fd5b50336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555061019c806100606000396000f3fe608060405234801561001057600080fd5b50600436106100415760003560e01c8063445df0ac146100465780638da5cb5b14610064578063fdacd576146100ae575b600080fd5b61004e6100dc565b6040518082815260200191505060405180910390f35b61006c6100e2565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b6100da600480360360208110156100c457600080fd5b8101908080359060200190929190505050610107565b005b60015481565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141561016457806001819055505b5056fea265627a7a72315820e54fe55a78b9d8bec22b4d3e6b94b7e59799daee3940423eb1aa30fe643eeb9a64736f6c634300051000328224c5a0439310c2d5509fc42486171b910cf8107542c86e23202a3a8ba43129cabcdbfea038966d36b41916f619c64bdc8c3ddcb021b35ea95d44875eb8201e9422fd98f0",
			46,
			8500000,
			"1000000",
			"0",
			EmptyAddress,
			false,
			508,
			"b676128dae841742e3ab6e518acb30badc6b26230fe870821d1de08c85823067",
			"049c6567f527f8fc98c0875d3d80097fcb4d5b7bfe037fc9dd5dbeaf563d58d7ff17a4f2b85df9734ecdb276622738e28f0b7cf224909ab7b128c5ca748729b0d2",
			"1904bfcb93edc9bf961eead2e5c0de81dcc1d37d",
		},
		// stake create
		{
			"f87402830f424082520894000000000000007374616b696e67437265617465809012033130302a090102030405060708098224c5a01f1eb2b9dd9b9af61b68e9e2dae9c65421e0c6e84bc2bf77c1528d3d795cd054a01a8f4b78a7f0e96d578bf2c3b2c3bfe48b24d026e7cadf2ccd6d2e332ed2128f",
			2,
			21000,
			"1000000",
			"0",
			"io1qqqqqqqqqqq8xarpdd5kue6rwfjkzar9k0wk6t",
			false,
			16,
			"ab7e6135186a3f9b8d8f7c31e428635382a2f04974ab43d8eb5c5f2036f05dec",
			"0468f50e1da7d0430b01d81ff24714cef5aea6a7a2b96271c7fff60c6a67896d201547001b8ee6ca0166e219f7303b62236c36ad5e79f6012e8e7eea4aaa99ded8",
			"d301e82c10561c6048f2d72a6630bfa87a8703f2",
		},
		// stake add deposit
		{
			"f87602830f4240825208940000007374616b696e674164644465706f7369748092080112033130301a090102030405060708098224c5a0a4aed9fd71e6c791660ea1387005c2955aa9a898554756bbeafa2a47aa896059a04eaf04155845678374a4f5bf2f574874609a912fb02d88c6125cc7422c96a7ec",
			2,
			21000,
			"1000000",
			"0",
			"io1qqqqqum5v94kjmn8g9jxg3r9wphhx6t58x7tye",
			false,
			18,
			"2d2931e804596dc3fe67edec97ae2abeee3b18dd5e50b1becf746cd8f422b734",
			"04c175e6f0470c3f372cb4b5ea38ac18cb957c7b9a96402d640ab24964e5642055b743a0bf7c90a43052c877f62668e05c968ba88a19b0b869553c19a6d7950934",
			"b2721edbd35f2fb19910cc8d6105bf35f3514785",
		},
		// stake change candidate
		{
			"f87702830f4240825208940000007374616b696e674368616e676543616e64809308011204746573741a090102030405060708098224c5a0305710c930636bf395fc5551ed61f3c0cc10c68a105fa6982da4f2687d29fc09a03879c1d182c36f5dd71578851dee43c5f38914b8b921a0fff0e1871c25ad834f",
			2,
			21000,
			"1000000",
			"0",
			"io1qqqqqum5v94kjmn8gd5xzmn8v4pkzmnye5v3fh",
			false,
			19,
			"e67bc6279d1ca53cecad37c1948c1da88392b215d62e8d4d9ff33b26ec5e1b25",
			"04d56527007d0655441b79a460e04a4eb60ae830394dbe3ea6155b604702c103a6bbdf8a6fb3df3c7b5a8f84e666d17eb55f4a032282b116102737a9a85a39dbd9",
			"a3f31665c7bf5435f8d1c5814290998bfdc91cfc",
		},
		// stake unstake
		{
			"f87102830f4240825208940000000000007374616b696e67556e7374616b65808d080112090102030405060708098224c5a0914abd6dec1c4393348ebee59de14c60f249109923c4c54a6f0825cc1039fe28a02ddf1698d3dd1887d1cea8fb3acba99ddfbdd15b331c302b2211f528b54b08f1",
			2,
			21000,
			"1000000",
			"0",
			"io1qqqqqqqqqpehgcttd9hxw4twwd6xz6m9pl4r27",
			false,
			13,
			"b1d7626b1dbd58d96b4952096085b0cbffbbe6f8524bcd841dd4f3c4fe81e547",
			"049413ebb87a801a8f5f08d033180ae02b43566983fcb710ea070d37c2759e9d055b4952822fab93ea310de3e68ec334abbe5d25feaf098a2bd80389f6a4873493",
			"7da86cdbeb81059ce8b0a58d36360b0a2fbb9bfd",
		},
		// stake withdraw stake
		{
			"f87102830f42408252089400000000007374616b696e675769746864726177808d080112090102030405060708098224c6a0128f64bf0b9aa994c2216dd783359af176c6a01e9014e4e2f3fb9838fc4a2c36a05a78b25dec739cb33a8fcc5f4f134b63bf7a3df20bd9c7d5db53cef46d6b38ca",
			2,
			21000,
			"1000000",
			"0",
			"io1qqqqqqqqwd6xz6mfden4w6t5dpj8ycthwsq5ng",
			false,
			13,
			"d194072b45e0ec737dfd01ed3860aa6b13572cd04a6856499c6943ec57f90751",
			"045d1b360af12b4f97a714530bcf3cde06cb89749604fc22418d0e3b26b63e07878eb50d6fbbc35e62aa103fd0b75e722328ee11932b82b6eadae01cef658e031a",
			"fefc74b9c11bfdb0fd1db767e35e7d1f943be9fb",
		},
		// stake restake
		{
			"f87302830f4240825208940000000000007374616b696e6752657374616b65808f0801100a22090102030405060708098224c6a0d4c6731cbc8e08dc8fe82dac7fc226b2f9561414c05a66ac152cbd44bd822ebea01c0f8b9985d84d73e8a5221c465944e29af5b524e82ab9eb4f3b89ff6720cb86",
			2,
			21000,
			"1000000",
			"0",
			"io1qqqqqqqqqpehgcttd9hxw5n9wd6xz6m995w4zm",
			false,
			15,
			"0043a8f8235d4440dcb8d0dbde9f5f70700bca447d4ca22790bdf3480b7e0864",
			"0474571cdde1f8ac340aad09fb9c20f01ae7c4686c40520ed00e529636421b2bb4d278527d757218c25a489ff37f1c11fcb670432fd310e4eeb67169f6c509145f",
			"a903703f05d42a7235d88bfb327f7fe9bf09c08e",
		},
		// stake transfer
		{
			"f89d02830f42408252089400000000007374616b696e675472616e7366657280b83808011229696f313433617638383078307863653474737939737877723861766870687135736768756d373763741a090102030405060708098224c5a088b51f422f5d52cc195aeeb48420b72daccf20236e3cfba253a1c6e5d182bd11a07324d43e047f66764105b469b40b1760ac25b0644b4fc45466becc9ea615c76d",
			2,
			21000,
			"1000000",
			"0",
			"io1qqqqqqqqwd6xz6mfden4gunpdeekvetjzwh99y",
			false,
			56,
			"1d92468b3f4a6deda63197fe0b6b774f5c045d7f4c276fcd3ecafdf095fdf1be",
			"044d93a3ddf6b9e3a9c14112850fa67174933198fa06c283e3069f9b91af63ceeeac828c1409615d9e3a1a2187b06d91d9a95115cc327a6f9310a8c99adecd023e",
			"b5c7dc76be22680743eff409f2fbdfaff1286bd4",
		},
		// stake candidate register
		{
			"f9010102830f4240830186a094007374616b696e67526567697374657243616e6480b89b0a5c0a04746573741229696f313433617638383078307863653474737939737877723861766870687135736768756d373763741a29696f317839716137306577677332347877616b36366c7a3564676d396b7537617038307677333037301203313030180a2a29696f316d666c70396d366863676d327163676863687364716a337a33656363726e656b783970306d7332090102030405060708098224c6a04648492f928753fa39c3aac5865be6e37515577cf1debecd629dde93f0e5aaffa00dcccd54bf329e7d11a51053a23ddd1cf9329e5ae01c4d39d596c736dd215196",
			2,
			100000,
			"1000000",
			"0",
			"io1qpehgcttd9hxw5n9va5hxar9wfpkzmnyahxhjk",
			false,
			155,
			"906119b6475a75c7b72fb083f19fc5a4e2d47a800b5564f07dc4c80d86d89627",
			"04b8b3978dad1591e511199cfb926eda5c8d8b3bef34febde233565887f3ee548e70a688c0241a5531f28fa20ee9cddb70c884b62a91ef945137f95864ee8865c7",
			"faf3c23ff653bc1f0139cb3ab4b92578d636c6c0",
		},
		// stake candidate update
		{
			"f8c202830f4240830186a0940000007374616b696e6755706461746543616e6480b85c0a04746573741229696f313433617638383078307863653474737939737877723861766870687135736768756d373763741a29696f317839716137306577677332347877616b36366c7a3564676d396b7537617038307677333037308224c5a0245f4874b1bb86dfae869b5f35daa07faa735bdfa1d42c0d7a7ec669835b40eea065d9ab7b7f94da4a80258b694359bcb604aee74540e6e4dbca0caf102f89940f",
			2,
			100000,
			"1000000",
			"0",
			"io1qqqqqum5v94kjmn824cxgct5v4pkzmnyxxj98n",
			false,
			92,
			"9dcd5da20c43860f3a2ee099821949cf615942c1477b289dbef39c0d4087672f",
			"04f79d369da4ae82d2e549f8ddc39cb38e1375434e595e5e5212d1db81ed7c7dbd319bb84f4654092de21f4f820e09467687f26a12420c1fca298f2a50f95455b5",
			"47525b648d809d07c8878f9dd4b140724e3ce783",
		},
	}

	for _, v := range rlpTests {
		encoded, err := hex.DecodeString(v.raw)
		require.NoError(err)

		// decode received RLP tx
		tx := types.Transaction{}
		require.NoError(rlp.DecodeBytes(encoded, &tx))

		// extract signature and recover pubkey
		w, r, s := tx.RawSignatureValues()
		recID := uint32(w.Int64()) - 2*config.EVMNetworkID() - 8
		sig := make([]byte, 64, 65)
		rSize := len(r.Bytes())
		copy(sig[32-rSize:32], r.Bytes())
		sSize := len(s.Bytes())
		copy(sig[64-sSize:], s.Bytes())
		sig = append(sig, byte(recID))

		// recover public key
		rawHash := types.NewEIP155Signer(big.NewInt(int64(config.EVMNetworkID()))).Hash(&tx)
		pubkey, err := crypto.RecoverPubkey(rawHash[:], sig)
		require.NoError(err)
		require.Equal(v.pubkey, pubkey.HexString())
		require.Equal(v.pkhash, hex.EncodeToString(pubkey.Hash()))

		// convert to our Execution
		pb := &iotextypes.Action{
			Encoding: iotextypes.Encoding_ETHEREUM_RLP,
		}
		pb.Core = convertToNativeProto(&tx, v.isTsf)
		pb.SenderPubKey = pubkey.Bytes()
		pb.Signature = sig

		// send on wire
		bs, err := proto.Marshal(pb)
		require.NoError(err)

		// receive from API
		proto.Unmarshal(bs, pb)
		selp := SealedEnvelope{}
		require.NoError(selp.LoadProto(pb))
		rlpTx, err := actionToRLP(selp.Action())
		require.NoError(err)

		// verify against original tx
		require.Equal(v.nonce, rlpTx.Nonce())
		require.Equal(v.price, rlpTx.GasPrice().String())
		require.Equal(v.limit, rlpTx.GasLimit())
		require.Equal(v.to, rlpTx.Recipient())
		require.Equal(v.amount, rlpTx.Amount().String())
		require.Equal(v.dataLen, len(rlpTx.Payload()))
		h, err := selp.Hash()
		fmt.Println("h:", hex.EncodeToString(h[:]))
		require.NoError(err)
		require.Equal(v.hash, hex.EncodeToString(h[:]))
		require.Equal(pubkey, selp.SrcPubkey())
		require.True(bytes.Equal(sig, selp.signature))
		raw, err := selp.envelopeHash()
		require.NoError(err)
		require.True(bytes.Equal(rawHash[:], raw[:]))
		require.NotEqual(raw, h)
		require.NoError(Verify(selp))
	}
}

func convertToNativeProto(tx *types.Transaction, isTsf bool) *iotextypes.ActionCore {
	pb := iotextypes.ActionCore{
		Version:  1,
		Nonce:    tx.Nonce(),
		GasLimit: tx.Gas(),
		GasPrice: tx.GasPrice().String(),
	}

	if isTsf {
		tsf := &Transfer{}
		tsf.nonce = tx.Nonce()
		tsf.gasLimit = tx.Gas()
		tsf.gasPrice = tx.GasPrice()
		tsf.amount = tx.Value()
		ioAddr, _ := address.FromBytes(tx.To().Bytes())
		tsf.recipient = ioAddr.String()
		tsf.payload = tx.Data()

		pb.Action = &iotextypes.ActionCore_Transfer{
			Transfer: tsf.Proto(),
		}
	} else {
		ex := &Execution{}
		ex.nonce = tx.Nonce()
		ex.gasLimit = tx.Gas()
		ex.gasPrice = tx.GasPrice()
		ex.amount = tx.Value()
		if tx.To() != nil {
			ioAddr, _ := address.FromBytes(tx.To().Bytes())
			ex.contract = ioAddr.String()
		}
		ex.data = tx.Data()

		pb.Action = &iotextypes.ActionCore_Execution{
			Execution: ex.Proto(),
		}
	}
	return &pb
}

func TestRawData(t *testing.T) {
	// register the extern chain ID
	config.SetEVMNetworkID(config.Default.Chain.EVMNetworkID)
	require := require.New(t)
	addr1, err := address.FromString("io143av880x0xce4tsy9sxwr8avhphq5sghum77ct")
	require.NoError(err)
	addr2, err := address.FromString("io1x9qa70ewgs24xwak66lz5dgm9ku7ap80vw3070")
	require.NoError(err)
	addr3, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)

	// create staking action
	pvk, err := crypto.GenerateKey()
	require.NoError(err)
	ab := AbstractAction{
		version:   1,
		nonce:     2,
		gasLimit:  100000,
		gasPrice:  big.NewInt(1000000),
		srcPubkey: pvk.PublicKey(),
	}

	testCases := []struct {
		Action
		addrHash address.Hash160
		dataLen  int
	}{
		{
			&CreateStake{
				AbstractAction: ab,
				amount:         big.NewInt(100),
				payload:        signByte,
			},
			address.StakingCreateAddrHash,
			16,
		},
		{
			&DepositToStake{
				AbstractAction: ab,
				bucketIndex:    1,
				amount:         big.NewInt(100),
				payload:        signByte,
			},
			address.StakingAddDepositAddrHash,
			18,
		},
		{
			&ChangeCandidate{
				AbstractAction: ab,
				bucketIndex:    1,
				candidateName:  "test",
				payload:        signByte,
			},
			address.StakingChangeCandAddrHash,
			19,
		},
		{
			&Unstake{
				reclaimStake{
					AbstractAction: ab,
					bucketIndex:    1,
					payload:        signByte,
				},
			},
			address.StakingUnstakeAddrHash,
			13,
		},
		{
			&WithdrawStake{
				reclaimStake{
					AbstractAction: ab,
					bucketIndex:    1,
					payload:        signByte,
				},
			},
			address.StakingWithdrawAddrHash,
			13,
		},
		{
			&Restake{
				AbstractAction: ab,
				bucketIndex:    1,
				duration:       10,
				autoStake:      false,
				payload:        signByte,
			},
			address.StakingRestakeAddrHash,
			15,
		},
		{
			&TransferStake{
				AbstractAction: ab,
				voterAddress:   addr1,
				bucketIndex:    1,
				payload:        signByte,
			},
			address.StakingTransferAddrHash,
			56,
		},
		{
			&CandidateRegister{
				AbstractAction:  ab,
				name:            "test",
				operatorAddress: addr1,
				rewardAddress:   addr2,
				ownerAddress:    addr3,
				amount:          big.NewInt(100),
				duration:        10,
				autoStake:       false,
				payload:         signByte,
			},
			address.StakingRegisterCandAddrHash,
			155,
		},
		{
			&CandidateUpdate{
				AbstractAction:  ab,
				name:            "test",
				operatorAddress: addr1,
				rewardAddress:   addr2,
			},
			address.StakingUpdateCandAddrHash,
			92,
		},
	}

	for i, stakingAct := range testCases {
		fmt.Printf("Act %d:\n", i)
		// special addr of staking action
		addr, _ := address.FromBytes(stakingAct.addrHash[:])
		fmt.Printf("special addr = %s\n", addr)

		// convert staking action into native tx
		rlpAct, err := actionToRLP(stakingAct.Action)
		require.NoError(err)
		sig := GetRLPSig(stakingAct.Action, pvk)
		signer := types.NewEIP155Signer(big.NewInt(int64(config.EVMNetworkID())))

		// sign tx
		signedTx, err := reconstructSignedRlpTxFromSig(rlpAct, config.EVMNetworkID(), sig)
		require.NoError(err)
		encoded, err := rlp.EncodeToBytes(signedTx)
		require.NoError(err)
		signedTxHex := hex.EncodeToString(encoded[:])
		rawHash := signer.Hash(signedTx)
		fmt.Printf("rawHash = %x\n", rawHash)
		fmt.Printf("signed TX = %s\n", signedTxHex)

		h, err := rlpSignedHash(rlpAct, config.EVMNetworkID(), sig)
		require.NoError(err)
		fmt.Printf("signed TX hash = %x\n", h)

		// recover public key
		pubk, err := crypto.RecoverPubkey(rawHash[:], sig)
		require.NoError(err)
		require.Equal(pubk.HexString(), pvk.PublicKey().HexString())
		require.True(pvk.PublicKey().Verify(rawHash[:], sig))
		fmt.Printf("publicKey = %s\n", pubk.HexString())
		fmt.Printf("publicKey hash = %s\n\n", hex.EncodeToString(pubk.Hash()))
	}
}
