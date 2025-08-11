package action

import (
	_ "embed"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
)

var (
	//go:embed account_contract_abi.json
	AccountContractABIJSON string

	accountContractABI           abi.ABI
	accountContractTransferEvent abi.Event

	onceAcc sync.Once
)

func initAccountContractABI() {
	onceAcc.Do(func() {
		var err error
		accountContractABI, err = abi.JSON(strings.NewReader(AccountContractABIJSON))
		if err != nil {
			panic("failed to load account contract ABI: " + err.Error())
		}
		var ok bool
		accountContractTransferEvent, ok = accountContractABI.Events["Transfer"]
		if !ok {
			panic("failed to load Transfer event from account contract ABI")
		}
	})
}

// AccountContractABI returns the ABI of the account contract
func AccountContractABI() *abi.ABI {
	initAccountContractABI()
	return &accountContractABI
}

// PackAccountTransferEvent packs the parameters for the account transfer event
func PackAccountTransferEvent(
	from, to address.Address, amount *big.Int, typ uint8,
) (Topics, []byte, error) {
	initAccountContractABI()
	data, err := accountContractTransferEvent.Inputs.NonIndexed().Pack(amount)
	if err != nil {
		return nil, nil, err
	}
	topics := make(Topics, 4)
	topics[0] = hash.Hash256(accountContractTransferEvent.ID)
	topics[1] = hash.Hash256(common.BytesToHash(from.Bytes()))
	topics[2] = hash.Hash256(common.BytesToHash(to.Bytes()))
	topics[3] = hash.Hash256(common.BytesToHash([]byte{typ}))
	return topics, data, nil
}
