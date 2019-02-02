// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"encoding/hex"
	"io/ioutil"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func addTestingTsfBlocks(bc blockchain.Blockchain) error {
	// Add block 1
	tsf0, _ := action.NewTransfer(
		1,
		blockchain.ConvertIotxToRau(3000000000),
		blockchain.Gen.CreatorAddr(),
		ta.Addrinfo["producer"].Bech32(),
		[]byte{}, uint64(100000),
		big.NewInt(0),
	)
	pubk, _ := keypair.DecodePublicKey(blockchain.Gen.CreatorPubKey)
	sig, _ := hex.DecodeString("da334834c0169a28d9e85035ca7b51df17ec03310bde7902be32d311d7233fe259f49af86330697a4d2d68b74a1d3219a0db003a31c6416b4c86b5fcbebfd8c800")
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf0).
		SetDestinationAddress(ta.Addrinfo["producer"].Bech32()).
		SetNonce(1).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()

	selp := action.AssembleSealedEnvelope(elp, blockchain.Gen.CreatorAddr(), pubk, sig)

	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[selp.SrcAddr()] = []action.SealedEnvelope{selp}
	blk, err := bc.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(),
		0,
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	addr0 := ta.Addrinfo["producer"].Bech32()
	priKey0 := ta.Keyinfo["producer"].PriKey
	addr1 := ta.Addrinfo["alfa"].Bech32()
	addr2 := ta.Addrinfo["bravo"].Bech32()
	addr3 := ta.Addrinfo["charlie"].Bech32()
	priKey3 := ta.Keyinfo["charlie"].PriKey
	addr4 := ta.Addrinfo["delta"].Bech32()
	priKey4 := ta.Keyinfo["delta"].PriKey
	addr5 := ta.Addrinfo["echo"].Bech32()
	priKey5 := ta.Keyinfo["echo"].PriKey
	addr6 := ta.Addrinfo["foxtrot"].Bech32()
	// Add block 2
	// test --> A, B, C, D, E, F
	tsf1, err := testutil.SignedTransfer(addr0, addr1, priKey0, 1, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err := testutil.SignedTransfer(addr0, addr2, priKey0, 2, big.NewInt(30), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err := testutil.SignedTransfer(addr0, addr3, priKey0, 3, big.NewInt(50), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err := testutil.SignedTransfer(addr0, addr4, priKey0, 4, big.NewInt(70), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf5, err := testutil.SignedTransfer(addr0, addr5, priKey0, 5, big.NewInt(110), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf6, err := testutil.SignedTransfer(addr0, addr6, priKey0, 6, big.NewInt(5<<20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[tsf1.SrcAddr()] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}
	blk, err = bc.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(),
		0,
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	// Charlie --> A, B, D, E, test
	tsf1, err = testutil.SignedTransfer(addr3, addr1, priKey3, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr3, addr2, priKey3, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr3, addr4, priKey3, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr3, addr5, priKey3, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf5, err = testutil.SignedTransfer(addr3, addr0, priKey3, 5, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[tsf1.SrcAddr()] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5}
	blk, err = bc.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(),
		0,
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	// Delta --> B, E, F, test
	tsf1, err = testutil.SignedTransfer(addr4, addr2, priKey4, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr4, addr5, priKey4, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr4, addr6, priKey4, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr4, addr0, priKey4, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[tsf1.SrcAddr()] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4}
	blk, err = bc.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(),
		0,
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 5
	// Delta --> A, B, C, D, F, test
	tsf1, err = testutil.SignedTransfer(addr5, addr1, priKey5, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf2, err = testutil.SignedTransfer(addr5, addr2, priKey5, 2, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf3, err = testutil.SignedTransfer(addr5, addr3, priKey5, 3, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf4, err = testutil.SignedTransfer(addr5, addr4, priKey5, 4, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf5, err = testutil.SignedTransfer(addr5, addr6, priKey5, 5, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}
	tsf6, err = testutil.SignedTransfer(addr5, addr0, priKey5, 6, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPrice))
	if err != nil {
		return err
	}

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[tsf1.SrcAddr()] = []action.SealedEnvelope{tsf1, tsf2, tsf3, tsf4, tsf5, tsf6}
	blk, err = bc.MintNewBlock(
		actionMap,
		ta.Keyinfo["producer"].PubKey,
		ta.Keyinfo["producer"].PriKey,
		ta.Addrinfo["producer"].Bech32(),
		0,
	)
	if err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
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
