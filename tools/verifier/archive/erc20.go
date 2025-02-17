package archive

import (
	"context"
	_ "embed"
	"fmt"
	"math/big"
	"strings"

	"github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/iotexproject/iotex-address/address"
)

type erc20Case struct {
	contract     address.Address
	addr         address.Address
	balance      string
	height       uint64
	balanceDelta string
}

var (
	cmdVerifyArchiveERC20 = &cobra.Command{
		Use:   "erc20",
		Short: "Verify the correctness of archive data of erc20 token",
		RunE:  runVerifyERC20,
	}

	//go:embed erc20_abi.json
	erc20ABIJSON string
	erc20ABI     abi.ABI
)

func init() {
	flag := cmdVerifyArchiveERC20.Flags()
	flag.StringSliceVar(&contracts, "contracts", []string{
		"0x6fbcdc1169b5130c59e72e51ed68a84841c98cd1", // ioUSDT
	}, "the contract addresses to verify, each format with 0x|io prefix")
	abi, err := abi.JSON(strings.NewReader(erc20ABIJSON))
	if err != nil {
		panic(err)
	}
	erc20ABI = abi
	CmdVerifyArchive.AddCommand(cmdVerifyArchiveERC20)
}

func runVerifyERC20(cmd *cobra.Command, args []string) error {
	for _, contract := range contracts {
		contract, err := formatAddress(contract)
		if err != nil {
			return errors.Wrap(err, "failed to format contract address")
		}
		if err := verifyERC20(contract); err != nil {
			return errors.Wrapf(err, "failed to verify contract %s", contract)
		}
	}
	return nil
}

func verifyERC20(contract address.Address) error {
	// fetch account addresses balance which erc20 balance have been changed via av2
	minH, maxH, err := contractBlockRange(db, contract.String())
	if err != nil {
		return errors.Wrap(err, "failed to fetch block range")
	}
	fmt.Printf("contract %s block range: %d - %d\n", contract.Hex(), minH, maxH)
	rev, err := readProgress()
	if err != nil {
		return errors.Wrap(err, "failed to read progress")
	}
	fmt.Printf("recover progress start from %d\n", rev)
	minH = max(minH, rev)
	state := make(map[string]*big.Int)
	for start := minH; start <= maxH; start += batch {
		end := start + batch
		var (
			cases      []*erc20Case
			err        error
			verifyFunc = verifyOne
		)
		if deltaMode {
			cases, err = fetchERC20DeltaCases(db, contract, start, end)
			verifyFunc = verifyOneDelta
		} else {
			cases, err = fetchERC20Cases(db, contract, state, start, end)
		}
		if err != nil {
			return errors.Wrapf(err, "failed to fetch erc20 cases from %d to %d", start, end)
		}

		if err := verifyCases(cases, verifyFunc); err != nil {
			return errors.Wrap(err, "failed to verify erc20 cases")
		}
		if err := writeProgress(end); err != nil {
			return errors.Wrap(err, "failed to write progress")
		}
	}
	return nil
}

func contractBlockRange(db *gorm.DB, contract string) (uint64, uint64, error) {
	type result struct {
		Min uint64 `gorm:"min"`
		Max uint64 `gorm:"max"`
	}
	var r result
	err := db.Table("erc20_transfers").Select("MIN(block_height) as min, MAX(block_height) as max").Where("contract_address = ?", contract).Find(&r).Error
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to fetch block range")
	}
	return r.Min, r.Max, nil
}

