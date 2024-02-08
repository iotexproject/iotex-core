package action

import (
	_ "embed" // import stakingContractABIJSON
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/iotexproject/iotex-core/pkg/log"
)

var (
	//go:embed staking_contract.abi
	stakingContractABIJSON string
	stakingContractABI     abi.ABI

	candidateActivateMethod    abi.Method
	candidateEndorsementMethod abi.Method
)

func init() {
	var (
		err error
		ok  bool
	)
	stakingContractABI, err = abi.JSON(strings.NewReader(stakingContractABIJSON))
	if err != nil {
		log.S().Panicf("failed to parse staking contract ABI: %v", err)
	}
	candidateActivateMethod, ok = stakingContractABI.Methods["candidateActivate"]
	if !ok {
		log.S().Panic("failed to get candidateSelfStake method")
	}
	candidateEndorsementMethod, ok = stakingContractABI.Methods["candidateEndorsement"]
	if !ok {
		log.S().Panic("failed to get candidateRegister method")
	}
}
