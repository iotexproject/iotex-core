package systemcontracts

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

var (
	SystemContractAddresses Config
)

type Config struct {
	PollContractAddress address.Address
}

func init() {
	poll, err := address.FromBytes(crypto.CreateAddress(common.Address{}, 0).Bytes())
	if err != nil {
		log.S().Panic("Invalid poll contract address: " + err.Error())
	}
	SystemContractAddresses = Config{
		PollContractAddress: poll,
	}
}
