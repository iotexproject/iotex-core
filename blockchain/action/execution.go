// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"encoding/hex"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

// EmptyAddress is the empty string
const EmptyAddress = ""

var (
	// ErrExecutionError indicates error for an execution action
	ErrExecutionError = errors.New("execution error")
)

// Execution defines the struct of account-based contract execution
type Execution struct {
	Version        uint32
	Nonce          uint64
	Amount         *big.Int
	Executor       string
	Contract       string
	ExecutorPubKey keypair.PublicKey
	Gas            uint64
	GasPrice       uint64
	Signature      []byte
	Data           []byte
}

// NewExecution returns a Execution instance
func NewExecution(executorAddress string, contractAddress string, nonce uint64, amount *big.Int, gas uint64, gasPrice uint64, data []byte) (*Execution, error) {
	if executorAddress == "" {
		return nil, errors.Wrap(ErrAddr, "address of the executor is empty")
	}

	return &Execution{
		Version:  version.ProtocolVersion,
		Nonce:    nonce,
		Data:     data,
		Amount:   amount,
		Gas:      gas,
		GasPrice: gasPrice,
		Executor: executorAddress,
		Contract: contractAddress,
	}, nil
}

// TotalSize returns the total size of this Execution
func (ex *Execution) TotalSize() uint32 {
	size := VersionSizeInBytes
	size += NonceSizeInBytes
	if ex.Amount != nil && len(ex.Amount.Bytes()) > 0 {
		size += len(ex.Amount.Bytes())
	}
	size += len(ex.Executor)
	size += len(ex.Contract)
	size += len(ex.ExecutorPubKey)
	size += len(ex.Signature)
	size += GasSizeInBytes
	size += GasPriceSizeInBytes
	size += len(ex.Data)
	return uint32(size)
}

// ByteStream returns a raw byte stream of this Transfer
func (ex *Execution) ByteStream() []byte {
	stream := make([]byte, VersionSizeInBytes)
	enc.MachineEndian.PutUint32(stream, ex.Version)
	temp := make([]byte, NonceSizeInBytes)
	enc.MachineEndian.PutUint64(temp, ex.Nonce)
	stream = append(stream, temp...)
	if ex.Amount != nil && len(ex.Amount.Bytes()) > 0 {
		stream = append(stream, ex.Amount.Bytes()...)
	}
	stream = append(stream, ex.Executor...)
	stream = append(stream, ex.Contract...)
	stream = append(stream, ex.ExecutorPubKey[:]...)
	temp = make([]byte, GasSizeInBytes)
	enc.MachineEndian.PutUint64(temp, ex.Gas)
	stream = append(stream, temp...)
	temp = make([]byte, GasPriceSizeInBytes)
	enc.MachineEndian.PutUint64(temp, ex.GasPrice)
	stream = append(stream, temp...)
	// Signature = Sign(hash(ByteStream())), so not included
	stream = append(stream, ex.Data...)
	return stream
}

// ConvertToActionPb converts Execution to protobuf's ActionPb
func (ex *Execution) ConvertToActionPb() *iproto.ActionPb {
	act := &iproto.ActionPb{
		Action: &iproto.ActionPb_Execution{
			Execution: &iproto.ExecutionPb{
				Executor:       ex.Executor,
				Contract:       ex.Contract,
				ExecutorPubKey: ex.ExecutorPubKey[:],
				Gas:            ex.Gas,
				GasPrice:       ex.GasPrice,
				Data:           ex.Data,
			},
		},
		Version:   ex.Version,
		Nonce:     ex.Nonce,
		Signature: ex.Signature,
	}
	if ex.Amount != nil && len(ex.Amount.Bytes()) > 0 {
		act.GetExecution().Amount = ex.Amount.Bytes()
	}

	return act
}

// ToJSON converts Execution to ExecutionJSON
func (ex *Execution) ToJSON() (*explorer.Execution, error) {
	execution := &explorer.Execution{
		Version:        int64(ex.Version),
		Nonce:          int64(ex.Nonce),
		Executor:       ex.Executor,
		Contract:       ex.Contract,
		Amount:         ex.Amount.Int64(),
		ExecutorPubKey: keypair.EncodePublicKey(ex.ExecutorPubKey),
		Gas:            int64(ex.Gas),
		GasPrice:       int64(ex.GasPrice),
		Data:           hex.EncodeToString(ex.Data),
		Signature:      hex.EncodeToString(ex.Signature),
	}
	return execution, nil
}