func fetchERC20Cases(db *gorm.DB, contract address.Address, state map[string]*big.Int, from, to uint64) ([]*erc20Case, error) {
	type record struct {
		BlockHeight uint64 `gorm:"column:block_height"`
		Amount      string `gorm:"column:amount"`
		Sender      string `gorm:"column:sender"`
		Recipient   string `gorm:"column:recipient"`
	}
	records := make([]*record, 0)
	err := db.Table("erc20_transfers").Select("block_height", "amount", "sender", "recipient").Where("block_height >= ? AND block_height <= ? AND contract_address = ?", from, to, contract.String()).Order("block_height, id").Find(&records).Error
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch erc20 cases")
	}
	// log.S().Infof("fetched %d erc20 transfer records from %d to %d", len(records), from, to)
	cases := make([]*erc20Case, 0)
	if len(records) == 0 {
		return cases, nil
	}
	addrMap := make(map[string]struct{})
	lastHeight := records[0].BlockHeight
	genCasesAt := func(height uint64, addrMap map[string]struct{}) []*erc20Case {
		cases := make([]*erc20Case, 0)
		for addr := range addrMap {
			if _, ok := state[addr]; !ok {
				state[addr] = big.NewInt(0)
			}
			a, _ := address.FromString(addr)
			cases = append(cases, &erc20Case{
				addr:     a,
				balance:  state[addr].String(),
				height:   height,
				contract: contract,
			})
		}
		return cases
	}
	for _, r := range records {
		if r.BlockHeight != lastHeight {
			cases = append(cases, genCasesAt(lastHeight, addrMap)...)
			addrMap = make(map[string]struct{})
			lastHeight = r.BlockHeight
		}
		// log.S().Infof("block %d: %s -> %s: %s", r.BlockHeight, r.Sender, r.Recipient, r.Amount)
		if _, ok := state[r.Sender]; !ok {
			state[r.Sender] = big.NewInt(0)
		}
		if _, ok := state[r.Recipient]; !ok {
			state[r.Recipient] = big.NewInt(0)
		}
		amount := new(big.Int)
		_, ok := amount.SetString(r.Amount, 10)
		if !ok {
			return nil, errors.Errorf("failed to parse amount %s", r.Amount)
		}
		state[r.Sender].Sub(state[r.Sender], amount)
		state[r.Recipient].Add(state[r.Recipient], amount)
		addrMap[r.Sender] = struct{}{}
		addrMap[r.Recipient] = struct{}{}
	}
	cases = append(cases, genCasesAt(lastHeight, addrMap)...)
	fmt.Printf("generated %d erc20 cases from %d to %d\n", len(cases), from, to)
	return cases, nil
}

func fetchERC20DeltaCases(db *gorm.DB, contract address.Address, from, to uint64) ([]*erc20Case, error) {
	type record struct {
		BlockHeight uint64 `gorm:"column:block_height"`
		Amount      string `gorm:"column:amount"`
		Sender      string `gorm:"column:sender"`
		Recipient   string `gorm:"column:recipient"`
	}
	records := make([]*record, 0)
	err := db.Table("erc20_transfers").Select("block_height", "amount", "sender", "recipient").Where("block_height >= ? AND block_height <= ? AND contract_address = ?", from, to, contract.String()).Order("block_height, id").Find(&records).Error
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch erc20 cases")
	}
	// log.S().Infof("fetched %d erc20 transfer records from %d to %d", len(records), from, to)
	cases := make([]*erc20Case, 0)
	if len(records) == 0 {
		return cases, nil
	}
	addrMap := make(map[string]*big.Int)
	lastHeight := records[0].BlockHeight
	genCasesAt := func(height uint64, addrMap map[string]*big.Int) []*erc20Case {
		cases := make([]*erc20Case, 0)
		for addr := range addrMap {
			a, _ := address.FromString(addr)
			cases = append(cases, &erc20Case{
				addr:         a,
				balanceDelta: addrMap[addr].String(),
				height:       height,
				contract:     contract,
			})
		}
		return cases
	}
	for _, r := range records {
		if r.BlockHeight != lastHeight {
			cases = append(cases, genCasesAt(lastHeight, addrMap)...)
			addrMap = make(map[string]*big.Int)
			lastHeight = r.BlockHeight
		}
		// log.S().Infof("block %d: %s -> %s: %s", r.BlockHeight, r.Sender, r.Recipient, r.Amount)
		if _, ok := addrMap[r.Sender]; !ok {
			addrMap[r.Sender] = big.NewInt(0)
		}
		if _, ok := addrMap[r.Recipient]; !ok {
			addrMap[r.Recipient] = big.NewInt(0)
		}
		amount := new(big.Int)
		_, ok := amount.SetString(r.Amount, 10)
		if !ok {
			return nil, errors.Errorf("failed to parse amount %s", r.Amount)
		}
		addrMap[r.Sender].Sub(addrMap[r.Sender], amount)
		addrMap[r.Recipient].Add(addrMap[r.Recipient], amount)
	}
	cases = append(cases, genCasesAt(lastHeight, addrMap)...)
	fmt.Printf("generated %d erc20 cases from %d to %d\n", len(cases), from, to)
	return cases, nil
}

