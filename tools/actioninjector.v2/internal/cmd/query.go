package cmd

import (
	"math/rand"
	"os"

	"github.com/bojand/ghz/printer"
	"github.com/bojand/ghz/runner"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/spf13/cobra"

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
	concurrency int
	total       int
)

func init() {
	queryCmd.PersistentFlags().IntVarP(&concurrency, "concurrency", "", 10, "Number of requests to run concurrently")
	queryCmd.PersistentFlags().IntVarP(&total, "total", "", 100, "Number of requests to run")
	rootCmd.AddCommand(queryCmd)
}

func query() error {
	report, err := runner.Run(
		"iotexapi.APIService.GetAccount",
		"api.testnet.iotex.one:443",
		runner.WithProtoFile("proto/api/api.proto", []string{"/Users/chenchen/dev/iotex-proto"}),
		runner.WithConcurrency(uint(concurrency)),
		runner.WithTotalRequests(uint(total)),
		runner.WithDataProvider(func(*runner.CallData) ([]*dynamic.Message, error) {
			protoMsg := &iotexapi.GetAccountRequest{
				Address: identityset.Address(rand.Intn(30)).String(),
			}
			dynamicMsg, err := dynamic.AsDynamicMessage(protoMsg)
			if err != nil {
				return nil, err
			}
			return []*dynamic.Message{dynamicMsg}, nil
		}),
	)
	if err != nil {
		return err
	}
	printer := printer.ReportPrinter{
		Out:    os.Stdout,
		Report: report,
	}
	printer.Print("pretty")
	return nil
}
