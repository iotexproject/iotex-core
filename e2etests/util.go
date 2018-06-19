// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etests

import (
	"encoding/hex"
	"math/big"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

func addTestingTsfBlocks(bc blockchain.Blockchain) error {
	tsf0 := action.NewTransfer(1, big.NewInt(int64(blockchain.Gen.TotalSupply)), blockchain.Gen.Creator, ta.Addrinfo["miner"].RawAddress)
	pubk, err := hex.DecodeString("d01164c3afe47406728d3e17861a3251dcff39e62bdc2b93ccb69a02785a175e195b5605517fd647eb7dd095b3d862dffb087f35eacf10c6859d04a100dbfb7358eeca9d5c37c904")
	sign, err := hex.DecodeString("492cf06f6bc77b248e236e09cde00fab433fac6d2d161d1a2b02176f5ed78a3aea4c0d002989f95722c9e282e2627aa49541d8e3e9c07bbdbb6574a907683074798fc9fad359f301")
	if err != nil {
		return err
	}
	tsf0.SenderPublicKey = pubk
	tsf0.Signature = sign
	// Add block 1
	// test --> A, B, C, D, E, F
	tsf1 := action.NewTransfer(1, big.NewInt(20), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, err = tsf1.Sign(ta.Addrinfo["miner"])
	tsf2 := action.NewTransfer(1, big.NewInt(30), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["miner"])
	tsf3 := action.NewTransfer(1, big.NewInt(50), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["charlie"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["miner"])
	tsf4 := action.NewTransfer(1, big.NewInt(70), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["delta"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["miner"])
	tsf5 := action.NewTransfer(1, big.NewInt(110), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["echo"].RawAddress)
	tsf5, err = tsf5.Sign(ta.Addrinfo["miner"])
	tsf6 := action.NewTransfer(1, big.NewInt(50<<20), ta.Addrinfo["miner"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress)
	tsf6, err = tsf6.Sign(ta.Addrinfo["miner"])

	blk, err := bc.MintNewBlock([]*action.Transfer{tsf0, tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 2
	// Charlie --> A, B, D, E, test
	tsf1 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, err = tsf1.Sign(ta.Addrinfo["charlie"])
	tsf2 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["charlie"])
	tsf3 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["delta"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["charlie"])
	tsf4 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["echo"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["charlie"])
	tsf5 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["miner"].RawAddress)
	tsf5, err = tsf5.Sign(ta.Addrinfo["charlie"])
	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5}, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	// Delta --> B, E, F, test
	tsf1 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf1, err = tsf1.Sign(ta.Addrinfo["delta"])
	tsf2 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["echo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["delta"])
	tsf3 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["delta"])
	tsf4 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["miner"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["delta"])
	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4}, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	// Delta --> A, B, C, D, F, test
	tsf1 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, err = tsf1.Sign(ta.Addrinfo["echo"])
	tsf2 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["echo"])
	tsf3 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["charlie"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["echo"])
	tsf4 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["delta"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["echo"])
	tsf5 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress)
	tsf5, err = tsf5.Sign(ta.Addrinfo["echo"])
	tsf6 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["miner"].RawAddress)
	tsf6, err = tsf6.Sign(ta.Addrinfo["echo"])
	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, nil, ta.Addrinfo["miner"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	return nil
}
