// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"encoding/hex"
	"math/big"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

func addTestingTsfBlocks(bc blockchain.Blockchain) error {
	tsf0 := action.NewTransfer(1, big.NewInt(100000000), blockchain.Gen.CreatorAddr, ta.Addrinfo["producer"].RawAddress)
	pubk, err := keypair.DecodePublicKey(blockchain.Gen.CreatorPubKey)
	sign, err := hex.DecodeString("847af98bf2c92873f3f7ed02399c7407d0df35c9a45da6947df43638bf2df32d263e59011fab0e4d5380c4c49c579ccd0a25b1260e586f5f379979f38db91ac5f3c7468a6e389d00")
	if err != nil {
		return err
	}
	tsf0.SenderPublicKey = pubk
	tsf0.Signature = sign
	// Add block 1
	// test --> A, B, C, D, E, F
	tsf1 := action.NewTransfer(1, big.NewInt(20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["alfa"].RawAddress)
	tsf1, err = tsf1.Sign(ta.Addrinfo["producer"])
	tsf2 := action.NewTransfer(1, big.NewInt(30), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress)
	tsf2, err = tsf2.Sign(ta.Addrinfo["producer"])
	tsf3 := action.NewTransfer(1, big.NewInt(50), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["charlie"].RawAddress)
	tsf3, err = tsf3.Sign(ta.Addrinfo["producer"])
	tsf4 := action.NewTransfer(1, big.NewInt(70), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["delta"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["producer"])
	tsf5 := action.NewTransfer(1, big.NewInt(110), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["echo"].RawAddress)
	tsf5, err = tsf5.Sign(ta.Addrinfo["producer"])
	tsf6 := action.NewTransfer(1, big.NewInt(50<<20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress)
	tsf6, err = tsf6.Sign(ta.Addrinfo["producer"])

	blk, err := bc.MintNewBlock([]*action.Transfer{tsf0, tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, nil, ta.Addrinfo["producer"], "")
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
	tsf5 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["producer"].RawAddress)
	tsf5, err = tsf5.Sign(ta.Addrinfo["charlie"])
	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5}, nil, ta.Addrinfo["producer"], "")
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
	tsf4 = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["producer"].RawAddress)
	tsf4, err = tsf4.Sign(ta.Addrinfo["delta"])
	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4}, nil, ta.Addrinfo["producer"], "")
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
	tsf6 = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["producer"].RawAddress)
	tsf6, err = tsf6.Sign(ta.Addrinfo["echo"])
	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, nil, ta.Addrinfo["producer"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	return nil
}