func verifyOne(c *erc20Case) (*mismatch, error) {
	ctx := context.Background()
	to := common.BytesToAddress(c.contract.Bytes())
	addr := common.BytesToAddress(c.addr.Bytes())
	data, err := abiCall(erc20ABI, "balanceOf(address)", addr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack abi call")
	}
	var (
		balance       *big.Int
		balanceLegacy *big.Int
		errWG         errgroup.Group
		retryMax      = 10
	)
	errWG.Go(func() error {
		return retry.Do(func() error {
			resp, err := ethcli.CallContract(ctx, ethereum.CallMsg{
				To:   &to,
				Data: data,
			}, big.NewInt(int64(c.height+1)))
			if err != nil {
				return errors.Wrap(err, "failed to call contract")
			}
			balance = new(big.Int).SetBytes(resp)
			return nil
		}, retry.Attempts(uint(retryMax)))
	})
	if ethcliLegacy != nil {
		errWG.Go(func() error {
			return retry.Do(func() error {
				resp, err := ethcliLegacy.CallContract(ctx, ethereum.CallMsg{
					To:   &to,
					Data: data,
				}, big.NewInt(int64(c.height)))
				if err != nil {
					return errors.Wrap(err, "failed to call contract")
				}
				balanceLegacy = new(big.Int).SetBytes(resp)
				return nil
			}, retry.Attempts(uint(retryMax)))
		})
	}
	if err := errWG.Wait(); err != nil {
		return nil, errors.Wrap(err, "failed to call contract")
	}
	if ethcliLegacy != nil {
		if balanceLegacy.String() != c.balance || balance.String() != c.balance {
			return &mismatch{
				height:         c.height,
				address:        c.addr.String(),
				balanceAv2:     c.balance,
				balanceErigon:  balance.String(),
				balanceArchive: balanceLegacy.String(),
			}, nil
		}
	} else if balance.String() != c.balance {
		return &mismatch{
			height:         c.height,
			address:        c.addr.String(),
			balanceAv2:     c.balance,
			balanceErigon:  balance.String(),
			balanceArchive: "",
		}, nil
	}
	return nil, nil
}

func verifyOneDelta(c *erc20Case) (*mismatch, error) {
	ctx := context.Background()
	to := common.BytesToAddress(c.contract.Bytes())
	addr := common.BytesToAddress(c.addr.Bytes())
	data, err := abiCall(erc20ABI, "balanceOf(address)", addr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to pack abi call")
	}
	var (
		balance           *big.Int
		prevBalance       *big.Int
		balanceLegacy     *big.Int
		prevBalanceLegacy *big.Int
		errWG             errgroup.Group
		retryMax          = 10
	)
	errWG.Go(func() error {
		return retry.Do(func() error {
			resp, err := ethcli.CallContract(ctx, ethereum.CallMsg{
				To:   &to,
				Data: data,
			}, big.NewInt(int64(c.height+1)))
			if err != nil {
				return errors.Wrap(err, "failed to call contract")
			}
			balance = new(big.Int).SetBytes(resp)
			return nil
		}, retry.Attempts(uint(retryMax)))
	})
	errWG.Go(func() error {
		return retry.Do(func() error {
			resp, err := ethcli.CallContract(ctx, ethereum.CallMsg{
				To:   &to,
				Data: data,
			}, big.NewInt(int64(c.height)))
			if err != nil {
				return errors.Wrap(err, "failed to call contract")
			}
			prevBalance = new(big.Int).SetBytes(resp)
			return nil
		}, retry.Attempts(uint(retryMax)))
	})
	if ethcliLegacy != nil {
		errWG.Go(func() error {
			return retry.Do(func() error {
				resp, err := ethcliLegacy.CallContract(ctx, ethereum.CallMsg{
					To:   &to,
					Data: data,
				}, big.NewInt(int64(c.height)))
				if err != nil {
					return errors.Wrap(err, "failed to call contract")
				}
				balanceLegacy = new(big.Int).SetBytes(resp)
				return nil
			}, retry.Attempts(uint(retryMax)))
		})
		errWG.Go(func() error {
			return retry.Do(func() error {
				resp, err := ethcliLegacy.CallContract(ctx, ethereum.CallMsg{
					To:   &to,
					Data: data,
				}, big.NewInt(int64(c.height-1)))
				if err != nil {
					return errors.Wrap(err, "failed to call contract")
				}
				prevBalanceLegacy = new(big.Int).SetBytes(resp)
				return nil
			}, retry.Attempts(uint(retryMax)))
		})
	}
	if err := errWG.Wait(); err != nil {
		return nil, errors.Wrap(err, "failed to call contract")
	}
	deltaLegacy := new(big.Int).Sub(prevBalanceLegacy, balanceLegacy)
	delta := new(big.Int).Sub(prevBalance, balance)
	if ethcliLegacy != nil {
		if deltaLegacy.String() != c.balanceDelta || delta.String() != c.balanceDelta {
			return &mismatch{
				height:         c.height,
				address:        c.addr.String(),
				balanceAv2:     c.balanceDelta,
				balanceErigon:  delta.String(),
				balanceArchive: deltaLegacy.String(),
			}, nil
		}
	} else if delta.String() != c.balanceDelta {
		return &mismatch{
			height:         c.height,
			address:        c.addr.String(),
			balanceAv2:     c.balanceDelta,
			balanceErigon:  delta.String(),
			balanceArchive: "",
		}, nil
	}
	return nil, nil
}
