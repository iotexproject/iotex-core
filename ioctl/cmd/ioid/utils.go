package ioid

import (
	"encoding/hex"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/ioctl/cmd/action"
)

func readContract(
	contract address.Address,
	contractABI abi.ABI,
	method string,
	value string,
	args ...interface{},
) ([]interface{}, error) {
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		return nil, err
	}
	res, err := action.Read(contract, value, data)
	if err != nil {
		return nil, err
	}
	data, _ = hex.DecodeString(res)
	return contractABI.Unpack(method, data)
}
