// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// exportblocks copies a range of blocks from a source BlockStore (FileDAO or
// gRPC BlockDAO) into a new WindowBlockStore database.
//
// Source selection:
//
//	-src-file  <path>     read from a local FileDAO chain.db
//	-src-grpc  <host:port> read from a remote gRPC BlockDAO service
//
// Block range selection (mutually exclusive with -latest):
//
//	-from <height>  first block to copy (default: 1)
//	-to   <height>  last  block to copy (default: source tip)
//
// Latest N blocks (mutually exclusive with -from/-to):
//
//	-latest <n>   copy the latest N blocks from the source
//
// Destination:
//
//	-out <path>          output WindowBlockStore database path (required)
//	-window <n>          window size for the output store (default: matches copied range)
//	-evm-network-id <n>  EVM network ID used for block deserialization (default: 4689)
//
// Example — copy last 256 blocks from a local chain.db:
//
//	exportblocks -src-file /var/data/chain.db -latest 256 -out /var/data/window.db
//
// Example — copy blocks 1000–2000 from a gRPC node:
//
//	exportblocks -src-grpc 192.168.1.1:14014 -from 1000 -to 2000 -out ./window.db
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

func main() {
	srcFile := flag.String("src-file", "", "Source: path to local FileDAO chain.db")
	srcGRPC := flag.String("src-grpc", "", "Source: gRPC BlockDAO endpoint (host:port)")
	grpcInsecure := flag.Bool("src-grpc-insecure", true, "Use insecure gRPC connection (no TLS)")
	fromHeight := flag.Uint64("from", 0, "First block height to copy (default: 1, or tip-latest+1 when -latest is set)")
	toHeight := flag.Uint64("to", 0, "Last block height to copy (default: source tip)")
	latest := flag.Uint64("latest", 0, "Copy the latest N blocks (overrides -from / -to)")
	outPath := flag.String("out", "", "Output WindowBlockStore database path (required)")
	windowSize := flag.Uint64("window", 0, "WindowBlockStore window size (default: number of blocks copied)")
	evmNetworkID := flag.Uint("evm-network-id", 4689, "EVM network ID for block deserialization")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: exportblocks -src-file|-src-grpc <source> -out <path> [-from N] [-to N] [-latest N] [-window N]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Validate flags.
	if *srcFile == "" && *srcGRPC == "" {
		fmt.Fprintln(os.Stderr, "error: one of -src-file or -src-grpc is required")
		flag.Usage()
		os.Exit(1)
	}
	if *srcFile != "" && *srcGRPC != "" {
		fmt.Fprintln(os.Stderr, "error: -src-file and -src-grpc are mutually exclusive")
		flag.Usage()
		os.Exit(1)
	}
	if *outPath == "" {
		fmt.Fprintln(os.Stderr, "error: -out is required")
		flag.Usage()
		os.Exit(1)
	}
	if *latest != 0 && (*fromHeight != 0 || *toHeight != 0) {
		fmt.Fprintln(os.Stderr, "error: -latest is mutually exclusive with -from / -to")
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()
	deser := block.NewDeserializer(uint32(*evmNetworkID))

	// Open source BlockStore.
	var src blockdao.BlockStore
	if *srcFile != "" {
		dbCfg := db.DefaultConfig
		dbCfg.DbPath = *srcFile
		dbCfg.ReadOnly = true
		fd, err := filedao.NewFileDAO(dbCfg, deser)
		if err != nil {
			log.S().Fatal("failed to open source FileDAO", zap.Error(err))
		}
		src = fd
	} else {
		src = blockdao.NewGrpcBlockDAO(*srcGRPC, *grpcInsecure, deser, 64)
	}

	if err := src.Start(ctx); err != nil {
		log.S().Fatal("failed to start source BlockStore", zap.Error(err))
	}
	defer src.Stop(ctx)

	// Resolve tip height from source.
	srcTip, err := src.Height()
	if err != nil {
		log.S().Fatal("failed to get source tip height", zap.Error(err))
	}
	if srcTip == 0 {
		fmt.Fprintln(os.Stderr, "error: source BlockStore is empty")
		os.Exit(1)
	}

	// Compute [from, to] range.
	from := *fromHeight
	to := *toHeight

	if *latest != 0 {
		if *latest > srcTip {
			from = 1
		} else {
			from = srcTip - *latest + 1
		}
		to = srcTip
	} else {
		if from == 0 {
			from = 1
		}
		if to == 0 {
			to = srcTip
		}
	}

	if from > to {
		fmt.Fprintf(os.Stderr, "error: from (%d) > to (%d)\n", from, to)
		os.Exit(1)
	}
	if to > srcTip {
		fmt.Fprintf(os.Stderr, "error: to (%d) exceeds source tip (%d)\n", to, srcTip)
		os.Exit(1)
	}

	count := to - from + 1

	// Default window size = number of blocks copied.
	ws := *windowSize
	if ws == 0 {
		ws = count
	}

	fmt.Printf("Source tip:  %d\n", srcTip)
	fmt.Printf("Copy range:  [%d, %d] (%d blocks)\n", from, to, count)
	fmt.Printf("Window size: %d\n", ws)
	fmt.Printf("Output:      %s\n\n", *outPath)

	// Open destination WindowBlockStore.
	dst, err := blockdao.NewWindowBlockStore(db.DefaultConfig, *outPath, ws, deser)
	if err != nil {
		log.S().Fatal("failed to create destination WindowBlockStore", zap.Error(err))
	}
	if err := dst.Start(ctx); err != nil {
		log.S().Fatal("failed to start destination WindowBlockStore", zap.Error(err))
	}
	defer dst.Stop(ctx)

	// Copy blocks in order.
	for h := from; h <= to; h++ {
		blk, err := src.GetBlockByHeight(h)
		if err != nil {
			log.S().Fatal("failed to get block from source", zap.Uint64("height", h), zap.Error(err))
		}

		// Attach receipts so they are stored in the destination.
		receipts, err := src.GetReceipts(h)
		if err != nil {
			// Non-fatal: some stores may not have receipts for all heights.
			log.S().Warn("failed to get receipts", zap.Uint64("height", h), zap.Error(err))
		} else {
			blk.Receipts = receipts
		}

		if err := dst.PutBlock(ctx, blk); err != nil {
			log.S().Fatal("failed to put block into destination", zap.Uint64("height", h), zap.Error(err))
		}

		if h%1000 == 0 || h == to {
			fmt.Printf("  copied %d / %d (height %d)\n", h-from+1, count, h)
		}
	}

	fmt.Printf("\nDone. Exported %d blocks to %s\n", count, *outPath)
}
