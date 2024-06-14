package cmd

import (
	_ "embed"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/bojand/ghz/printer"
	"github.com/bojand/ghz/runner"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/runtime/protoiface"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/test/identityset"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Sub-Command for querying IoTeX blockchain via gRPC",
	Long:  "Sub-Command for querying IoTeX blockchain via gRPC",
	RunE: func(cmd *cobra.Command, args []string) error {
		return query()
	},
}

var (
	//go:embed erc20.abi.json
	erc20JSONABI string
	erc20ABI     abi.ABI

	//go:embed api.protoset
	protosetBinary []byte

	rps         int
	concurrency int
	total       int
	endpoint    string
	insecure    bool
	async       bool
	duration    time.Duration

	methods = []string{
		"iotexapi.APIService.GetAccount",
		"iotexapi.APIService.EstimateActionGasConsumption",
		"iotexapi.APIService.ReadContract",
	}
)

func init() {
	var err error
	erc20ABI, err = abi.JSON(strings.NewReader(erc20JSONABI))
	if err != nil {
		panic(err)
	}

	queryCmd.PersistentFlags().IntVarP(&rps, "rps", "", 100, "Number of requests per second")
	queryCmd.PersistentFlags().IntVarP(&concurrency, "concurrency", "", 10, "Number of requests to run concurrently")
	// queryCmd.PersistentFlags().IntVarP(&total, "total", "", 100, "Number of requests to run")
	queryCmd.PersistentFlags().StringVarP(&endpoint, "endpoint", "", "api.testnet.iotex.one:443", "gRPC endpoint to query")
	queryCmd.PersistentFlags().BoolVarP(&insecure, "insecure", "", false, "Use insecure connection")
	queryCmd.PersistentFlags().BoolVarP(&async, "async", "", false, "Use async mode")
	queryCmd.PersistentFlags().DurationVarP(&duration, "duration", "", 0, "Duration of test")
	rootCmd.AddCommand(queryCmd)
}

func query() error {
	reports := make([]*runner.Report, len(methods))
	g := errgroup.Group{}
	conMethod := concurrency / len(methods)
	rpcMethod := rps / len(methods)
	for i := range methods {
		id := i
		g.Go(func() error {
			report, err := runner.Run(
				methods[id],
				endpoint,
				runner.WithProtosetBinary(protosetBinary),
				runner.WithConcurrency(uint(conMethod)),
				runner.WithAsync(async),
				runner.WithRPS(uint(rpcMethod)),
				runner.WithRunDuration(duration),
				runner.WithDataProvider(provider),
				runner.WithInsecure(insecure),
			)
			if err != nil {
				return err
			}
			reports[id] = report
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	for i, report := range reports {
		printer := printer.ReportPrinter{
			Out:    os.Stdout,
			Report: report,
		}
		printer.Out.Write([]byte("--------------------\n"))
		printer.Out.Write([]byte(fmt.Sprintf("RPC Method: %s\n", methods[i])))
		printer.Print("summary")
	}
	return nil
}

func provider(call *runner.CallData) ([]*dynamic.Message, error) {
	var protoMsg protoiface.MessageV1
	switch call.MethodName {
	case "ReadContract":
		data, err := erc20ABI.Pack("balanceOf", common.BytesToAddress(identityset.Address(rand.Intn(30)).Bytes()))
		if err != nil {
			return nil, err
		}
		execution, err := action.NewExecution("io1qfvgvmk6lpxkpqwlzanqx4atyzs86ryqjnfuad", 0, big.NewInt(0), 0, big.NewInt(0), data)
		if err != nil {
			return nil, err
		}
		protoMsg = &iotexapi.ReadContractRequest{
			Execution:     execution.Proto(),
			CallerAddress: identityset.Address(rand.Intn(30)).String(),
		}
	case "EstimateActionGasConsumption":
		data, err := erc20ABI.Pack("balanceOf", common.BytesToAddress(identityset.Address(rand.Intn(30)).Bytes()))
		if err != nil {
			return nil, err
		}
		execution, err := action.NewExecution("io1qfvgvmk6lpxkpqwlzanqx4atyzs86ryqjnfuad", 0, big.NewInt(0), 0, big.NewInt(0), data)
		if err != nil {
			return nil, err
		}
		protoMsg = &iotexapi.EstimateActionGasConsumptionRequest{
			Action: &iotexapi.EstimateActionGasConsumptionRequest_Execution{
				Execution: execution.Proto(),
			},
			CallerAddress: identityset.Address(rand.Intn(30)).String(),
		}
	case "GetAccount":
		protoMsg = &iotexapi.GetAccountRequest{
			Address: identityset.Address(rand.Intn(30)).String(),
		}
	default:
		return nil, errors.Errorf("not supported method %s", call.MethodName)
	}

	dynamicMsg, err := dynamic.AsDynamicMessage(protoMsg)
	if err != nil {
		return nil, err
	}
	return []*dynamic.Message{dynamicMsg}, nil
}
