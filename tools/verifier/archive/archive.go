package archive

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type mismatch struct {
	height         uint64
	address        string
	balanceAv2     string
	balanceErigon  string
	balanceArchive string
	isNonce        bool
	nonceAv2       uint64
	nonceErigon    uint64
	nonceArchive   uint64
}

var (
	CmdVerifyArchive = &cobra.Command{
		Use:   "archive",
		Short: "Verify the correctness of archive data",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := initProgress()
			if err != nil {
				return err
			}
			db, err = connectAv2(dsn)
			if err != nil {
				return err
			}
			return connectRPC()
		},
	}

	// params
	dsn         string
	rpc         string
	legacyRPC   string
	contracts   []string
	batch       uint64
	workerNum   int
	deltaMode   bool
	startHeight uint64
	endHeight   uint64

	// global variables
	db               *gorm.DB
	ethcli           *ethclient.Client
	ethcliLegacy     *ethclient.Client
	progressFile     *os.File
	progressFilePath string
)

func init() {
	flag := CmdVerifyArchive.PersistentFlags()
	flag.StringVar(&dsn, "dsn", "", "the pgdsn of av2")
	flag.StringVar(&rpc, "rpc", "http://iotex-erigon:15014", "the rpc endpoint of iotex blockchain")
	flag.StringVar(&legacyRPC, "rpc-legacy", "https://archive-mainnet.iotex.io", "the legacy archive rpc endpoint of iotex blockchain")
	flag.Uint64Var(&batch, "batch", 10000, "the batch block size of fetching erc20 transfer records")
	flag.IntVar(&workerNum, "workers", 8, "the number of workers to verify erc20 balance")
	flag.StringVar(&progressFilePath, "progress", "progress.dat", "the file to store progress")
	flag.BoolVar(&deltaMode, "delta", false, "verify delta mode")
	flag.Uint64Var(&startHeight, "start", 0, "the start height to verify")
	flag.Uint64Var(&endHeight, "end", math.MaxUint64, "the end height to verify")
}

func connectAv2(dsn string) (*gorm.DB, error) {
	gormConfig := &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   false,
		PrepareStmt:                              true,
		Logger:                                   logger.Discard,
	}
	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return db, errors.Wrap(err, "failed to connect to av2")
	}
	fmt.Printf("connected to av2 %s\n", dsn)
	return db, nil
}

func connectRPC() error {
	var err error
	ethcli, err = ethclient.Dial(rpc)
	if err != nil {
		return errors.Wrapf(err, "failed to connect to eth client %s", rpc)
	}
	fmt.Printf("connected to eth client %s\n", rpc)
	if len(legacyRPC) > 0 {
		ethcliLegacy, err = ethclient.Dial(legacyRPC)
		if err != nil {
			return errors.Wrapf(err, "failed to connect to eth client %s", legacyRPC)
		}
		fmt.Printf("connected to eth client %s\n", legacyRPC)
	}
	return nil
}

func verifyCases[T any](cases []T, verifyFunc func(T) (*mismatch, error)) error {
	bar := progressbar.NewOptions(len(cases),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
	)
	bar.RenderBlank()
	taskQ := make(chan T, 1000)
	resultQ := make(chan *mismatch, 1000)
	finished := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(workerNum)
	for i := 0; i < workerNum; i++ {
		// worker to verify erc20 balance
		go func() {
			defer wg.Done()
			for c := range taskQ {
				m, err := verifyFunc(c)
				if err != nil {
					os.Stderr.WriteString(fmt.Sprintf("failed to verify case: %s\n", err.Error()))
				}
				bar.Add(1)
				if m != nil {
					resultQ <- m
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(resultQ)
	}()
	go func() {
		for m := range resultQ {
			if m != nil {
				if m.isNonce {
					os.Stderr.WriteString(fmt.Sprintln("nonce mismatch", "av2", m.nonceAv2, "archive", m.nonceArchive, "erigon", m.nonceErigon, "block_height", m.height, "account", m.address))
				} else {
					if m.balanceErigon != m.balanceArchive {
						os.Stderr.WriteString(fmt.Sprintf("balance mismatch_archive: %s: %s, av2 %s, archive %s at %d height\n", m.address, m.balanceErigon, m.balanceAv2, m.balanceArchive, m.height))
					}
					os.Stderr.WriteString(fmt.Sprintf("balance mismatch_av2: %s: %s, av2 %s, archive %s at %d height\n", m.address, m.balanceErigon, m.balanceAv2, m.balanceArchive, m.height))
				}
			}
		}
		finished <- struct{}{}
	}()
	for _, c := range cases {
		taskQ <- c
	}
	close(taskQ)
	<-finished
	bar.Finish()
	fmt.Println()
	return nil
}

func formatAddress(addr string) (address.Address, error) {
	if addr[:2] == "io" {
		a, err := address.FromString(addr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse address %s as io-format", addr)
		}
		return a, nil
	} else {
		a, err := address.FromHex(addr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse address %s as 0x-format", addr)
		}
		return a, nil
	}
}

func methodSignToID(sign string) []byte {
	hash := crypto.Keccak256Hash([]byte(sign))
	return hash.Bytes()[:4]
}

func abiCall(_abi abi.ABI, methodSign string, args ...interface{}) ([]byte, error) {
	if methodSign == "" {
		return _abi.Pack("", args...)
	}
	m, err := _abi.MethodById(methodSignToID(methodSign))
	if err != nil {
		return nil, err
	}
	return _abi.Pack(m.Name, args...)
}

func initProgress() error {
	var err error
	progressFile, err = os.OpenFile(progressFilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to open progress file")
	}
	return nil
}

func readProgress() (uint64, error) {
	if progressFile == nil {
		if err := initProgress(); err != nil {
			return 0, err
		}
	}
	if _, err := progressFile.Seek(0, io.SeekStart); err != nil {
		return 0, errors.Wrap(err, "failed to seek progress file")
	}
	data, err := io.ReadAll(progressFile)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read progress file")
	}
	if len(data) == 0 {
		return 0, nil
	}
	progress, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse progress value")
	}
	return progress, nil
}

func writeProgress(height uint64) error {
	if progressFile == nil {
		if err := initProgress(); err != nil {
			return err
		}
	}
	data := []byte(fmt.Sprintf("%d", height))
	if err := progressFile.Truncate(0); err != nil {
		return errors.Wrap(err, "failed to truncate progress file")
	}
	if _, err := progressFile.Seek(0, io.SeekStart); err != nil {
		return errors.Wrap(err, "failed to seek progress file")
	}
	if _, err := progressFile.Write(data); err != nil {
		return errors.Wrap(err, "failed to write progress file")
	}
	return nil
}
