// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testaddress

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

const (
	prikeyA        = "d3e7252d95ecef433bf152e9878f15e1c5072867399d18226fe7f8668618492c"
	prikeyB        = "a873f9173d456767241f13122d5143b395eeb64694980bee5fb900b689bd98da"
	prikeyC        = "5b0cf587e7817c971f8e4b15a780b0a7d815ef66a6672f9a494a660ba9775e4e"
	prikeyD        = "f0a470f2bdb8471aa59a0a25cde14fc4f7eef96df7880d68ccd24026900b2019"
	prikeyE        = "54a17da109b4679d24304ece6718127f7a3a83d921ee0027163b4d950225042d"
	prikeyF        = "f2b7c8b45a951c9b924face10bbcd0dc752f8fa524e06bfffb89ad289114c480"
	prikeyG        = "5deb4c7bc5d714e1bcde9b43d0a8a268f5bb8692dc7149f434ea0f212b7d52f1"
	prikeyProducer = "cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1"
)

// Addrinfo contains the address information
var Addrinfo map[string]*address.AddrV1

// Keyinfo contains the private key information
var Keyinfo map[string]*ecdsa.PrivateKey

func init() {
	Addrinfo = make(map[string]*address.AddrV1)
	Keyinfo = make(map[string]*ecdsa.PrivateKey)

	priKey, _ := keypair.DecodePrivateKey(prikeyProducer)
	Addrinfo["producer"] = address.V1.New(keypair.HashPubKey(&priKey.PublicKey))
	fmt.Printf("Producer's address is %s\n", Addrinfo["producer"].Bech32())
	Keyinfo["producer"] = priKey

	priKey, _ = keypair.DecodePrivateKey(prikeyA)
	Addrinfo["alfa"] = address.V1.New(keypair.HashPubKey(&priKey.PublicKey))
	fmt.Printf("Alfa's address is %s\n", Addrinfo["alfa"].Bech32())
	Keyinfo["alfa"] = priKey

	priKey, _ = keypair.DecodePrivateKey(prikeyB)
	Addrinfo["bravo"] = address.V1.New(keypair.HashPubKey(&priKey.PublicKey))
	fmt.Printf("Bravo's address is %s\n", Addrinfo["bravo"].Bech32())
	Keyinfo["bravo"] = priKey

	priKey, _ = keypair.DecodePrivateKey(prikeyC)
	Addrinfo["charlie"] = address.V1.New(keypair.HashPubKey(&priKey.PublicKey))
	Keyinfo["charlie"] = priKey

	priKey, _ = keypair.DecodePrivateKey(prikeyD)
	Addrinfo["delta"] = address.V1.New(keypair.HashPubKey(&priKey.PublicKey))
	Keyinfo["delta"] = priKey

	priKey, _ = keypair.DecodePrivateKey(prikeyE)
	Addrinfo["echo"] = address.V1.New(keypair.HashPubKey(&priKey.PublicKey))
	Keyinfo["echo"] = priKey

	priKey, _ = keypair.DecodePrivateKey(prikeyF)
	Addrinfo["foxtrot"] = address.V1.New(keypair.HashPubKey(&priKey.PublicKey))
	Keyinfo["foxtrot"] = priKey

	priKey, _ = keypair.DecodePrivateKey(prikeyG)
	Addrinfo["galilei"] = address.V1.New(keypair.HashPubKey(&priKey.PublicKey))
	Keyinfo["galilei"] = priKey
}
