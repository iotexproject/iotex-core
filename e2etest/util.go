// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"encoding/hex"
	"math/big"
	"os"
	"path/filepath"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func addTestingTsfBlocks(bc blockchain.Blockchain, ap actpool.ActPool) error {
	ctx := context.Background()
	addOneTx := func(tx action.SealedEnvelope, err error) error {
		if err != nil {
			return err
		}
		if err := ap.Add(ctx, tx); err != nil {
			return err
		}
		return nil
	}
	// Add block 1
	// deploy simple smart contract
	sk0 := identityset.PrivateKey(0)
	data, _ := hex.DecodeString("608060405234801561001057600080fd5b50610233806100206000396000f300608060405260043610610057576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680635bec9e671461005c57806360fe47b114610073578063c2bc2efc146100a0575b600080fd5b34801561006857600080fd5b506100716100f7565b005b34801561007f57600080fd5b5061009e60048036038101908080359060200190929190505050610143565b005b3480156100ac57600080fd5b506100e1600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061017a565b6040518082815260200191505060405180910390f35b5b6001156101155760008081548092919060010191905055506100f8565b7f8bfaa460932ccf8751604dd60efa3eafa220ec358fccb32ef703f91c509bc3ea60405160405180910390a1565b80600081905550807fdf7a95aebff315db1b7716215d602ab537373cdb769232aae6055c06e798425b60405160405180910390a250565b60008073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff16141515156101b757600080fd5b6000548273ffffffffffffffffffffffffffffffffffffffff167fbde7a70c2261170a87678200113c8e12f82f63d0a1d1cfa45681cbac328e87e360405160405180910390a360005490509190505600a165627a7a723058203198d0390613dab2dff2fa053c1865e802618d628429b01ab05b8458afc347eb0029")
	if err := addOneTx(action.SignedExecution(action.EmptyAddress, sk0, 1, big.NewInt(0), 200000, big.NewInt(testutil.TestGasPriceInt64), data)); err != nil {
		return err
	}
	if err := addOneTx(action.SignedTransfer(identityset.Address(27).String(), sk0, 2, unit.ConvertIotxToRau(90000000), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	addr0 := identityset.Address(27).String()
	priKey0 := identityset.PrivateKey(27)
	addr1 := identityset.Address(28).String()
	addr2 := identityset.Address(29).String()
	addr3 := identityset.Address(30).String()
	priKey3 := identityset.PrivateKey(30)
	addr4 := identityset.Address(31).String()
	priKey4 := identityset.PrivateKey(31)
	addr5 := identityset.Address(32).String()
	priKey5 := identityset.PrivateKey(32)
	addr6 := identityset.Address(33).String()
	// Add block 2
	// test --> A, B, C, D, E, F
	if err = addOneTx(action.SignedTransfer(addr1, priKey0, 1, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr2, priKey0, 2, big.NewInt(30), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr3, priKey0, 3, big.NewInt(50), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr4, priKey0, 4, big.NewInt(70), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr5, priKey0, 5, big.NewInt(110), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr6, priKey0, 6, big.NewInt(5<<20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	// call set()
	data, _ = hex.DecodeString("60fe47b1")
	data = append(data, hash.ZeroHash256[:]...)
	if err := addOneTx(action.SignedExecution("io18vlvlj0v02yye70kpqtzhu4uek3qqz27zm7g42", sk0, 3, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data)); err != nil {
		return err
	}
	blk, err = bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	// Charlie --> A, B, D, E, test
	if err = addOneTx(action.SignedTransfer(addr1, priKey3, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr2, priKey3, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr4, priKey3, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr5, priKey3, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr0, priKey3, 5, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	// call get()
	getData, _ := hex.DecodeString("c2bc2efc")
	getData = append(getData, hash.ZeroHash256[:]...)
	if err = addOneTx(action.SignedExecution("io18vlvlj0v02yye70kpqtzhu4uek3qqz27zm7g42", sk0, 4, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), getData)); err != nil {
		return err
	}
	// call set()
	if err = addOneTx(action.SignedExecution("io18vlvlj0v02yye70kpqtzhu4uek3qqz27zm7g42", sk0, 5, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data)); err != nil {
		return err
	}
	blk, err = bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	// Delta --> B, E, F, test
	if err = addOneTx(action.SignedTransfer(addr2, priKey4, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr5, priKey4, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr6, priKey4, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr0, priKey4, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	// call get()
	if err = addOneTx(action.SignedExecution("io18vlvlj0v02yye70kpqtzhu4uek3qqz27zm7g42", sk0, 6, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), getData)); err != nil {
		return err
	}
	// call set()
	if err = addOneTx(action.SignedExecution("io18vlvlj0v02yye70kpqtzhu4uek3qqz27zm7g42", sk0, 7, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data)); err != nil {
		return err
	}
	// call get()
	if err = addOneTx(action.SignedExecution("io18vlvlj0v02yye70kpqtzhu4uek3qqz27zm7g42", sk0, 8, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), getData)); err != nil {
		return err
	}
	blk, err = bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 5
	// Delta --> A, B, C, D, F, test
	if err = addOneTx(action.SignedTransfer(addr1, priKey5, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr2, priKey5, 2, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr3, priKey5, 3, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr4, priKey5, 4, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr6, priKey5, 5, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	if err = addOneTx(action.SignedTransfer(addr0, priKey5, 6, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))); err != nil {
		return err
	}
	blk, err = bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}

func copyDB(srcDB, dstDB string) error {
	input, err := os.ReadFile(filepath.Clean(srcDB))
	if err != nil {
		return errors.Wrap(err, "failed to read source db file")
	}
	if err := os.WriteFile(dstDB, input, 0644); err != nil {
		return errors.Wrap(err, "failed to copy db file")
	}
	return nil
}
