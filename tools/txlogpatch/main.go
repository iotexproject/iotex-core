// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// txlogpatch generates a transaction-log patch BoltDB that removes a set of forged
// IN_CONTRACT_TRANSFER records from specific block heights. A node loads the produced
// file via chain.patchTransactionLogPath / chain.patchTransactionLogEndHeight and then
// serves the corrected transaction logs for those heights. Transaction logs are not part
// of any receipt or state root, so this does not affect consensus.
//
// Usage:
//
//	go run ./tools/txlogpatch \
//	  -csv FORGERY_FINAL.csv -endpoint api.mainnet.iotex.one:443 -out txlog.db.patch
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

type forgedRec struct {
	height    uint64
	actHash   string // hex, no 0x
	sender    string
	recipient string
	amountRau string
}

func main() {
	csvPath := flag.String("csv", "FORGERY_FINAL.csv", "CSV: block_height,tx_hash,attacker_sender,victim_recipient,amount_IOTX")
	endpoint := flag.String("endpoint", "api.mainnet.iotex.one:443", "iotex gRPC API endpoint")
	secure := flag.Bool("secure", true, "use TLS for the gRPC endpoint")
	outPath := flag.String("out", "txlog.db.patch", "output patch BoltDB path")
	flag.Parse()

	forged, err := readForged(*csvPath)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("loaded %d forged records\n", len(forged))

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

	byHeight := map[uint64][]forgedRec{}
	for _, f := range forged {
		byHeight[f.height] = append(byHeight[f.height], f)
	}

	for height, recs := range byHeight {
		resp, err := cli.GetTransactionLogByBlockHeight(context.Background(),
			&iotexapi.GetTransactionLogByBlockHeightRequest{BlockHeight: height})
		if err != nil {
			fatal(fmt.Errorf("get txlogs for height %d: %w", height, err))
		}
		logs := resp.GetTransactionLogs()
		before := countTx(logs)
		corrected, removed := strip(logs, recs)
		if removed != len(recs) {
			fatal(fmt.Errorf("height %d: expected to remove %d forged record(s), removed %d — refusing to write an incorrect patch", height, len(recs), removed))
		}
		if err := ti.Put(height, corrected); err != nil {
			fatal(fmt.Errorf("put height %d: %w", height, err))
		}
		fmt.Printf("height %d: transactions %d -> %d (removed %d forged)\n", height, before, countTx(corrected), removed)
	}
	fmt.Printf("OK: patch written to %s (%d block(s))\n", *outPath, len(byHeight))
}

// strip returns a copy of logs with the forged IN_CONTRACT_TRANSFER records removed,
// and the count of records actually removed. Non-forged records (GAS_FEE, PRIORITY_FEE,
// real transfers, other actions) are preserved. A log entry whose transactions become
// empty is dropped.
func strip(logs *iotextypes.TransactionLogs, recs []forgedRec) (*iotextypes.TransactionLogs, int) {
	out := &iotextypes.TransactionLogs{}
	removed := 0
	for _, lg := range logs.GetLogs() {
		ah := hex.EncodeToString(lg.GetActionHash())
		kept := make([]*iotextypes.TransactionLog_Transaction, 0, len(lg.GetTransactions()))
		for _, tx := range lg.GetTransactions() {
			isForged := false
			for _, f := range recs {
				if f.actHash == ah &&
					tx.GetType() == iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER &&
					tx.GetSender() == f.sender &&
					tx.GetRecipient() == f.recipient &&
					tx.GetAmount() == f.amountRau {
					isForged = true
					removed++
					break
				}
			}
			if !isForged {
				kept = append(kept, tx)
			}
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
	return out, removed
}

func countTx(logs *iotextypes.TransactionLogs) int {
	n := 0
	for _, lg := range logs.GetLogs() {
		n += len(lg.GetTransactions())
	}
	return n
}

func readForged(path string) ([]forgedRec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	var out []forgedRec
	oneIOTX := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	for i, r := range rows {
		if i == 0 && strings.HasPrefix(r[0], "block") { // header
			continue
		}
		if len(r) < 5 {
			return nil, fmt.Errorf("row %d: expected 5 columns, got %d", i, len(r))
		}
		var h uint64
		if _, err := fmt.Sscan(r[0], &h); err != nil {
			return nil, fmt.Errorf("row %d: bad height %q: %w", i, r[0], err)
		}
		iotx, ok := new(big.Int).SetString(strings.TrimSpace(r[4]), 10)
		if !ok {
			return nil, fmt.Errorf("row %d: bad amount %q", i, r[4])
		}
		out = append(out, forgedRec{
			height:    h,
			actHash:   strings.TrimPrefix(strings.TrimSpace(r[1]), "0x"),
			sender:    strings.TrimSpace(r[2]),
			recipient: strings.TrimSpace(r[3]),
			amountRau: new(big.Int).Mul(iotx, oneIOTX).String(),
		})
	}
	return out, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
