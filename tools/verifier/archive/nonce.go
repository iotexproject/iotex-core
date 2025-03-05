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
	cmdVerifyNonce = &cobra.Command{
		Use:   "nonce",
		Short: "Verify the correctness of account nonce",
		RunE:  verifyNonce,
	}
)

type nonceCase struct {
	addr   address.Address
	height uint64
	nonce  uint64
}

func init() {
	CmdVerifyArchive.AddCommand(cmdVerifyNonce)
}

func verifyNonce(cmd *cobra.Command, args []string) error {
	minH, maxH, err := nonceBlockRange(db)
	if err != nil {
		return errors.Wrap(err, "failed to fetch block range")
	}
	minH = max(minH, startHeight)
	maxH = min(maxH, endHeight)
	fmt.Printf("nonce block range: %d - %d\n", minH, maxH)
	rev, err := readProgress()
	if err != nil {
		return errors.Wrap(err, "failed to read progress")
	}
	fmt.Printf("recover nonce progress start from %d\n", rev)
	minH = max(minH, rev)
	for start := minH; start <= maxH; start += batch {
		end := start + batch

		cases, err := fetchNonceCases(db, start, end)
		if err != nil {
			return errors.Wrapf(err, "failed to nonce cases from %d to %d", start, end)
		}
		if err := verifyCases(cases, verifyOneNonce); err != nil {
			return errors.Wrap(err, "failed to verify nonce cases")
		}
		if err := writeProgress(end); err != nil {
			return errors.Wrap(err, "failed to write nonce progress")
		}
	}
	return nil
}

func fetchNonceCases(db *gorm.DB, from, to uint64) ([]*nonceCase, error) {
	type record struct {
		BlockHeight uint64 `gorm:"column:block_height"`
		Sender      string `gorm:"column:sender"`
		Nonce       uint64 `gorm:"column:nonce"`
		ActionType  string `gorm:"column:action_type"`
	}
	records := make([]*record, 0)
	err := db.Table("block_action").Select("block_height", "sender", "nonce", "action_type").Where("block_height >= ? AND block_height <= ?", from, to).Order("block_height, id").Find(&records).Error
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch nonce cases")
	}

	recordMap := map[string]*nonceCase{}
	for _, r := range records {
		if r.Nonce == 0 && (r.ActionType == "grantReward" || r.ActionType == "putPollResult") {
			continue
		}
		a, err := address.FromString(r.Sender)
		if err != nil {
			os.Stderr.WriteString(fmt.Sprintln("failed to parse address", "addr", r.Sender, "error", err))
			continue
		}
		recordMap[fmt.Sprintf("%s-%d", r.Sender, r.BlockHeight)] = &nonceCase{
			addr:   a,
			height: r.BlockHeight,
			nonce:  r.Nonce,
		}
	}

	cases := make([]*nonceCase, 0, len(recordMap))
	for _, r := range recordMap {
		cases = append(cases, r)
	}

	fmt.Println("fetched nonce cases", "from", from, "to", to, "amount", len(cases))
	return cases, nil
}

func verifyOneNonce(c *nonceCase) (*mismatch, error) {
	ctx := context.Background()
	if address.IsAddrV1Special(c.addr.String()) {
		return nil, nil
	}
	addr := common.BytesToAddress(c.addr.Bytes())
	var (
		nonce       uint64
		nonceLegacy uint64
		errWG       errgroup.Group
		retryMax    = 10
	)
	errWG.Go(func() error {
		return retry.Do(func() error {
			resp, err := ethcli.NonceAt(ctx, addr, big.NewInt(int64(c.height)))
			if err != nil {
				return errors.Wrap(err, "failed to get nonce")
			}
			nonce = resp
			return nil
		}, retry.Attempts(uint(retryMax)))
	})
	if ethcliLegacy != nil {
		errWG.Go(func() error {
			return retry.Do(func() error {
				resp, err := ethcliLegacy.NonceAt(ctx, addr, big.NewInt(int64(c.height)))
				if err != nil {
					return errors.Wrap(err, "failed to get nonce legacy")
				}
				nonceLegacy = resp
				return nil
			}, retry.Attempts(uint(retryMax)))
		})
	}
	if err := errWG.Wait(); err != nil {
		return nil, errors.Wrap(err, "failed to call chain")
	}
	if ethcliLegacy != nil {
		if nonceLegacy != c.nonce+1 || nonce != c.nonce+1 {
			return &mismatch{
				isNonce:      true,
				height:       c.height,
				address:      c.addr.String(),
				nonceAv2:     c.nonce,
				nonceErigon:  nonce,
				nonceArchive: nonceLegacy,
			}, nil
		}
	} else if nonce != c.nonce+1 {
		return &mismatch{
			isNonce:      true,
			height:       c.height,
			address:      c.addr.String(),
			nonceAv2:     c.nonce,
			nonceErigon:  nonce,
			nonceArchive: 0,
		}, nil
	}
	return nil, nil
}

func nonceBlockRange(db *gorm.DB) (uint64, uint64, error) {
	type result struct {
		Min uint64 `gorm:"min"`
		Max uint64 `gorm:"max"`
	}
	var r result
	err := db.Table("block_action").Select("MIN(block_height) as min, MAX(block_height) as max").Find(&r).Error
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to fetch block range")
	}
	return r.Min, r.Max, nil
}
