// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
)

func TestSendRaw(t *testing.T) {
	config.ReadConfig.Endpoint = "api.testnet.iotex.one:80"
	config.ReadConfig.SecureConnect = false
	require := require.New(t)
	nonce := uint64(33)
	amount := big.NewInt(1000000000000000000) //1 iotx
	receipt := "io1eyn9tc6t782zx4zgy3hgt32hpz6t8v7pgf524z"
	gaslimit := uint64(300000)
	gasprice := big.NewInt(1000000000000)
	tx, err := action.NewTransfer(nonce, amount,
		receipt, []byte(""), gaslimit, gasprice)
	require.NoError(err)
	elp := (&action.EnvelopeBuilder{}).
		SetNonce(nonce).
		SetGasPrice(gasprice).
		SetGasLimit(gaslimit).
		SetAction(tx).Build()
	pri, err := crypto.HexStringToPrivateKey("0d4d9b248110257c575ef2e8d93dd53471d9178984482817dcbd6edb607f8cc5")
	require.NoError(err)
	sealed, err := action.Sign(elp, pri)
	act := sealed.Proto()
	act.Signature[64] = act.Signature[64] + 27
	b, err := proto.Marshal(act)
	require.NoError(err)
	fmt.Println(hex.EncodeToString(b))

	actBytes, err := hex.DecodeString(hex.EncodeToString(b))
	require.NoError(err)
	actRet := &iotextypes.Action{}
	err = proto.Unmarshal(actBytes, actRet)
	require.NoError(err)
	require.NoError(sendRaw(actRet)) //connect error

	// test action deploy
	bytecodeString := "608060405234801561001057600080fd5b50610123600102600281600019169055503373ffffffffffffffffffffffffffffffffffffffff166001026003816000191690555060035460025417600481600019169055506102ae806100656000396000f300608060405260043610610078576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680630cc0e1fb1461007d57806328f371aa146100b05780636b1d752b146100df578063d4b8399214610112578063daea85c514610145578063eb6fd96a14610188575b600080fd5b34801561008957600080fd5b506100926101bb565b60405180826000191660001916815260200191505060405180910390f35b3480156100bc57600080fd5b506100c56101c1565b604051808215151515815260200191505060405180910390f35b3480156100eb57600080fd5b506100f46101d7565b60405180826000191660001916815260200191505060405180910390f35b34801561011e57600080fd5b506101276101dd565b60405180826000191660001916815260200191505060405180910390f35b34801561015157600080fd5b50610186600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506101e3565b005b34801561019457600080fd5b5061019d61027c565b60405180826000191660001916815260200191505060405180910390f35b60035481565b6000600454600019166001546000191614905090565b60025481565b60045481565b3373ffffffffffffffffffffffffffffffffffffffff166001028173ffffffffffffffffffffffffffffffffffffffff16600102176001816000191690555060016000808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81548160ff02191690831515021790555050565b600154815600a165627a7a7230582089b5f99476d642b66a213c12cd198207b2e813bb1caf3bd75e22be535ebf5d130029"
	bytecode, err := hex.DecodeString(strings.TrimPrefix(bytecodeString, "0x"))
	require.NoError(err)
	exec, err := action.NewExecution("", nonce+1, amount, gaslimit, gasprice, bytecode)
	require.NoError(err)
	elp = (&action.EnvelopeBuilder{}).
		SetNonce(nonce + 1).
		SetGasPrice(gasprice).
		SetGasLimit(gaslimit).
		SetAction(exec).Build()
	sealed, err = action.Sign(elp, pri)
	act = sealed.Proto()
	act.Signature[64] = act.Signature[64] + 37
	b, err = proto.Marshal(act)
	require.NoError(err)
	fmt.Println(hex.EncodeToString(b))

	actBytes, err = hex.DecodeString(hex.EncodeToString(b))
	require.NoError(err)
	actRet = &iotextypes.Action{}
	err = proto.Unmarshal(actBytes, actRet)
	require.NoError(err)
	require.NoError(sendRaw(actRet))
}
