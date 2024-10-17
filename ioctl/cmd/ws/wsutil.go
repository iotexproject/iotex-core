package ws

import (
	"bytes"
	"context"
	"encoding/hex"
	"math/big"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

func NewContractCaller(contractabi abi.ABI, contractaddress string) (*ContractCaller, error) {
	caller := &ContractCaller{
		abi:     contractabi,
		wait:    time.Second * 10,
		backoff: 3,
		ctx:     context.Background(),
		amount:  big.NewInt(0),
	}

	// read current account from global config
	addr, err := config.GetContextAddressOrAlias()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current account")
	}
	sender, err := util.Address(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get current account address: %s", addr)
	}
	caller.sender, err = address.FromString(sender)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current account")
	}

	// contract address
	addr, err = util.Address(contractaddress)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse contract address: %s", contractaddress)
	}
	caller.contract, err = address.FromString(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse contract address: %s", contractaddress)
	}

	// initialize api client
	caller.conn, err = util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to chain endpoint: %s", config.ReadConfig.Endpoint)
	}

	caller.client = iotexapi.NewAPIServiceClient(caller.conn)
	if metadata, err := util.JwtAuth(); err == nil {
		caller.ctx = metautils.NiceMD(metadata).ToOutgoing(caller.ctx)
	}

	// read chain id once
	response, err := caller.client.GetChainMeta(caller.ctx, &iotexapi.GetChainMetaRequest{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get chain meta")
	}
	caller.chainID = response.GetChainMeta().GetChainID()
	return caller, nil
}

func NewContractCallerByABI(content []byte, address string) (*ContractCaller, error) {
	_abi, err := abi.JSON(bytes.NewBuffer(content))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse abi content")
	}
	return NewContractCaller(_abi, address)
}

type ContractCaller struct {
	sender   address.Address           // sender as contract caller
	password string                    // password debug only
	client   iotexapi.APIServiceClient // client api service client
	conn     *grpc.ClientConn          // conn grpc connection
	chainID  uint32                    // chainID responding global chain endpoint
	ctx      context.Context           // ctx api invoke context
	abi      abi.ABI                   // abi contract abi
	contract address.Address           // contract address
	wait     time.Duration             // wait wait duration
	backoff  uint64                    // backoff times
	amount   *big.Int                  // amount for tx amount
}

func (c *ContractCaller) SetAmount(amount *big.Int) {
	c.amount = amount
}

func (c *ContractCaller) Sender() address.Address {
	return c.sender
}

func (c *ContractCaller) Close() {
	_ = c.conn.Close()
}

func (c *ContractCaller) SetPassword(password string) {
	c.password = password
}

func (c *ContractCaller) IsSender(cmp common.Address) bool {
	return bytes.Equal(c.sender.Bytes(), cmp.Bytes())
}

func (c *ContractCaller) Read(method string, arguments []any, result *ContractResult) error {
	bytecode, err := c.abi.Pack(method, arguments...)
	if err != nil {
		return errors.Wrapf(err, "failed to pack method: %s", method)
	}

	response, err := c.client.ReadContract(c.ctx, &iotexapi.ReadContractRequest{
		Execution: &iotextypes.Execution{
			Amount:   "0",
			Contract: c.contract.String(),
			Data:     bytecode,
		},
		CallerAddress: c.sender.String(),
	})
	if err != nil {
		return errors.Wrapf(err, "failed to read contract: %s", method)
	}

	if result != nil {
		result.parseOutput(response.Data)
	}
	return nil
}

func (c *ContractCaller) envelop(method string, arguments ...any) (action.Envelope, error) {
	bytecode, err := c.abi.Pack(method, arguments...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to pack method: %s", method)
	}
	tx := action.NewExecution(c.contract.String(), c.amount, bytecode)
	gasPrice, err := c.gasPrice()
	if err != nil {
		return nil, err
	}
	meta, err := c.accountMeta()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get account meta")
	}
	gasLimit, err := c.gasLimit(tx.Proto())
	if err != nil {
		return nil, err
		// gasLimit = 20000000
	}
	balance, ok := new(big.Int).SetString(meta.GetBalance(), 10)
	if !ok {
		return nil, errors.Errorf("failed to convert balance: %s", meta.GetBalance())
	}
	elp := (&action.EnvelopeBuilder{}).SetNonce(meta.GetPendingNonce()).SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).SetAction(tx).SetChainID(c.chainID).Build()
	cost, _ := elp.Cost()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get action's cost")
	}
	if balance.Cmp(cost) < 0 {
		return nil, errors.Errorf("balance is not enough: %s", meta.GetAddress())
	}
	return elp, nil
}

