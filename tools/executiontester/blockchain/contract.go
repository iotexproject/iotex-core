// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"encoding/hex"
	"log"
	"math/big"
	"strconv"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/action"
)

const (
	// Producer indicates block producer's encoded address
	Producer = "io13rjq2c07mqhe8sdd7nf9a4vcmnyk9mn72hu94e"
	// ProducerPubKey indicates block producer's public key
	ProducerPubKey = "04403d3c0dbd3270ddfc248c3df1f9aafd60f1d8e7456961c9ef26292262cc68f0ea9690263bef9e197a38f06026814fc70912c2b98d2e90a68f8ddc5328180a01"
	// ProducerPrivKey indicates block producer's private key
	ProducerPrivKey = "82a1556b2dbd0e3615e367edf5d3b90ce04346ec4d12ed71f67c70920ef9ac90"
	// GasLimitPerByte indicates the gas limit for each byte
	GasLimitPerByte uint64 = 100
	// GasPrice indicates the gas price
	GasPrice = 0
)

type (
	// Contract is a basic contract
	Contract interface {
		Start() error
		Exist(string) bool
		Explorer() string
		Deploy(string, ...[]byte) (string, error)
		Read(string, ...[]byte) (string, error)
		ReadValue(string, string, string) (int64, error)
		ReadAndParseToDecimal(string, string, string) (string, error)
		Call(string, ...[]byte) (string, error)
		Transact([]byte, bool) (string, error)
		CheckCallResult(string) (*iotextypes.Receipt, error)

		Address() string
		SetAddress(string) Contract
		SetOwner(string, string) Contract
		SetExecutor(string) Contract
		SetPrvKey(string) Contract
		RunAsOwner() Contract
	}

	contract struct {
		cs       string // blockchain service endpoint
		owner    string // owner of the smart contract
		ownerSk  string // owner's private key
		addr     string // address of the smart contract
		executor string // executor of the smart contract
		prvkey   string // private key of executor
	}
)

// NewContract creates a new contract
func NewContract(exp string) Contract {
	return &contract{cs: exp}
}

func (c *contract) Start() error {
	return nil
}

func (c *contract) Exist(addr string) bool {
	// TODO: add blockchain API to query if the contract exist or not
	return true
}

func (c *contract) Explorer() string {
	return c.cs
}

// ReadValue reads the value
func (c *contract) ReadValue(token, method, sender string) (int64, error) {
	addrSender, err := address.FromString(sender)
	if err != nil {
		return 0, errors.Errorf("invalid account address = %s", sender)
	}
	// check sender balance
	res, err := c.RunAsOwner().SetAddress(token).Read(method, addrSender.Bytes())
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read bytes")
	}
	return strconv.ParseInt(res, 16, 64)
}

// ReadAndParseToDecimal reads the value and parse to decimal string
func (c *contract) ReadAndParseToDecimal(contract, method, sender string) (string, error) {
	addrSender, err := address.FromString(sender)
	if err != nil {
		return "", errors.Errorf("invalid account address = %s", sender)
	}
	// check sender balance
	res, err := c.RunAsOwner().SetAddress(contract).Read(method, addrSender.Bytes())
	if err != nil {
		return "", errors.Wrapf(err, "failed to read bytes")
	}
	bal, err := strconv.ParseInt(res, 16, 64)
	if err != nil {
		return "", errors.Wrapf(err, "failed to convert bytes")
	}
	return strconv.FormatInt(bal, 10), nil
}

func (c *contract) Read(method string, args ...[]byte) (string, error) {
	data, err := hex.DecodeString(method)
	if err != nil {
		return "", err
	}
	if len(data) != 4 {
		return "", errors.Errorf("invalid method id format, length = %d", len(data))
	}
	for _, arg := range args {
		if arg != nil {
			if len(arg) < 32 {
				value := hash.BytesToHash256(arg)
				data = append(data, value[:]...)
			} else {
				data = append(data, arg...)
			}
		}
	}
	return c.Transact(data, true)
}

