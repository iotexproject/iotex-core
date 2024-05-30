package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"github.com/schollz/progressbar/v2"

	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
	"github.com/iotexproject/iotex-core/tools/archivevalidator/erc20"
)

var (
	endpoint             = ""
	balanceCasePath      = ""
	erc20BalanceCasePath = ""
	workerNum            = 10
	timeout              = 10

	cli *ethclient.Client
)

type (
	BalanceCase struct {
		Address     common.Address
		BlockHeight int
		Balance     *big.Int
	}

	ContractBalanceCase struct {
		ContractAddress common.Address
		Address         common.Address
		BlockHeight     int
		Balance         *big.Int
	}
)

func init() {
	flag.StringVar(&endpoint, "endpoint", "", "endpoint of the archive node")
	flag.StringVar(&balanceCasePath, "balance", "", "balance case file path")
	flag.StringVar(&erc20BalanceCasePath, "erc20balance", "", "erc20 balance case file path")
	flag.IntVar(&workerNum, "worker", 10, "number of workers")
	flag.IntVar(&timeout, "timeout", 10, "timeout in seconds")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "usage: archivevalidator -balance=[string] -erc20balance=[string]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
}

func main() {
	var err error
	cli, err = ethclient.DialContext(context.Background(), endpoint)
	if err != nil {
		fmt.Println("failed to connect to archive node:", err)
		return
	}
	if balanceCasePath != "" {
		balanceCases, err := parseBalanceCaseFile(balanceCasePath)
		if err != nil {
			fmt.Println("failed to parse balance case file:", err)
			os.Exit(1)
		}
		validateCases[*BalanceCase](balanceCases)
	}
	if erc20BalanceCasePath != "" {
		contractBalanceCases, err := parseContractCaseFile(erc20BalanceCasePath)
		if err != nil {
			fmt.Println("failed to parse erc20 balance case file:", err)
			os.Exit(1)
		}
		validateCases[*ContractBalanceCase](contractBalanceCases)
	}
}

func validateCases[T any](cases []T) {
	fmt.Printf("Validating balance cases from %s: %d\n", balanceCasePath, len(cases))
	pb := progressbar.NewOptions(len(cases), progressbar.OptionShowCount(), progressbar.OptionSetBytes(0), progressbar.OptionShowIts(), progressbar.OptionThrottle(time.Second))
	if err := pb.RenderBlank(); err != nil {
		fmt.Println("failed to render progress bar:", err)
	}
	ch := make(chan T, 10*workerNum)
	wg := sync.WaitGroup{}
	wg.Add(workerNum)
	for i := 0; i < workerNum; i++ {
		go func() {
			for bc := range ch {
				if err := validateCase(bc); err != nil {
					fmt.Printf("failed to validate case %+v, %+v\n", bc, err)
				}
				pb.Add(1)
			}
			wg.Done()
		}()
	}
	for _, bc := range cases {
		ch <- bc
	}
	close(ch)
	wg.Wait()
	pb.Finish()
	fmt.Printf("\n")
}

func validateCase(c any) error {
	return backoff.Retry(func() error {
		switch v := c.(type) {
		case *BalanceCase:
			return validateBalanceCases(v)
		case *ContractBalanceCase:
			return validateERC20BalanceCases(v)
		default:
			return errors.Errorf("unknown case type: %T", c)
		}
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second), 3))
}

func validateBalanceCases(bc *BalanceCase) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Second*time.Duration(timeout))
	b, err := cli.BalanceAt(ctx, bc.Address, big.NewInt(int64(bc.BlockHeight)))
	if err != nil {
		return errors.Wrapf(err, "failed to get balance at %d", bc.BlockHeight)
	}
	if b.Cmp(bc.Balance) != 0 {
		return errors.Errorf("balance mismatch: expected %s, got %s", bc.Balance.String(), b.String())
	}
	return nil
}

func validateERC20BalanceCases(bc *ContractBalanceCase) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Second*time.Duration(timeout))
	token, err := erc20.NewErc20(bc.ContractAddress, cli)
	if err != nil {
		return errors.Wrapf(err, "failed to create erc20 contract")
	}
	bal, err := token.BalanceOf(&bind.CallOpts{
		Context:     ctx,
		BlockNumber: big.NewInt(int64(bc.BlockHeight)),
	}, bc.Address)
	if err != nil {
		return errors.Wrapf(err, "failed to get balance at %d", bc.BlockHeight)
	}
	if bal.Cmp(bc.Balance) != 0 {
		return errors.Errorf("balance mismatch: expected %s, got %s", bc.Balance.String(), bal.String())
	}
	return nil
}

func parseBalanceCaseFile(path string) ([]*BalanceCase, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	lines, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var cases []*BalanceCase
	for _, line := range lines[1:] { // Skip header line
		var c BalanceCase
		addr, err := addrutil.IoAddrToEvmAddr(line[0])
		if err != nil {
			return nil, err
		}
		c.Address = addr
		c.BlockHeight, err = strconv.Atoi(line[1])
		if err != nil {
			return nil, err
		}
		balance, ok := big.NewInt(0).SetString(line[2], 10)
		if !ok {
			return nil, errors.Errorf("failed to parse balance: %s", line[2])
		}
		c.Balance = balance
		cases = append(cases, &c)
	}
	return cases, nil
}

func parseContractCaseFile(path string) ([]*ContractBalanceCase, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	lines, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var cases []*ContractBalanceCase
	for _, line := range lines[1:] { // Skip header line
		var c ContractBalanceCase
		addr, err := addrutil.IoAddrToEvmAddr(line[0])
		if err != nil {
			return nil, err
		}
		c.ContractAddress = addr
		addr, err = addrutil.IoAddrToEvmAddr(line[1])
		if err != nil {
			return nil, err
		}
		c.Address = addr
		c.BlockHeight, _ = strconv.Atoi(line[2])
		balance, ok := big.NewInt(0).SetString(line[3], 10)
		if !ok {
			return nil, errors.Errorf("failed to parse balance: %s", line[2])
		}
		c.Balance = balance
		cases = append(cases, &c)
	}

	return cases, nil
}
