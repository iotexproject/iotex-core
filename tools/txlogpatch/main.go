// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// txlogpatch generates a transaction-log patch BoltDB that overrides the
// IN_CONTRACT_TRANSFER records served for specific block heights. A node loads the
// produced file via chain.patchTransactionLogPath and serves the corrected transaction
// logs for those heights. Transaction logs are not part of any receipt or state root,
// so this does not affect consensus.
//
// Two modes:
//
//   - strip (default): removes forged records. CSV columns (amount in IOTX):
//     block_height,tx_hash,sender,recipient,amount_IOTX
//     go run ./tools/txlogpatch -csv FORGERY_FINAL.csv -out txlog.db.patch
//
//   - correct (-correct): rewrites a record's amount to the correct value, keeping the
//     record. Used for the SELFDESTRUCT log amount corruption. CSV columns (RAU):
//     block_height,tx_hash,sender,recipient,wrong_amount_rau,correct_amount_rau
//     go run ./tools/txlogpatch -correct -csv SELFDESTRUCT_FIX.csv -out txlog.db.patch
package main

import (
	"context"
	"crypto/tls"
	"encoding/csv"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/db"
)

type patchRec struct {
	height    uint64
	actHash   string // hex, no 0x
	sender    string
	recipient string
	amountRau string // amount to match in the current log
	newRau    string // replacement amount (correct mode only)
}

func main() {
	csvPath := flag.String("csv", "FORGERY_FINAL.csv", "input CSV (see mode docs)")
	endpoint := flag.String("endpoint", "api.mainnet.iotex.one:443", "iotex gRPC API endpoint")
	secure := flag.Bool("secure", true, "use TLS for the gRPC endpoint")
	outPath := flag.String("out", "txlog.db.patch", "output patch BoltDB path")
	correct := flag.Bool("correct", false, "correct-amount mode: rewrite (not remove) a record's amount; CSV carries a 6th correct_amount_rau column")
	flag.Parse()

	recs, err := readCSV(*csvPath, *correct)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("loaded %d record(s) [%s mode]\n", len(recs), mode(*correct))

	dialCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var conn *grpc.ClientConn
	if *secure {
		conn, err = grpc.DialContext(dialCtx, *endpoint, grpc.WithBlock(),
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
	} else {
		conn, err = grpc.DialContext(dialCtx, *endpoint, grpc.WithBlock(),
			grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if err != nil {
		fatal(fmt.Errorf("dial %s: %w", *endpoint, err))
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)

	dbCfg := db.DefaultConfig
	dbCfg.DbPath = *outPath
	dbCfg.ReadOnly = false
	ti := blockdao.NewTransactionLogIndexer(db.NewBoltDB(dbCfg))
	if err := ti.Start(context.Background()); err != nil {
		fatal(err)
	}
	defer ti.Stop(context.Background())

	byHeight := map[uint64][]patchRec{}
	for _, f := range recs {
		byHeight[f.height] = append(byHeight[f.height], f)
	}

	for height, hr := range byHeight {
		resp, err := cli.GetTransactionLogByBlockHeight(context.Background(),
			&iotexapi.GetTransactionLogByBlockHeightRequest{BlockHeight: height})
		if err != nil {
			fatal(fmt.Errorf("get txlogs for height %d: %w", height, err))
		}
		logs := resp.GetTransactionLogs()
		before := countTx(logs)
		out, n := apply(logs, hr, *correct)
		if n != len(hr) {
			fatal(fmt.Errorf("height %d: expected to match %d record(s), matched %d — refusing to write an incorrect patch", height, len(hr), n))
		}
		if err := ti.Put(height, out); err != nil {
			fatal(fmt.Errorf("put height %d: %w", height, err))
		}
		fmt.Printf("height %d: transactions %d -> %d (%s %d)\n", height, before, countTx(out), mode(*correct), n)
	}
	fmt.Printf("OK: patch written to %s (%d block(s))\n", *outPath, len(byHeight))
}

// apply rewrites (correct mode) or removes (strip mode) the matching
// IN_CONTRACT_TRANSFER records and returns the resulting logs plus the number matched.
// Every other record (GAS_FEE, PRIORITY_FEE, real transfers, other actions) is preserved
// unchanged. In strip mode a log whose transactions all get removed is dropped.
func apply(logs *iotextypes.TransactionLogs, recs []patchRec, correct bool) (*iotextypes.TransactionLogs, int) {
	out := &iotextypes.TransactionLogs{}
	matched := 0
	for _, lg := range logs.GetLogs() {
		ah := hex.EncodeToString(lg.GetActionHash())
		kept := make([]*iotextypes.TransactionLog_Transaction, 0, len(lg.GetTransactions()))
		for _, tx := range lg.GetTransactions() {
			var hit *patchRec
			for i := range recs {
				f := &recs[i]
				if f.actHash == ah &&
					tx.GetType() == iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER &&
					tx.GetSender() == f.sender &&
					tx.GetRecipient() == f.recipient &&
					tx.GetAmount() == f.amountRau {
					hit = f
					break
				}
			}
			if hit == nil {
				kept = append(kept, tx)
				continue
			}
			matched++
			if correct {
				tx.Amount = hit.newRau // rewrite the amount, keep the record
				kept = append(kept, tx)
			}
			// strip mode: drop the record (do not append)
		}
		if len(kept) == 0 {
			continue
		}
		out.Logs = append(out.Logs, &iotextypes.TransactionLog{
			ActionHash:      lg.GetActionHash(),
			NumTransactions: uint64(len(kept)),
			Transactions:    kept,
		})
	}
	return out, matched
}

func countTx(logs *iotextypes.TransactionLogs) int {
	n := 0
	for _, lg := range logs.GetLogs() {
		n += len(lg.GetTransactions())
	}
	return n
}

func readCSV(path string, correct bool) ([]patchRec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	need := 5
	if correct {
		need = 6
	}
	var out []patchRec
	oneIOTX := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	for i, r := range rows {
		if i == 0 && strings.HasPrefix(r[0], "block") { // header
			continue
		}
		if len(r) < need {
			return nil, fmt.Errorf("row %d: expected %d columns, got %d", i, need, len(r))
		}
		var h uint64
		if _, err := fmt.Sscan(r[0], &h); err != nil {
			return nil, fmt.Errorf("row %d: bad height %q: %w", i, r[0], err)
		}
		rec := patchRec{
			height:    h,
			actHash:   strings.TrimPrefix(strings.TrimSpace(r[1]), "0x"),
			sender:    strings.TrimSpace(r[2]),
			recipient: strings.TrimSpace(r[3]),
		}
		if correct {
			// amounts already in RAU
			for _, c := range []string{r[4], r[5]} {
				if _, ok := new(big.Int).SetString(strings.TrimSpace(c), 10); !ok {
					return nil, fmt.Errorf("row %d: bad RAU amount %q", i, c)
				}
			}
			rec.amountRau = strings.TrimSpace(r[4])
			rec.newRau = strings.TrimSpace(r[5])
		} else {
			iotx, ok := new(big.Int).SetString(strings.TrimSpace(r[4]), 10)
			if !ok {
				return nil, fmt.Errorf("row %d: bad amount %q", i, r[4])
			}
			rec.amountRau = new(big.Int).Mul(iotx, oneIOTX).String()
		}
		out = append(out, rec)
	}
	return out, nil
}

func mode(correct bool) string {
	if correct {
		return "correct"
	}
	return "strip"
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