func (c *contract) Call(method string, args ...[]byte) (string, error) {
	data, err := hex.DecodeString(method)
	if err != nil {
		return "", err
	}
	if len(data) != 4 {
		return "", errors.Errorf("invalid method id format, length = %d", len(data))
	}
	for _, arg := range args {
		if arg != nil {
			if len(arg) < 32 {
				value := hash.BytesToHash256(arg)
				data = append(data, value[:]...)
			} else {
				data = append(data, arg...)
			}
		}
	}
	return c.Transact(data, false)
}

func (c *contract) Deploy(hexCode string, args ...[]byte) (string, error) {
	data, err := hex.DecodeString(hexCode)
	if err != nil {
		return "", err
	}
	for _, arg := range args {
		if arg != nil {
			if len(arg) < 32 {
				value := hash.BytesToHash256(arg)
				data = append(data, value[:]...)
			} else {
				data = append(data, arg...)
			}
		}
	}
	// deploy send to empty address
	return c.SetAddress("").Transact(data, false)
}

func (c *contract) Transact(data []byte, readOnly bool) (string, error) {
	conn, err := grpc.Dial(c.cs, grpc.WithInsecure())
	if err != nil {
		return "", err
	}
	defer conn.Close()

	// get executor's nounce
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	response, err := cli.GetAccount(ctx, &iotexapi.GetAccountRequest{Address: c.executor})
	if err != nil {
		return "", err
	}
	nonce := response.AccountMeta.PendingNonce
	gasPrice := big.NewInt(0)
	gasLimit := GasLimitPerByte*uint64(len(data)) + 5000000
	tx, err := action.NewExecution(
		c.addr,
		nonce,
		big.NewInt(0),
		gasLimit,
		gasPrice,
		data)
	if err != nil {
		return "", err
	}

	if readOnly {
		response, err := cli.ReadContract(ctx, &iotexapi.ReadContractRequest{
			Execution:     tx.Proto(),
			CallerAddress: c.executor,
		})
		if err != nil {
			return "", err
		}
		return response.Data, nil
	}

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(tx).Build()
	prvKey, err := crypto.HexStringToPrivateKey(c.prvkey)
	if err != nil {
		return "", crypto.ErrInvalidKey
	}
	defer prvKey.Zero()
	selp, err := action.Sign(elp, prvKey)
	prvKey.Zero()
	if err != nil {
		return "", err
	}

	_, err = cli.SendAction(ctx, &iotexapi.SendActionRequest{Action: selp.Proto()})
	h, err := selp.Hash()
	if err != nil {
		return "", err
	}
	hex := hex.EncodeToString(h[:])
	if err != nil {
		return hex, errors.Wrapf(err, "tx 0x%s failed to send to Blockchain", hex)
	}
	log.Printf("\n\ntx hash = %s\n\n", hex)
	return hex, nil
}

func (c *contract) CheckCallResult(h string) (*iotextypes.Receipt, error) {
	var rec *iotextypes.Receipt
	// max retry 120 times with interval = 500ms
	checkNum := 120
	err := backoff.Retry(func() error {
		var err error
		rec, err = c.checkCallResult(h)
		log.Printf("Hash: %s <= CheckNum: %d", h, checkNum)
		checkNum--
		return err
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Millisecond*500), uint64(checkNum)))
	return rec, err
}

func (c *contract) checkCallResult(h string) (*iotextypes.Receipt, error) {
	conn, err := grpc.Dial(c.cs, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()
	response, err := cli.GetReceiptByAction(ctx, &iotexapi.GetReceiptByActionRequest{ActionHash: h})
	if err != nil {
		return nil, err
	}
	if response.ReceiptInfo.Receipt.Status != 1 {
		return nil, errors.Errorf("tx 0x%s execution on Blockchain failed", h)
	}
	// TODO: check topics
	return response.ReceiptInfo.Receipt, nil
}

func (c *contract) Address() string {
	return c.addr
}

func (c *contract) SetAddress(addr string) Contract {
	c.addr = addr
	return c
}

func (c *contract) SetOwner(owner, sk string) Contract {
	c.owner = owner
	c.ownerSk = sk
	return c
}

func (c *contract) SetExecutor(exec string) Contract {
	c.executor = exec
	return c
}

func (c *contract) SetPrvKey(prvk string) Contract {
	c.prvkey = prvk
	return c
}

func (c *contract) RunAsOwner() Contract {
	c.executor = c.owner
	c.prvkey = c.ownerSk
	return c
}
