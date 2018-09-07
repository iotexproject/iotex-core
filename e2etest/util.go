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
	// Add block 1
	tsf0, _ := action.NewTransfer(1, big.NewInt(3000000000), blockchain.Gen.CreatorAddr, ta.Addrinfo["producer"].RawAddress, []byte{}, uint64(100000), uint64(10))
	pubk, _ := keypair.DecodePublicKey(blockchain.Gen.CreatorPubKey)
	sign, err := hex.DecodeString("d977ab7dc2d878cd22d47601058a1ba60aecc8931a2054885ad09d07a183418813d28000fe7d86793c0eb826d1b4c37fad9bc22ecd9ebd5a1e03ce34a9d51a8e105543429079f001")
	if err != nil {
		return err
	}
	tsf0.SenderPublicKey = pubk
	tsf0.Signature = sign
	blk, err := bc.MintNewBlock([]*action.Transfer{tsf0}, nil, nil, ta.Addrinfo["producer"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	// Add block 2
	// test --> A, B, C, D, E, F
	tsf1, _ := action.NewTransfer(1, big.NewInt(20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf1, _ = tsf1.Sign(ta.Addrinfo["producer"])
	tsf2, _ := action.NewTransfer(2, big.NewInt(30), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf2, _ = tsf2.Sign(ta.Addrinfo["producer"])
	tsf3, _ := action.NewTransfer(3, big.NewInt(50), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["charlie"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf3, _ = tsf3.Sign(ta.Addrinfo["producer"])
	tsf4, _ := action.NewTransfer(4, big.NewInt(70), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf4, _ = tsf4.Sign(ta.Addrinfo["producer"])
	tsf5, _ := action.NewTransfer(5, big.NewInt(110), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["echo"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf5, _ = tsf5.Sign(ta.Addrinfo["producer"])
	tsf6, _ := action.NewTransfer(6, big.NewInt(5<<20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf6, _ = tsf6.Sign(ta.Addrinfo["producer"])

	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, nil, nil, ta.Addrinfo["producer"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	// Charlie --> A, B, D, E, test
	tsf1, _ = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf1, _ = tsf1.Sign(ta.Addrinfo["charlie"])
	tsf2, _ = action.NewTransfer(2, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf2, _ = tsf2.Sign(ta.Addrinfo["charlie"])
	tsf3, _ = action.NewTransfer(3, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf3, _ = tsf3.Sign(ta.Addrinfo["charlie"])
	tsf4, _ = action.NewTransfer(4, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["echo"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf4, _ = tsf4.Sign(ta.Addrinfo["charlie"])
	tsf5, _ = action.NewTransfer(5, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["producer"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf5, _ = tsf5.Sign(ta.Addrinfo["charlie"])
	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5}, nil, nil, ta.Addrinfo["producer"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	// Delta --> B, E, F, test
	tsf1, _ = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf1, _ = tsf1.Sign(ta.Addrinfo["delta"])
	tsf2, _ = action.NewTransfer(2, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["echo"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf2, _ = tsf2.Sign(ta.Addrinfo["delta"])
	tsf3, _ = action.NewTransfer(3, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf3, _ = tsf3.Sign(ta.Addrinfo["delta"])
	tsf4, _ = action.NewTransfer(4, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["producer"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf4, _ = tsf4.Sign(ta.Addrinfo["delta"])
	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4}, nil, nil, ta.Addrinfo["producer"], "")
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 5
	// Delta --> A, B, C, D, F, test
	tsf1, _ = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf1, _ = tsf1.Sign(ta.Addrinfo["echo"])
	tsf2, _ = action.NewTransfer(2, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf2, _ = tsf2.Sign(ta.Addrinfo["echo"])
	tsf3, _ = action.NewTransfer(3, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["charlie"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf3, _ = tsf3.Sign(ta.Addrinfo["echo"])
	tsf4, _ = action.NewTransfer(4, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf4, _ = tsf4.Sign(ta.Addrinfo["echo"])
	tsf5, _ = action.NewTransfer(5, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf5, _ = tsf5.Sign(ta.Addrinfo["echo"])
	tsf6, _ = action.NewTransfer(6, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["producer"].RawAddress, []byte{}, uint64(100000), uint64(10))
	tsf6, _ = tsf6.Sign(ta.Addrinfo["echo"])
	blk, err = bc.MintNewBlock([]*action.Transfer{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, nil, nil, ta.Addrinfo["producer"], "")
	if err != nil {
		return err
	}

	return bc.CommitBlock(blk)
}

func addTestingDummyBlock(bc blockchain.Blockchain) error {
	// Add block 1
	if err := bc.CommitBlock(bc.MintNewDummyBlock()); err != nil {
		return err
	}
	// Add block 2
	return bc.CommitBlock(bc.MintNewDummyBlock())
}
