// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testaddress

import (
	"encoding/hex"

	"github.com/iotexproject/iotex-core/iotxaddress"
)

const (
	pubkeyA     = "09d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	prikeyA     = "9bb680a519ac1c034231c33f0e2f46816f6263b2ff642808a5f33998f7e412b109d8c6fc6f5cb0a03df112da90486fad7cdece1501aaab658551f8afbe7f59ee"
	pubkeyB     = "247a92febf1daac9c9d1a0f5224bb61eb14b2f291d10e01daa0aa5cf17d3f83a"
	prikeyB     = "aad44df530cb1a52e546b4ecd84ccd63c078887e99c73b6f13f2794b20a4bf15247a92febf1daac9c9d1a0f5224bb61eb14b2f291d10e01daa0aa5cf17d3f83a"
	pubkeyC     = "335664daa5d054e02004c71c28ed4ab4aaec650098fb29e518e23e25274a7273"
	prikeyC     = "a0c0ce98a89f47dede4d380334612a0f155b5d161d4f89256386c9c8b5c92ab9335664daa5d054e02004c71c28ed4ab4aaec650098fb29e518e23e25274a7273"
	pubkeyD     = "7c5ec537514f4d33b7d99bf3f7a1c94c698db6121e322b54e073ab123212ca03"
	prikeyD     = "3ade330ac95c22517ea23bc37f8f980e2e63796af2124435ba0f862627cb579f7c5ec537514f4d33b7d99bf3f7a1c94c698db6121e322b54e073ab123212ca03"
	pubkeyE     = "705c9a0517381e060e148e65955c54ea8a1cf7c8b4ad47e2df1e14d779e6acc6"
	prikeyE     = "0ba67d6be4700b8825708b95d2489cb3150836794e1893278d1df7357677a161705c9a0517381e060e148e65955c54ea8a1cf7c8b4ad47e2df1e14d779e6acc6"
	pubkeyF     = "78214053ae1171cccf40e564f1d178e9356072b4e69b81d0f234472d4fc08304"
	prikeyF     = "73df37d072a90da42589546cae50558b732d8a0f9105b3db135da97540f93c8478214053ae1171cccf40e564f1d178e9356072b4e69b81d0f234472d4fc08304"
	pubkeyG     = "c9a3b81ce7ea795890b116be65d38ee80c5fa879ca56829291882ffc9f00cb5d"
	prikeyG     = "324819666a70f84c6ed5b352494775ab8eeb5e125faef96822326039576a06afc9a3b81ce7ea795890b116be65d38ee80c5fa879ca56829291882ffc9f00cb5d"
	pubkeyMiner = "b9b8d7316705dc4ff62bb323e610f3f5072abedc9834e999d6537f6681284ea2"
	prikeyMiner = "7fbb20b87d34eade61351165aa4c6fa5d87dd349368dd6b9034ea3d3e918c706b9b8d7316705dc4ff62bb323e610f3f5072abedc9834e999d6537f6681284ea2"
)

// Addrinfo contains the address information
var Addrinfo map[string]*iotxaddress.Address

func constructAddress(pubkey, prikey string) *iotxaddress.Address {
	pubk, err := hex.DecodeString(pubkey)
	if err != nil {
		panic(err)
	}
	prik, err := hex.DecodeString(prikey)
	if err != nil {
		panic(err)
	}
	addr, err := iotxaddress.GetAddress(pubk, false, []byte{0x01, 0x02, 0x03, 0x04})
	if err != nil {
		panic(err)
	}
	addr.PrivateKey = prik
	return addr
}

func init() {
	Addrinfo = make(map[string]*iotxaddress.Address)

	Addrinfo["miner"] = constructAddress(pubkeyMiner, prikeyMiner)
	Addrinfo["alfa"] = constructAddress(pubkeyA, prikeyA)
	Addrinfo["bravo"] = constructAddress(pubkeyB, prikeyB)
	Addrinfo["charlie"] = constructAddress(pubkeyC, prikeyC)
	Addrinfo["delta"] = constructAddress(pubkeyD, prikeyD)
	Addrinfo["echo"] = constructAddress(pubkeyE, prikeyE)
	Addrinfo["foxtrot"] = constructAddress(pubkeyF, prikeyF)
	Addrinfo["galilei"] = constructAddress(pubkeyG, prikeyG)
}