func (c *ContractCaller) gasPrice() (*big.Int, error) {
	response, err := c.client.SuggestGasPrice(c.ctx, &iotexapi.SuggestGasPriceRequest{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get suggest gas price")
	}
	return new(big.Int).SetUint64(response.GetGasPrice()), nil
}

func (c *ContractCaller) gasLimit(tx *iotextypes.Execution) (uint64, error) {
	response, err := c.client.EstimateActionGasConsumption(c.ctx, &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_Execution{
			Execution: tx,
		},
		CallerAddress: c.sender.String(),
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to estimate action gas consumption")
	}
	return response.GetGas(), nil
}

func (c *ContractCaller) accountMeta() (*iotextypes.AccountMeta, error) {
	response, err := c.client.GetAccount(c.ctx, &iotexapi.GetAccountRequest{
		Address: c.sender.String(),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get account meta: %s", c.sender)
	}
	return response.GetAccountMeta(), nil
}

func (c *ContractCaller) Call(method string, arguments ...any) (string, error) {
	envelop, err := c.envelop(method, arguments...)
	if err != nil {
		return "", errors.Wrap(err, "failed to build envelop")
	}

	sk, err := account.PrivateKeyFromSigner(c.sender.String(), c.password)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get private key from %s", c.sender)
	}

	sealed, err := action.Sign(envelop, sk)
	if err != nil {
		return "", errors.Wrap(err, "failed to sign action")
	}
	sk.Zero()

	response, err := c.client.SendAction(c.ctx, &iotexapi.SendActionRequest{Action: sealed.Proto()})
	if err != nil {
		return "", errors.Wrap(err, "failed to send action")
	}
	return response.ActionHash, nil
}

// CallAndRetrieveResult will call contract with `method` and `arguments` and parse result from `targetABI` and `eventName`
func (c *ContractCaller) CallAndRetrieveResult(method string, arguments []any, results ...*ContractResult) (string, error) {
	tx, err := c.Call(method, arguments...)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return tx, nil
	}

	var receipt *iotexapi.GetReceiptByActionResponse
	err = backoff.Retry(func() error {
		receipt, err = c.client.GetReceiptByAction(c.ctx, &iotexapi.GetReceiptByActionRequest{ActionHash: tx})
		if err != nil {
			return err
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(c.wait), c.backoff))

	if err != nil {
		return "", errors.Wrapf(err, "failed to query transaction [tx: %s]", tx)
	}
	if code := receipt.ReceiptInfo.Receipt.Status; code != uint64(iotextypes.ReceiptStatus_Success) {
		return "", errors.Errorf("contract call not executed success: [tx: %s] [code: %d]", tx, code)
	}

	for _, result := range results {
		result.parseEvent(receipt.ReceiptInfo.Receipt.Logs)
	}
	return tx, nil
}

func NewContractResult(target *abi.ABI, name string, value any) *ContractResult {
	r := &ContractResult{
		target: target,
		value:  value,
	}

	_event, ok := target.Events[name]
	if ok {
		r.event = &_event
		r.sig = crypto.Keccak256Hash([]byte(_event.Sig))
	}
	_method, ok := target.Methods[name]
	if ok {
		r.method = &_method
	}
	if r.method == nil && r.event == nil {
		r.err = errors.Errorf("not fount event or method from target abi: %s", name)
	}
	return r
}

type ContractResult struct {
	value  any
	err    error
	target *abi.ABI
	event  *abi.Event
	method *abi.Method
	sig    common.Hash
}

func (r *ContractResult) parseOutput(data string) {
	if r.err != nil {
		return
	}

	if r.method == nil {
		r.err = errors.Errorf("not found method from target abi")
		return
	}

	raw, err := hex.DecodeString(data)
	if err != nil {
		r.err = errors.Wrap(err, "failed to decode hex string")
		return
	}
	res, err := r.target.Unpack(r.method.Name, raw)
	if err != nil {
		r.err = errors.Wrap(err, "failed to unpack method")
		return
	}

	defer func() {
		if err := recover(); err != nil {
			r.err = errors.Errorf("failed to convert from %T to %T", res[0], r.value)
		}
	}()

	r.value = abi.ConvertType(res[0], r.value)
}

func (r *ContractResult) parseEvent(logs []*iotextypes.Log) {
	if r.err != nil {
		return
	}

	if r.event == nil {
		r.err = errors.Errorf("not found event from target abi")
		return
	}

	var log *iotextypes.Log
	for _, l := range logs {
		for _, t := range l.Topics {
			if bytes.Equal(r.sig[:], t) {
				log = l
				break
			}
		}
	}

	// found event log
	if log == nil {
		r.err = errors.Errorf("target event not fount from logs")
		return
	}

	// if value is not assigned skip parsing
	if r.value == nil {
		return
	}

	if err := r.target.UnpackIntoInterface(r.value, r.event.Name, log.Data); err != nil {
		r.err = errors.Wrap(err, "failed to unpack target event data")
		return
	}
	var indexed abi.Arguments
	for _, argument := range r.event.Inputs {
		if argument.Indexed {
			indexed = append(indexed, argument)
		}
	}

	if len(log.Topics) > 1 {
		topics := make([]common.Hash, 0)
		for _, t := range log.Topics[1:] {
			topics = append(topics, common.BytesToHash(t))
		}

		if err := abi.ParseTopics(r.value, indexed, topics); err != nil {
			r.err = errors.Wrap(err, "failed to parse topic data")
			return
		}
	}
}

func (r *ContractResult) Result() (any, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.value, nil
}
