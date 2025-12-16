package ethabi

import (
	_ "embed"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

var (
	//go:embed poll.abi.json
	pollABIJSON                                                           string
	pollABI                                                               = initPollABI(pollABIJSON)
	pollCandidatesMethod, pollCandidatesByEpochMethod                     *abi.Method
	pollBlockProducersMethod, pollBlockProducersByEpochMethod             *abi.Method
	pollActiveBlockProducersMethod, pollActiveBlockProducersByEpochMethod *abi.Method
	pollProbationListMethod, pollProbationListByEpochMethod               *abi.Method
)

func initPollABI(abiJSON string) *abi.ABI {
	a, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		panic(err)
	}
	pollCandidatesMethod, err = a.MethodById(signatureToMethodID("CandidatesByEpoch()"))
	if err != nil {
		panic(err)
	}
	pollCandidatesByEpochMethod, err = a.MethodById(signatureToMethodID("CandidatesByEpoch(uint256)"))
	if err != nil {
		panic(err)
	}
	pollBlockProducersMethod, err = a.MethodById(signatureToMethodID("BlockProducersByEpoch()"))
	if err != nil {
		panic(err)
	}
	pollBlockProducersByEpochMethod, err = a.MethodById(signatureToMethodID("BlockProducersByEpoch(uint256)"))
	if err != nil {
		panic(err)
	}
	pollActiveBlockProducersMethod, err = a.MethodById(signatureToMethodID("ActiveBlockProducersByEpoch()"))
	if err != nil {
		panic(err)
	}
	pollActiveBlockProducersByEpochMethod, err = a.MethodById(signatureToMethodID("ActiveBlockProducersByEpoch(uint256)"))
	if err != nil {
		panic(err)
	}
	pollProbationListMethod, err = a.MethodById(signatureToMethodID("ProbationListByEpoch()"))
	if err != nil {
		panic(err)
	}
	pollProbationListByEpochMethod, err = a.MethodById(signatureToMethodID("ProbationListByEpoch(uint256)"))
	if err != nil {
		panic(err)
	}
	return &a
}

func signatureToMethodID(sig string) []byte {
	return crypto.Keccak256([]byte(sig))[:4]
}

func BuildReadStateRequest(data []byte) (protocol.StateContext, error) {
	if len(data) < 4 {
		return nil, errors.Errorf("invalid call data length: %d", len(data))
	}
	method, err := pollABI.MethodById(data[:4])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get method by id %x", data[:4])
	}
	switch method.RawName {
	case pollCandidatesMethod.RawName, pollCandidatesByEpochMethod.RawName:
		return newCandidateListStateContext(data[4:], method, "CandidatesByEpoch")
	case pollBlockProducersMethod.RawName, pollBlockProducersByEpochMethod.RawName:
		return newCandidateListStateContext(data[4:], method, "BlockProducersByEpoch")
	case pollActiveBlockProducersMethod.RawName, pollActiveBlockProducersByEpochMethod.RawName:
		return newCandidateListStateContext(data[4:], method, "ActiveBlockProducersByEpoch")
	case pollProbationListMethod.RawName, pollProbationListByEpochMethod.RawName:
		return newProbationListStateContext(data[4:], method)
	default:
	}
	return nil, nil
}
