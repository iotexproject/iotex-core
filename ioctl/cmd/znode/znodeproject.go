package znode

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/hex"
	"fmt"
	"github.com/cenkalti/backoff"
	"github.com/ethereum/go-ethereum/common"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"math/big"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/spf13/cobra"
)

var (
	// znodeProject represents the znode project management command
	znodeProject = &cobra.Command{
		Use:   "project",
		Short: config.TranslateInLang(znodeProjectShorts, config.UILanguage),
	}

	// znodeProjectShorts znode project shorts multi-lang support
	znodeProjectShorts = map[config.Language]string{
		config.English: "znode project management",
		config.Chinese: "znode项目管理",
	}

	_flagProjectRegisterContractAddressUsages = map[config.Language]string{
		config.English: "project register contract address",
		config.Chinese: "项目注册合约地址",
	}

	znodeProjectRegisterContractAddress string
	znodeProjectRegisterContractABI     abi.ABI

	//go:embed znodeproject.abi
	znodeProjectRegisterContractJsonABI []byte
)

const (
	createZnodeProjectFuncName  = "createProject"
	createZnodeProjectEventName = "ProjectUpserted"
	updateZnodeProjectFuncName  = "updateProject"
	queryZnodeProjectFuncName   = "projects"
)

func init() {
	var err error
	znodeProjectRegisterContractABI, err = abi.JSON(bytes.NewReader(znodeProjectRegisterContractJsonABI))
	if err != nil {
		log.L().Panic("cannot get abi JSON data", zap.Error(err))
	}

	znodeProject.AddCommand(znodeProjectCreate)
	znodeProject.AddCommand(znodeProjectUpdate)
	znodeProject.AddCommand(znodeProjectQuery)

	znodeProject.PersistentFlags().StringVarP(
		&znodeProjectRegisterContractAddress,
		"contract-address",
		"c",
		"",
		config.TranslateInLang(_flagProjectRegisterContractAddressUsages, config.UILanguage),
	)
	_ = znodeProject.MarkFlagRequired("contract-address")
}

func convertStringToAbiBytes32(hash string) (interface{}, error) {
	t, _ := abi.NewType("bytes32", "", nil)

	bytecode, err := hex.DecodeString(util.TrimHexPrefix(hash))
	if err != nil {
		return nil, err
	}

	if t.Size != len(bytecode) {
		return nil, errors.New("invalid arg")
	}

	bytesType := reflect.ArrayOf(t.Size, reflect.TypeOf(uint8(0)))
	bytesVal := reflect.New(bytesType).Elem()

	for i, b := range bytecode {
		bytesVal.Index(i).Set(reflect.ValueOf(b))
	}

	return bytesVal.Interface(), nil
}

func waitReceiptByActionHash(h string) (*iotexapi.GetReceiptByActionResponse, error) {
	conn, err := util.ConnectToEndpoint(config.ReadConfig.SecureConnect && !config.Insecure)
	if err != nil {
		return nil, output.NewError(output.NetworkError, "failed to connect to endpoint", err)
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	ctx := context.Background()

	jwtMD, err := util.JwtAuth()
	if err == nil {
		ctx = metautils.NiceMD(jwtMD).ToOutgoing(ctx)
	}

	var rsp *iotexapi.GetReceiptByActionResponse
	err = backoff.Retry(func() error {
		rsp, err = cli.GetReceiptByAction(ctx, &iotexapi.GetReceiptByActionRequest{
			ActionHash: h,
		})
		return err
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(30*time.Second), 3))
	if err != nil {
		sta, ok := status.FromError(err)
		if ok && sta.Code() == codes.NotFound {
			return nil, output.NewError(output.APIError, "not found", nil)
		} else if ok {
			return nil, output.NewError(output.APIError, sta.Message(), nil)
		}
		return nil, output.NewError(output.NetworkError, "failed to invoke GetReceiptByAction api", err)
	}
	return rsp, nil
}

func getEventInputsByName(logs []*iotextypes.Log, eventName string) (map[string]any, error) {
	var (
		abievent *abi.Event
		log      *iotextypes.Log
	)
	for _, l := range logs {
		evabi, err := znodeProjectRegisterContractABI.EventByID(common.BytesToHash(l.Topics[0]))
		if err != nil {
			return nil, errors.Wrapf(err, "get event abi from topic %v failed", l.Topics[0])
		}
		if evabi.Name == eventName {
			abievent, log = evabi, l
			break
		}
	}

	if abievent == nil || log == nil {
		return nil, errors.Errorf("event not found: %s", eventName)
	}

	inputs := make(map[string]any)
	if len(log.Data) > 0 {
		if err := abievent.Inputs.UnpackIntoMap(inputs, log.Data); err != nil {
			return nil, errors.Wrap(err, "unpack event data failed")
		}
	}
	args := make(abi.Arguments, 0)
	for _, arg := range abievent.Inputs {
		if arg.Indexed {
			args = append(args, arg)
		}
	}
	topics := make([]common.Hash, 0)
	for i, topic := range log.Topics {
		if i > 0 {
			topics = append(topics, common.BytesToHash(topic))
		}
	}
	if err := abi.ParseTopicsIntoMap(inputs, args, topics); err != nil {
		return nil, errors.Wrap(err, "unpack event indexed fields failed")
	}

	return inputs, nil
}

func parseOutput(targetAbi *abi.ABI, targetMethod string, result string) (string, error) {
	resultBytes, err := hex.DecodeString(result)
	if err != nil {
		return "", errors.Wrap(err, "failed to decode result")
	}

	var (
		outputArgs = targetAbi.Methods[targetMethod].Outputs
		tupleStr   = make([]string, 0, len(outputArgs))
	)

	v, err := targetAbi.Unpack(targetMethod, resultBytes)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse output")
	}

	if len(outputArgs) == 1 {
		elemStr, _ := parseOutputArgument(v[0], &outputArgs[0].Type)
		return elemStr, nil
	}

	for i, field := range v {
		elemStr, _ := parseOutputArgument(field, &outputArgs[i].Type)
		tupleStr = append(tupleStr, outputArgs[i].Name+":"+elemStr)
	}
	return "{" + strings.Join(tupleStr, " ") + "}", nil
}

