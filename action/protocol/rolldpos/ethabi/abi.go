package ethabi

import (
	_ "embed"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

var (
	//go:embed rolldpos.abi.json
	rolldposABIJSON             string
	rolldposABI                 = initRolldposABI(rolldposABIJSON)
	numCandidateDelegatesMethod *abi.Method
	numDelegatesMethod          *abi.Method
	numSubEpochsMethod          *abi.Method
	epochNumberMethod           *abi.Method
	epochHeightMethod           *abi.Method
	epochLastHeightMethod       *abi.Method
	subEpochNumberMethod        *abi.Method
)

func initRolldposABI(abiJSON string) *abi.ABI {
	a, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		panic(err)
	}
	numCandidateDelegatesMethod, err = a.MethodById(signatureToMethodID("NumCandidateDelegates()"))
	if err != nil {
		panic(err)
	}
	numDelegatesMethod, err = a.MethodById(signatureToMethodID("NumDelegates()"))
	if err != nil {
		panic(err)
	}
	numSubEpochsMethod, err = a.MethodById(signatureToMethodID("NumSubEpochs(uint256)"))
	if err != nil {
		panic(err)
	}
	epochNumberMethod, err = a.MethodById(signatureToMethodID("EpochNumber(uint256)"))
	if err != nil {
		panic(err)
	}
	epochHeightMethod, err = a.MethodById(signatureToMethodID("EpochHeight(uint256)"))
	if err != nil {
		panic(err)
	}
	epochLastHeightMethod, err = a.MethodById(signatureToMethodID("EpochLastHeight(uint256)"))
	if err != nil {
		panic(err)
	}
	subEpochNumberMethod, err = a.MethodById(signatureToMethodID("SubEpochNumber(uint256)"))
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
	method, err := rolldposABI.MethodById(data[:4])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get method by id %x", data[:4])
	}
	switch method.RawName {
	case numCandidateDelegatesMethod.RawName:
		return newUint64StateContext(data[4:], method, "NumCandidateDelegates")
	case numDelegatesMethod.RawName:
		return newUint64StateContext(data[4:], method, "NumDelegates")
	case numSubEpochsMethod.RawName:
		return newUint64StateContext(data[4:], method, "NumSubEpochs")
	case epochNumberMethod.RawName:
		return newUint64StateContext(data[4:], method, "EpochNumber")
	case epochHeightMethod.RawName:
		return newUint64StateContext(data[4:], method, "EpochHeight")
	case epochLastHeightMethod.RawName:
		return newUint64StateContext(data[4:], method, "EpochLastHeight")
	case subEpochNumberMethod.RawName:
		return newUint64StateContext(data[4:], method, "SubEpochNumber")
	default:
	}
	return nil, nil
}

// Uint64StateContext handles methods that return a uint64 value
type Uint64StateContext struct {
	*protocol.BaseStateContext
}

func newUint64StateContext(data []byte, methodABI *abi.Method, name string) (*Uint64StateContext, error) {
	var args [][]byte
	paramsMap := map[string]any{}
	if err := methodABI.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}

	// Handle methods with parameters (blockHeight or epochNumber)
	for paramName, paramValue := range paramsMap {
		if paramName == "blockHeight" || paramName == "epochNumber" {
			valueBigInt, ok := paramValue.(*big.Int)
			if !ok {
				return nil, errors.Errorf("failed to cast %s to *big.Int", paramName)
			}
			args = append(args, []byte(valueBigInt.Text(10)))
		}
	}

	return &Uint64StateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: []byte(name),
				Arguments:  args,
			},
			Method: methodABI,
		},
	}, nil
}

// EncodeToEth encode proto to eth
func (r *Uint64StateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	// Parse the response data which is a string representation of uint64
	value, err := strconv.ParseUint(string(resp.Data), 10, 64)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse uint64 response")
	}

	data, err := r.Method.Outputs.Pack(new(big.Int).SetUint64(value), new(big.Int).SetUint64(resp.BlockIdentifier.Height))
	if err != nil {
		return "", errors.Wrap(err, "failed to pack uint64 response")
	}

	return hex.EncodeToString(data), nil
}
