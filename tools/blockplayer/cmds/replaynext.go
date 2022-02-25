package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/pprof"

	"github.com/fatih/color"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	BUFFERSIZE int64 = 1000000
	// This Var is useless currently
	ReplayNext = &cobra.Command{
		Use:   "replaynext",
		Short: "Sync stateDB height to height x",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadConfig()
			// tmpPath, err := copy(cfg.Chain.TrieDBPath, "tmpState", BUFFERSIZE)
			// if err != nil {
			// 	return err
			// }
			// defer testutil.CleanupPathV2(tmpPath)
			// cfg.Chain.TrieDBPath = tmpPath

			svr := NewMiniServer(cfg)
			dao := svr.BlockDao()
			sf := svr.Factory()
			initCtx := svr.Context()

			if err := checkSanityV2(dao, sf); err != nil {
				return err
			}
			if err := replayNext(initCtx, dao, sf); err != nil {
				return err
			}
			c := color.New(color.FgRed).Add(color.Bold)
			c.Println("cpu.profile is generated at current dir")
			return nil
		},
	}
)

func checkSanityV2(dao blockdao.BlockDAO, indexer blockdao.BlockIndexer) error {
	daoHeight, err := dao.Height()
	if err != nil {
		return err
	}
	indexerHeight, err := indexer.Height()
	if err != nil {
		return err
	}

	if indexerHeight >= daoHeight {
		return errors.New("the height of indexer should be smallerr than the height of chainDB")
	}
	return nil
}

func replayNext(ctx context.Context, dao blockdao.BlockDAO, indexer blockdao.BlockIndexer) error {
	indexerHeight, err := indexer.Height()
	if err != nil {
		panic(err)
	}
	tipBlk, err := dao.GetBlockByHeight(indexerHeight)
	if err != nil {
		return err
	}
	log.L().Info("indexerHeight", zap.Uint64("h", indexerHeight))
	newHeight := indexerHeight + 1
	blk, err := getBlockFromChainDB(dao, newHeight)
	if err != nil {
		return err
	}
	f, _ := os.OpenFile("cpu.pprof", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	defer f.Close()
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	if err := indexBlock(ctx, tipBlk, blk, indexer); err != nil {
		return err
	}
	return nil
}

func copy(src, dst string, BUFFERSIZE int64) (string, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return "", err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file.", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer source.Close()

	_, err = os.Stat(dst)
	if err == nil {
		return "", fmt.Errorf("File %s already exists.", dst)
	}

	tempFile, err := os.CreateTemp(os.TempDir(), dst)
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	if err != nil {
		panic(err)
	}

	buf := make([]byte, BUFFERSIZE)
	for {
		n, err := source.Read(buf)
		if err != nil && err != io.EOF {
			return "", err
		}
		if n == 0 {
			break
		}

		if _, err := tempFile.Write(buf[:n]); err != nil {
			return "", err
		}
	}
	return tempFile.Name(), err
}
