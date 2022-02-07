// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"math/big"
	"os"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func addTestingTsfBlocks(bc blockchain.Blockchain, ap actpool.ActPool) error {
	// Add block 1
	tsf0, _ := action.NewTransfer(
		1,
		unit.ConvertIotxToRau(90000000),
		identityset.Address(27).String(),
		[]byte{}, uint64(100000),
		big.NewInt(0),
	)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(tsf0).
		SetNonce(1).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()
	selp, err := action.Sign(elp, identityset.PrivateKey(0))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), selp); err != nil {
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
	tsf1, err := action.SignedTransfer(addr1, priKey0, 1, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf1); err != nil {
		return err
	}
	tsf2, err := action.SignedTransfer(addr2, priKey0, 2, big.NewInt(30), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf2); err != nil {
		return err
	}
	tsf3, err := action.SignedTransfer(addr3, priKey0, 3, big.NewInt(50), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf3); err != nil {
		return err
	}
	tsf4, err := action.SignedTransfer(addr4, priKey0, 4, big.NewInt(70), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf4); err != nil {
		return err
	}
	tsf5, err := action.SignedTransfer(addr5, priKey0, 5, big.NewInt(110), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf5); err != nil {
		return err
	}
	tsf6, err := action.SignedTransfer(addr6, priKey0, 6, big.NewInt(5<<20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf6); err != nil {
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
	tsf1, err = action.SignedTransfer(addr1, priKey3, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf1); err != nil {
		return err
	}
	tsf2, err = action.SignedTransfer(addr2, priKey3, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf2); err != nil {
		return err
	}
	tsf3, err = action.SignedTransfer(addr4, priKey3, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf3); err != nil {
		return err
	}
	tsf4, err = action.SignedTransfer(addr5, priKey3, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf4); err != nil {
		return err
	}
	tsf5, err = action.SignedTransfer(addr0, priKey3, 5, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf5); err != nil {
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
	tsf1, err = action.SignedTransfer(addr2, priKey4, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf1); err != nil {
		return err
	}
	tsf2, err = action.SignedTransfer(addr5, priKey4, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf2); err != nil {
		return err
	}
	tsf3, err = action.SignedTransfer(addr6, priKey4, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf3); err != nil {
		return err
	}
	tsf4, err = action.SignedTransfer(addr0, priKey4, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf4); err != nil {
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
	tsf1, err = action.SignedTransfer(addr1, priKey5, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf1); err != nil {
		return err
	}
	tsf2, err = action.SignedTransfer(addr2, priKey5, 2, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf2); err != nil {
		return err
	}
	tsf3, err = action.SignedTransfer(addr3, priKey5, 3, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf3); err != nil {
		return err
	}
	tsf4, err = action.SignedTransfer(addr4, priKey5, 4, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf4); err != nil {
		return err
	}
	tsf5, err = action.SignedTransfer(addr6, priKey5, 5, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf5); err != nil {
		return err
	}
	tsf6, err = action.SignedTransfer(addr0, priKey5, 6, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), tsf6); err != nil {
		return err
	}

	blk, err = bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}

func copyDB(srcDB, dstDB string) error {
	input, err := os.ReadFile(srcDB)
	if err != nil {
		return errors.Wrap(err, "failed to read source db file")
	}
	if err := os.WriteFile(dstDB, input, 0644); err != nil {
		return errors.Wrap(err, "failed to copy db file")
	}
	return nil
}
