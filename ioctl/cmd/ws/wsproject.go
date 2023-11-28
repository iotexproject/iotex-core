package ws

import (
	"bytes"
	"context"
	_ "embed" // import ws project ABI
	"encoding/hex"
	"reflect"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
)

var (
	// wsProject represents the w3bstream project management command
	wsProject = &cobra.Command{
		Use:   "project",
		Short: config.TranslateInLang(wsProjectShorts, config.UILanguage),
	}

	// wsProjectShorts w3bstream project shorts multi-lang support
	wsProjectShorts = map[config.Language]string{
		config.English: "w3bstream project management",
		config.Chinese: "w3bstream项目管理",
	}

	_flagProjectRegisterContractAddressUsages = map[config.Language]string{
		config.English: "project register contract address",
		config.Chinese: "项目注册合约地址",
	}

	wsProjectRegisterContractAddress string
	wsProjectRegisterContractABI     abi.ABI

	//go:embed wsproject.json
	wsProjectRegisterContractJSONABI []byte
)

const (
	createWsProjectFuncName  = "createProject"
	createWsProjectEventName = "ProjectUpserted"
	updateWsProjectFuncName  = "updateProject"
	queryWsProjectFuncName   = "projects"
)

func init() {
	var err error
	wsProjectRegisterContractABI, err = abi.JSON(bytes.NewReader(wsProjectRegisterContractJSONABI))
	if err != nil {
		log.L().Panic("cannot get abi JSON data", zap.Error(err))
	}

	wsProject.AddCommand(wsProjectCreate)
	wsProject.AddCommand(wsProjectUpdate)
	wsProject.AddCommand(wsProjectQuery)

	wsProject.PersistentFlags().StringVarP(
		&wsProjectRegisterContractAddress,
		"contract-address",
		"c",
		"",
		config.TranslateInLang(_flagProjectRegisterContractAddressUsages, config.UILanguage),
	)
	_ = wsProject.MarkFlagRequired("contract-address")
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
		evabi, err := wsProjectRegisterContractABI.EventByID(common.BytesToHash(l.Topics[0]))
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
