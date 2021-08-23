// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package identityset

import (
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
)

var keyPortfolio = []string{
	"bace9b2435db45b119e1570b4ea9c57993b2311e0c408d743d87cd22838ae892",
	"f964b7ccc40ccace513d3159fa9c30514c4a186ebfdd7c63d69cd79a29b804b0",
	"b437800aab0715903d36f85ea963eb2a0b6e386e7f9345a24354422a3b455757",
	"414efa99dfac6f4095d6954713fb0085268d400d6a05a8ae8a69b5b1c10b4bed",
	"d1acb5110e20becd3f1e2575e5c67f7befac58cd925767601a5f26223dddd1c8",
	"3aa779c846a62a62217f7481b9c3265f1b7fbc8e3217b7dd192d75a65da8a162",
	"c9b58691ee786b92980ab1d273254acaa0b31ab49e39e24b809dd6c36a2c165a",
	"9a3296d4237fd5bd2aacc68c09eea1f6b2c225fff46098597889fec8bd703ac1",
	"5af7498f89772c20917ca0f95671e538d360979447fd1098ec7941f2ded7b563",
	"370d2da29479db621aef14259738d38e59470a46cc3d30962f253851d67fe564",
	"6f221f32adb566b3a04fa0e76a2764eaa1278f890f7321399152695e2b0a5c43",
	"9254d943485d0fb859ff63c5581acc44f00fc2110343ac0445b99dfe39a6f1a5",
	"99d8664a9ddc19d73dff6a6f053f9124dd2ed830a04c3d7f9d1b4ffff57b843d",
	"73c7b4a62bf165dccf8ebdea8278db811efd5b8638e2ed9683d2d94889450426",
	"a4ed7333b1112fee1bdb7b7badb3e86dfeb7e7bebeabb13f96f5c95fdff17b31",
	"499d21e1d2c8a0af8a5462592bcf756d176465071230bd924d2a6286842f5dff",
	"daac551250eec1bd7f5041a1fe4f0c3daa6e26758fc52eeb117c9db6c466eefc",
	"b130cffa3055499c1b09bd53c7c8c3ddc6904be8af9e4d4d9345f68748aee9e2",
	"8ec2825ffa1b6d5144fd5be58a238c679eaf6c1b40643935f63ad073dbb35a78",
	"890aedc449be24c49eb2765b734237d604633aa26d4795355dcebde19812f6db",
	"549565256d4c7076c9baf292bda75483c5a2ee53ecbfdf507ffd909e397a5048",
	"9cdf22c5caa8a4d99eb674da27756b438c05c6b1e8995f4a0586745e2071b115",
	"8c379a71721322d16912a88b1602c5596ca9e99a5f70777561c3029efa71a435",
	"bd8092db5aeb99eae13e3ca2c01088780c60626c9fdc88036707f03974d77183",
	"918d077b170ba8e91cfa6c382dcddf50c6818a4b6b13c57920c707abe9148c07",
	"483edbc578e05dc8c20fbf77b394b252ede7e17107ee9d3d8b2bf9465ea17be9",
	"3489b2fef5fd4a63bc5a46ddab7fcfe9d614b733173e6e99ada07b19063b574e",
	"cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", //producer
	"d3e7252d95ecef433bf152e9878f15e1c5072867399d18226fe7f8668618492c", //alfa
	"a873f9173d456767241f13122d5143b395eeb64694980bee5fb900b689bd98da", //bravo
	"5b0cf587e7817c971f8e4b15a780b0a7d815ef66a6672f9a494a660ba9775e4e", //charlie
	"f0a470f2bdb8471aa59a0a25cde14fc4f7eef96df7880d68ccd24026900b2019", //delta
	"54a17da109b4679d24304ece6718127f7a3a83d921ee0027163b4d950225042d", //echo
	"f2b7c8b45a951c9b924face10bbcd0dc752f8fa524e06bfffb89ad289114c480", //foxtrot
	"5deb4c7bc5d714e1bcde9b43d0a8a268f5bb8692dc7149f434ea0f212b7d52f1", //galilei
}

// Size returns the size of the address
func Size() int {
	return 27 //27 is origin size before add last 8 private key,len(keyPortfolio)
}

// PrivateKey returns the i-th identity's private key
func PrivateKey(i int) crypto.PrivateKey {
	sk, err := crypto.HexStringToPrivateKey(keyPortfolio[i])
	if err != nil {
		log.L().Panic(
			"Error when decoding private key string",
			zap.String("keyStr", keyPortfolio[i]),
			zap.Error(err),
		)
	}
	return sk
}

// Address returns the i-th identity's address
func Address(i int) address.Address {
	sk := PrivateKey(i)
	addr := sk.PublicKey().Address()
	if addr == nil {
		log.L().Panic("Error when constructing the address", zap.Error(errors.New("failed to get address")))
	}
	return addr
}
