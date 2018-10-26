// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"math/big"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

func addTestingTsfBlocks(bc blockchain.Blockchain) error {
	// Add block 1
	tsf0, _ := action.NewTransfer(
		1,
		blockchain.ConvertIotxToRau(3000000000),
		blockchain.Gen.CreatorAddr(config.Default.Chain.ID),
		ta.Addrinfo["producer"].RawAddress,
		[]byte{}, uint64(100000),
		big.NewInt(10),
	)
	sk, err := keypair.DecodePrivateKey(blockchain.Gen.CreatorPrivKey)
	if err != nil {
		return err
	}
	action.Sign(tsf0, sk)
	blk, err := bc.MintNewBlock([]action.Action{tsf0}, ta.Addrinfo["producer"], nil, nil, "")
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	// Add block 2
	// test --> A, B, C, D, E, F
	tsf1, _ := action.NewTransfer(1, big.NewInt(20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf1, ta.Addrinfo["producer"].PrivateKey)
	tsf2, _ := action.NewTransfer(2, big.NewInt(30), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf2, ta.Addrinfo["producer"].PrivateKey)
	tsf3, _ := action.NewTransfer(3, big.NewInt(50), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["charlie"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf3, ta.Addrinfo["producer"].PrivateKey)
	tsf4, _ := action.NewTransfer(4, big.NewInt(70), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf4, ta.Addrinfo["producer"].PrivateKey)
	tsf5, _ := action.NewTransfer(5, big.NewInt(110), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["echo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf5, ta.Addrinfo["producer"].PrivateKey)
	tsf6, _ := action.NewTransfer(6, big.NewInt(5<<20), ta.Addrinfo["producer"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf6, ta.Addrinfo["producer"].PrivateKey)

	blk, err = bc.MintNewBlock([]action.Action{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, ta.Addrinfo["producer"], nil, nil, "")
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	// Charlie --> A, B, D, E, test
	tsf1, _ = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf1, ta.Addrinfo["charlie"].PrivateKey)
	tsf2, _ = action.NewTransfer(2, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf2, ta.Addrinfo["charlie"].PrivateKey)
	tsf3, _ = action.NewTransfer(3, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf3, ta.Addrinfo["charlie"].PrivateKey)
	tsf4, _ = action.NewTransfer(4, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["echo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf4, ta.Addrinfo["charlie"].PrivateKey)
	tsf5, _ = action.NewTransfer(5, big.NewInt(1), ta.Addrinfo["charlie"].RawAddress, ta.Addrinfo["producer"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf5, ta.Addrinfo["charlie"].PrivateKey)
	blk, err = bc.MintNewBlock([]action.Action{tsf1, tsf2, tsf3, tsf4, tsf5}, ta.Addrinfo["producer"], nil, nil, "")
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	// Delta --> B, E, F, test
	tsf1, _ = action.NewTransfer(1, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf1, ta.Addrinfo["delta"].PrivateKey)
	tsf2, _ = action.NewTransfer(2, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["echo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf2, ta.Addrinfo["delta"].PrivateKey)
	tsf3, _ = action.NewTransfer(3, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf3, ta.Addrinfo["delta"].PrivateKey)
	tsf4, _ = action.NewTransfer(4, big.NewInt(1), ta.Addrinfo["delta"].RawAddress, ta.Addrinfo["producer"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf4, ta.Addrinfo["delta"].PrivateKey)
	blk, err = bc.MintNewBlock([]action.Action{tsf1, tsf2, tsf3, tsf4}, ta.Addrinfo["producer"], nil, nil, "")
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 5
	// Delta --> A, B, C, D, F, test
	tsf1, _ = action.NewTransfer(1, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["alfa"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf1, ta.Addrinfo["echo"].PrivateKey)
	tsf2, _ = action.NewTransfer(2, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["bravo"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf2, ta.Addrinfo["echo"].PrivateKey)
	tsf3, _ = action.NewTransfer(3, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["charlie"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf3, ta.Addrinfo["echo"].PrivateKey)
	tsf4, _ = action.NewTransfer(4, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["delta"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf4, ta.Addrinfo["echo"].PrivateKey)
	tsf5, _ = action.NewTransfer(5, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["foxtrot"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf5, ta.Addrinfo["echo"].PrivateKey)
	tsf6, _ = action.NewTransfer(6, big.NewInt(2), ta.Addrinfo["echo"].RawAddress, ta.Addrinfo["producer"].RawAddress, []byte{}, uint64(100000), big.NewInt(10))
	_ = action.Sign(tsf6, ta.Addrinfo["echo"].PrivateKey)
	blk, err = bc.MintNewBlock([]action.Action{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, ta.Addrinfo["producer"], nil, nil, "")
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}
