// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"encoding/hex"
	"math/big"
	"io/ioutil"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func addTestingTsfBlocks(bc blockchain.Blockchain) error {
	// Add block 1
	tsf0, _ := action.NewTransfer(
		1,
		blockchain.ConvertIotxToRau(3000000000),
		blockchain.Gen.CreatorAddr(config.Default.Chain.ID),
		ta.Addrinfo["producer"].RawAddress,
		[]byte{}, uint64(100000),
		big.NewInt(0),
	)
	pk, _ := hex.DecodeString(blockchain.Gen.CreatorPubKey)
	pubk, _ := keypair.BytesToPublicKey(pk)
	sig, _ := hex.DecodeString("b60242721edc706ff81369ad4be4002b70cdc81d7465ba1a1669369afc7696b7f8cc9b0085eee1092ffdb2ccaaed89943cd2dfb65f3cc269735fa4207d38d8e03e548cff4116b900")
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf0).
		SetDestinationAddress(ta.Addrinfo["producer"].RawAddress).
		SetNonce(1).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()
	selp := action.AssembleSealedEnvelope(elp, blockchain.Gen.CreatorAddr(config.Default.Chain.ID), pubk, sig)

	blk, err := bc.MintNewBlock([]action.SealedEnvelope{selp}, ta.Addrinfo["producer"], nil, nil, "")
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
	tsf1, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["alfa"], 1, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["bravo"], 2, big.NewInt(30), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["charlie"], 3, big.NewInt(50), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["delta"], 4, big.NewInt(70), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf5, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["echo"], 5, big.NewInt(110), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf6, err := testutil.SignedTransfer(ta.Addrinfo["producer"], ta.Addrinfo["foxtrot"], 6, big.NewInt(5<<20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}

	blk, err = bc.MintNewBlock([]action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, ta.Addrinfo["producer"], nil, nil, "")
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
	tsf1, err = testutil.SignedTransfer(ta.Addrinfo["charlie"], ta.Addrinfo["alfa"], 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(ta.Addrinfo["charlie"], ta.Addrinfo["bravo"], 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(ta.Addrinfo["charlie"], ta.Addrinfo["delta"], 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(ta.Addrinfo["charlie"], ta.Addrinfo["echo"], 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf5, err = testutil.SignedTransfer(ta.Addrinfo["charlie"], ta.Addrinfo["producer"], 5, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	blk, err = bc.MintNewBlock([]action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5}, ta.Addrinfo["producer"], nil, nil, "")
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
	tsf1, err = testutil.SignedTransfer(ta.Addrinfo["delta"], ta.Addrinfo["bravo"], 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(ta.Addrinfo["delta"], ta.Addrinfo["echo"], 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(ta.Addrinfo["delta"], ta.Addrinfo["foxtrot"], 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(ta.Addrinfo["delta"], ta.Addrinfo["producer"], 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	blk, err = bc.MintNewBlock([]action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4}, ta.Addrinfo["producer"], nil, nil, "")
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
	tsf1, err = testutil.SignedTransfer(ta.Addrinfo["echo"], ta.Addrinfo["alfa"], 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(ta.Addrinfo["echo"], ta.Addrinfo["bravo"], 2, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(ta.Addrinfo["echo"], ta.Addrinfo["charlie"], 3, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(ta.Addrinfo["echo"], ta.Addrinfo["delta"], 4, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf5, err = testutil.SignedTransfer(ta.Addrinfo["echo"], ta.Addrinfo["foxtrot"], 5, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf6, err = testutil.SignedTransfer(ta.Addrinfo["echo"], ta.Addrinfo["producer"], 6, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	blk, err = bc.MintNewBlock([]action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}, ta.Addrinfo["producer"], nil, nil, "")
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk, true); err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}

func copyDB(srcDB, dstDB string) error {
	input, err := ioutil.ReadFile(srcDB)
	if err != nil {
		return errors.Wrap(err, "failed to read source db file")
	}
	if err := ioutil.WriteFile(dstDB, input, 0644); err != nil {
		return errors.Wrap(err, "failed to copy db file")
	}
	return nil
}