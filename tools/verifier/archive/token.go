package archive

import (
	"context"
	"fmt"
	"math/big"
	"os"

	"github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

var (
	cmdVerifyToken = &cobra.Command{
		Use:   "token",
		Short: "Verify the correctness of native token balance",
		RunE:  verifyToken,
	}
)

type tokenCase struct {
	addr         address.Address
	balance      string
	balanceDelta string
	height       uint64
}

func init() {
	CmdVerifyArchive.AddCommand(cmdVerifyToken)
}

func verifyToken(cmd *cobra.Command, args []string) error {
	minH, maxH, err := tokenBlockRange(db)
	if err != nil {
		return errors.Wrap(err, "failed to fetch block range")
	}
	fmt.Printf("token block range: %d - %d\n", minH, maxH)
	rev, err := readProgress()
	if err != nil {
		return errors.Wrap(err, "failed to read progress")
	}
	fmt.Printf("recover token progress start from %d\n", rev)
	minH = max(minH, rev)
	state := make(map[string]*big.Int)
	for start := minH; start <= maxH; start += batch {
		end := start + batch
		var (
			cases      []*tokenCase
			err        error
			verifyFunc = verifyOneToken
		)
		if deltaMode {
			cases, err = fetchTokenDeltaCases(db, start, end)
			verifyFunc = verifyOneTokenDelta
		} else {
			cases, err = fetchTokenCases(db, state, start, end)
		}
		if err != nil {
			return errors.Wrapf(err, "failed to fetch token cases from %d to %d", start, end)
		}
		if err := verifyCases(cases, verifyFunc); err != nil {
			return errors.Wrap(err, "failed to verify token cases")
		}
		if err := writeProgress(end); err != nil {
			return errors.Wrap(err, "failed to write token progress")
		}
	}
	return nil
}

func fetchTokenCases(db *gorm.DB, state map[string]*big.Int, from, to uint64) ([]*tokenCase, error) {
	type record struct {
		BlockHeight uint64 `gorm:"column:block_height"`
		Amount      string `gorm:"column:amount"`
		Sender      string `gorm:"column:sender"`
		Recipient   string `gorm:"column:recipient"`
	}
	records := make([]*record, 0)
	err := db.Table("block_receipt_transactions").Select("block_height", "amount", "sender", "recipient").Where("block_height >= ? AND block_height <= ?", from, to).Order("block_height, id").Find(&records).Error
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch token cases")
	}
	// log.S().Infof("fetched %d erc20 transfer records from %d to %d", len(records), from, to)
	cases := make([]*tokenCase, 0)
	if len(records) == 0 {
		return cases, nil
	}
	addrMap := make(map[string]struct{})
	lastHeight := records[0].BlockHeight
	genCasesAt := func(height uint64, addrMap map[string]struct{}) []*tokenCase {
		cases := make([]*tokenCase, 0)
		for addr := range addrMap {
			if _, ok := state[addr]; !ok {
				state[addr] = big.NewInt(0)
			}
			a, err := address.FromString(addr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to parse address %s: %s\n", addr, err.Error())
				continue
			}
			cases = append(cases, &tokenCase{
				addr:    a,
				balance: state[addr].String(),
				height:  height,
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
	fmt.Printf("generated %d token cases from %d to %d\n", len(cases), from, to)
	return cases, nil
}

func fetchTokenDeltaCases(db *gorm.DB, from, to uint64) ([]*tokenCase, error) {
	type record struct {
		BlockHeight uint64 `gorm:"column:block_height"`
		Amount      string `gorm:"column:amount"`
		Sender      string `gorm:"column:sender"`
		Recipient   string `gorm:"column:recipient"`
	}
	records := make([]*record, 0)
	err := db.Table("block_receipt_transactions").Select("block_height", "amount", "sender", "recipient").Where("block_height >= ? AND block_height <= ?", from, to).Order("block_height, id").Find(&records).Error
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch token cases")
	}
	// log.S().Infof("fetched %d erc20 transfer records from %d to %d", len(records), from, to)
	cases := make([]*tokenCase, 0)
	if len(records) == 0 {
		return cases, nil
	}
	addrMap := make(map[string]*big.Int)
	lastHeight := records[0].BlockHeight
	genCasesAt := func(height uint64, addrMap map[string]*big.Int) []*tokenCase {
		cases := make([]*tokenCase, 0)
		for addr := range addrMap {
			a, err := address.FromString(addr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to parse address %s: %s\n", addr, err.Error())
				continue
			}
			cases = append(cases, &tokenCase{
				addr:         a,
				balanceDelta: addrMap[addr].String(),
				height:       height,
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
	fmt.Printf("generated %d token cases from %d to %d\n", len(cases), from, to)
	return cases, nil
}

func verifyOneToken(c *tokenCase) (*mismatch, error) {
	ctx := context.Background()
	if address.IsAddrV1Special(c.addr.String()) {
		return nil, nil
	}
	addr := common.BytesToAddress(c.addr.Bytes())
	var (
		balance       *big.Int
		balanceLegacy *big.Int
		errWG         errgroup.Group
		retryMax      = 10
	)
	errWG.Go(func() error {
		return retry.Do(func() error {
			resp, err := ethcli.BalanceAt(ctx, addr, big.NewInt(int64(c.height+1)))
			if err != nil {
				return errors.Wrap(err, "failed to get balance")
			}
			balance = new(big.Int).SetBytes(resp.Bytes())
			return nil
		}, retry.Attempts(uint(retryMax)))
	})
	if ethcliLegacy != nil {
		errWG.Go(func() error {
			return retry.Do(func() error {
				resp, err := ethcliLegacy.BalanceAt(ctx, addr, big.NewInt(int64(c.height)))
				if err != nil {
					return errors.Wrap(err, "failed to get balance")
				}
				balanceLegacy = new(big.Int).SetBytes(resp.Bytes())
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

func verifyOneTokenDelta(c *tokenCase) (*mismatch, error) {
	ctx := context.Background()
	if address.IsAddrV1Special(c.addr.String()) {
		return nil, nil
	}
	addr := common.BytesToAddress(c.addr.Bytes())
	var (
		balance, prevBalance             *big.Int
		balanceLegacy, prevBalanceLegacy *big.Int
		errWG                            errgroup.Group
		retryMax                         = 10
	)
	errWG.Go(func() error {
		return retry.Do(func() error {
			resp, err := ethcli.BalanceAt(ctx, addr, big.NewInt(int64(c.height+1)))
			if err != nil {
				return errors.Wrap(err, "failed to get balance")
			}
			balance = new(big.Int).SetBytes(resp.Bytes())
			return nil
		}, retry.Attempts(uint(retryMax)))
	})
	errWG.Go(func() error {
		return retry.Do(func() error {
			resp, err := ethcli.BalanceAt(ctx, addr, big.NewInt(int64(c.height)))
			if err != nil {
				return errors.Wrap(err, "failed to get balance")
			}
			prevBalance = new(big.Int).SetBytes(resp.Bytes())
			return nil
		}, retry.Attempts(uint(retryMax)))
	})
	if ethcliLegacy != nil {
		errWG.Go(func() error {
			return retry.Do(func() error {
				resp, err := ethcliLegacy.BalanceAt(ctx, addr, big.NewInt(int64(c.height)))
				if err != nil {
					return errors.Wrap(err, "failed to get balance")
				}
				balanceLegacy = new(big.Int).SetBytes(resp.Bytes())
				return nil
			}, retry.Attempts(uint(retryMax)))
		})
		errWG.Go(func() error {
			return retry.Do(func() error {
				resp, err := ethcliLegacy.BalanceAt(ctx, addr, big.NewInt(int64(c.height-1)))
				if err != nil {
					return errors.Wrap(err, "failed to get balance")
				}
				prevBalanceLegacy = new(big.Int).SetBytes(resp.Bytes())
				return nil
			}, retry.Attempts(uint(retryMax)))
		})
	}
	if err := errWG.Wait(); err != nil {
		return nil, errors.Wrap(err, "failed to call contract")
	}
	delta := new(big.Int).Sub(balance, prevBalance)
	deltaLegacy := new(big.Int).Sub(balanceLegacy, prevBalanceLegacy)
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

func tokenBlockRange(db *gorm.DB) (uint64, uint64, error) {
	type result struct {
		Min uint64 `gorm:"min"`
		Max uint64 `gorm:"max"`
	}
	var r result
	err := db.Table("block_receipt_transactions").Select("MIN(block_height) as min, MAX(block_height) as max").Find(&r).Error
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to fetch block range")
	}
	return r.Min, r.Max, nil
}