func parseOutputArgument(v interface{}, t *abi.Type) (string, bool) {
	str := fmt.Sprint(v)
	ok := false

	switch t.T {
	case abi.StringTy, abi.BoolTy:
		// case abi.StringTy & abi.BoolTy can be handled by fmt.Sprint()
		ok = true

	case abi.TupleTy:
		if reflect.TypeOf(v).Kind() == reflect.Struct {
			ok = true

			tupleStr := make([]string, 0, len(t.TupleElems))
			for i, elem := range t.TupleElems {
				elemStr, elemOk := parseOutputArgument(reflect.ValueOf(v).Field(i).Interface(), elem)
				tupleStr = append(tupleStr, t.TupleRawNames[i]+":"+elemStr)
				ok = ok && elemOk
			}

			str = "{" + strings.Join(tupleStr, " ") + "}"
		}

	case abi.SliceTy, abi.ArrayTy:
		if reflect.TypeOf(v).Kind() == reflect.Slice || reflect.TypeOf(v).Kind() == reflect.Array {
			ok = true

			value := reflect.ValueOf(v)
			sliceStr := make([]string, 0, value.Len())
			for i := 0; i < value.Len(); i++ {
				elemStr, elemOk := parseOutputArgument(value.Index(i).Interface(), t.Elem)
				sliceStr = append(sliceStr, elemStr)
				ok = ok && elemOk
			}

			str = "[" + strings.Join(sliceStr, " ") + "]"
		}

	case abi.IntTy, abi.UintTy:
		if reflect.TypeOf(v) == reflect.TypeOf(big.NewInt(0)) {
			var bigInt *big.Int
			bigInt, ok = v.(*big.Int)
			if ok {
				str = bigInt.String()
			}
		} else if 2 <= reflect.TypeOf(v).Kind() && reflect.TypeOf(v).Kind() <= 11 {
			// other integer types (int8,uint16,...) can be handled by fmt.Sprint(v)
			ok = true
		}

	case abi.AddressTy:
		if reflect.TypeOf(v) == reflect.TypeOf(common.Address{}) {
			var ethAddr common.Address
			ethAddr, ok = v.(common.Address)
			if ok {
				ioAddress, err := address.FromBytes(ethAddr.Bytes())
				if err == nil {
					str = ioAddress.String()
				}
			}
		}

	case abi.BytesTy:
		if reflect.TypeOf(v) == reflect.TypeOf([]byte{}) {
			var bytes []byte
			bytes, ok = v.([]byte)
			if ok {
				str = "0x" + hex.EncodeToString(bytes)
			}
		}

	case abi.FixedBytesTy, abi.FunctionTy:
		if reflect.TypeOf(v).Kind() == reflect.Array && reflect.TypeOf(v).Elem() == reflect.TypeOf(byte(0)) {
			bytesValue := reflect.ValueOf(v)
			byteSlice := reflect.MakeSlice(reflect.TypeOf([]byte{}), bytesValue.Len(), bytesValue.Len())
			reflect.Copy(byteSlice, bytesValue)

			str = "0x" + hex.EncodeToString(byteSlice.Bytes())
			ok = true
		}
	}

	return str, ok
}
