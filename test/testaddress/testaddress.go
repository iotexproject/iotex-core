// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testaddress

import (
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

// Key indicates the public key and private key of an account
type Key struct {
	PubKey keypair.PublicKey
	PriKey keypair.PrivateKey
}

// Addrinfo contains the address information
var Addrinfo map[string]*address.AddrV1

// Keyinfo contains the private key information
var Keyinfo map[string]*Key

func init() {
	Addrinfo = make(map[string]*address.AddrV1)
	Keyinfo = make(map[string]*Key)

	priKey, _ := keypair.DecodePrivateKey(prikeyProducer)
	pubKey := &priKey.PublicKey
	Addrinfo["producer"] = address.V1.New(keypair.HashPubKey(pubKey))
	fmt.Printf("Producer's address is %s\n", Addrinfo["producer"].Bech32())
	Keyinfo["producer"] = &Key{PubKey: pubKey, PriKey: priKey}

	priKey, _ = keypair.DecodePrivateKey(prikeyA)
	pubKey = &priKey.PublicKey
	Addrinfo["alfa"] = address.V1.New(keypair.HashPubKey(pubKey))
	fmt.Printf("Alfa's address is %s\n", Addrinfo["alfa"].Bech32())
	Keyinfo["alfa"] = &Key{PubKey: pubKey, PriKey: priKey}

	priKey, _ = keypair.DecodePrivateKey(prikeyB)
	pubKey = &priKey.PublicKey
	Addrinfo["bravo"] = address.V1.New(keypair.HashPubKey(pubKey))
	fmt.Printf("Bravo's address is %s\n", Addrinfo["bravo"].Bech32())
	Keyinfo["bravo"] = &Key{PubKey: pubKey, PriKey: priKey}

	priKey, _ = keypair.DecodePrivateKey(prikeyC)
	pubKey = &priKey.PublicKey
	Addrinfo["charlie"] = address.V1.New(keypair.HashPubKey(pubKey))
	Keyinfo["charlie"] = &Key{PubKey: pubKey, PriKey: priKey}

	priKey, _ = keypair.DecodePrivateKey(prikeyD)
	pubKey = &priKey.PublicKey
	Addrinfo["delta"] = address.V1.New(keypair.HashPubKey(pubKey))
	Keyinfo["delta"] = &Key{PubKey: pubKey, PriKey: priKey}

	priKey, _ = keypair.DecodePrivateKey(prikeyE)
	pubKey = &priKey.PublicKey
	Addrinfo["echo"] = address.V1.New(keypair.HashPubKey(pubKey))
	Keyinfo["echo"] = &Key{PubKey: pubKey, PriKey: priKey}

	priKey, _ = keypair.DecodePrivateKey(prikeyF)
	pubKey = &priKey.PublicKey
	Addrinfo["foxtrot"] = address.V1.New(keypair.HashPubKey(pubKey))
	Keyinfo["foxtrot"] = &Key{PubKey: pubKey, PriKey: priKey}

	priKey, _ = keypair.DecodePrivateKey(prikeyG)
	pubKey = &priKey.PublicKey
	Addrinfo["galilei"] = address.V1.New(keypair.HashPubKey(pubKey))
	Keyinfo["galilei"] = &Key{PubKey: pubKey, PriKey: priKey}
}
