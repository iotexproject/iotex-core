// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testaddress

import (
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

var (
	testSKs = map[string]string{
		"producer": "cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1",
		"alfa":     "d3e7252d95ecef433bf152e9878f15e1c5072867399d18226fe7f8668618492c",
		"bravo":    "a873f9173d456767241f13122d5143b395eeb64694980bee5fb900b689bd98da",
		"charlie":  "5b0cf587e7817c971f8e4b15a780b0a7d815ef66a6672f9a494a660ba9775e4e",
		"delta":    "f0a470f2bdb8471aa59a0a25cde14fc4f7eef96df7880d68ccd24026900b2019",
		"echo":     "54a17da109b4679d24304ece6718127f7a3a83d921ee0027163b4d950225042d",
		"foxtrot":  "f2b7c8b45a951c9b924face10bbcd0dc752f8fa524e06bfffb89ad289114c480",
		"galilei":  "5deb4c7bc5d714e1bcde9b43d0a8a268f5bb8692dc7149f434ea0f212b7d52f1",
	}
)

// Key indicates the public key and private key of an account
type Key struct {
	PubKey keypair.PublicKey
	PriKey keypair.PrivateKey
}

// Addrinfo contains the address information
var Addrinfo map[string]address.Address

// Keyinfo contains the private key information
var Keyinfo map[string]*Key

func init() {
	Addrinfo = make(map[string]address.Address)
	Keyinfo = make(map[string]*Key)
	for name, skStr := range testSKs {
		priKey, _ := keypair.HexStringToPrivateKey(skStr)
		pubKey := priKey.PublicKey()
		Addrinfo[name], _ = address.FromBytes(pubKey.Hash())
		Keyinfo[name] = &Key{PubKey: pubKey, PriKey: priKey}
	}
}
