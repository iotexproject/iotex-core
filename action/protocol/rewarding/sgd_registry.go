package rewarding

import (
	"context"

	"github.com/iotexproject/iotex-address/address"
)

type SGDRegistry interface {
	CheckContract(contract string) (address.Address, uint64, bool)
}

// ReadContract defines a callback function to read contract
type ReadContract func(context.Context, string, []byte, bool) ([]byte, error)

type sgdRegistry struct {
	sgdContractAddress string
	readContract       ReadContract
}

func NewSGDRegistry(sgdContractAddress string, readContract ReadContract) SGDRegistry {
	return &sgdRegistry{
		sgdContractAddress: sgdContractAddress,
		readContract:       readContract,
	}
}

func (r *sgdRegistry) CheckContract(contract string) (address.Address, uint64, bool) {

	return &address.AddrV1{}, 0, false
}