// Serialize returns a serialized byte stream for the Execution
func (ex *Execution) Serialize() ([]byte, error) {
	return proto.Marshal(ex.ConvertToActionPb())
}

// ConvertFromActionPb converts a protobuf's ActionPb to Execution
func (ex *Execution) ConvertFromActionPb(pbAct *iproto.ActionPb) {
	ex.Version = pbAct.GetVersion()
	ex.Nonce = pbAct.GetNonce()
	pbExecution := pbAct.GetExecution()
	ex.Executor = pbExecution.Executor
	ex.Contract = pbExecution.GetContract()
	copy(ex.ExecutorPubKey[:], pbExecution.GetExecutorPubKey())
	ex.Gas = pbExecution.GetGas()
	ex.GasPrice = pbExecution.GetGasPrice()
	ex.Signature = pbAct.GetSignature()
	ex.Data = pbExecution.GetData()
	if ex.Amount == nil {
		ex.Amount = big.NewInt(0)
	}
	if len(pbExecution.Amount) > 0 {
		ex.Amount.SetBytes(pbExecution.GetAmount())
	}
}

// NewExecutionFromJSON creates a new Execution from ExecutionJSON
func NewExecutionFromJSON(jsonExecution *explorer.Execution) (*Execution, error) {
	ex := &Execution{}
	ex.Version = uint32(jsonExecution.Version)
	ex.Nonce = uint64(jsonExecution.Nonce)
	ex.Executor = jsonExecution.Executor
	ex.Contract = jsonExecution.Contract
	executorPubKey, err := keypair.StringToPubKeyBytes(jsonExecution.ExecutorPubKey)
	if err != nil {
		logger.Error().Err(err).Msg("Fail to create a new Execution from ExecutionJSON")
		return nil, err
	}
	copy(ex.ExecutorPubKey[:], executorPubKey)
	data, err := hex.DecodeString(jsonExecution.Data)
	if err != nil {
		logger.Error().Err(err).Msg("Fail to create a new Execution from ExecutionJSON")
		return nil, err
	}
	ex.Data = data
	signature, err := hex.DecodeString(jsonExecution.Signature)
	if err != nil {
		logger.Error().Err(err).Msg("Fail to create a new Execution from ExecutionJSON")
		return nil, err
	}
	ex.Signature = signature

	return ex, nil
}

// Deserialize parse the byte stream into Execution
func (ex *Execution) Deserialize(buf []byte) error {
	pbAction := &iproto.ActionPb{}
	if err := proto.Unmarshal(buf, pbAction); err != nil {
		return err
	}
	ex.ConvertFromActionPb(pbAction)
	return nil
}

// Hash returns the hash of the Execution
func (ex *Execution) Hash() hash.Hash32B {
	return blake2b.Sum256(ex.ByteStream())
}

// Sign signs the Execution using executer's private key
func (ex *Execution) Sign(executor *iotxaddress.Address) (*Execution, error) {
	// check the sender is correct
	if ex.Executor != executor.RawAddress {
		return nil, errors.Wrapf(ErrExecutionError, "signing addr %s does not match with Execution addr %s",
			ex.Executor, executor.RawAddress)
	}
	// check the public key is actually owned by sender
	pkhash, err := iotxaddress.GetPubkeyHash(executor.RawAddress)
	if err != nil {
		return nil, errors.Wrap(err, "error when get the pubkey hash")
	}
	if !bytes.Equal(pkhash, keypair.HashPubKey(executor.PublicKey)) {
		return nil, errors.Wrapf(ErrExecutionError, "signing addr %s does not own correct public key",
			executor.RawAddress)
	}
	ex.ExecutorPubKey = executor.PublicKey
	if err := ex.sign(executor); err != nil {
		return nil, err
	}
	return ex, nil
}

// Verify verifies the Execution using sender's public key
func (ex *Execution) Verify(sender *iotxaddress.Address) error {
	hash := ex.Hash()
	if success := crypto.EC283.Verify(sender.PublicKey, hash[:], ex.Signature); success {
		return nil
	}
	return errors.Wrapf(ErrExecutionError, "Failed to verify Execution signature = %x", ex.Signature)
}

//======================================
// private functions
//======================================

func (ex *Execution) sign(sender *iotxaddress.Address) error {
	hash := ex.Hash()
	if ex.Signature = crypto.EC283.Sign(sender.PrivateKey, hash[:]); ex.Signature != nil {
		return nil
	}
	return errors.Wrapf(ErrExecutionError, "Failed to sign Execution hash = %x", hash)
}
