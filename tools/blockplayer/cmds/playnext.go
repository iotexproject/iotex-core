package cmd

import (
	"os"
	"runtime/pprof"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type playNextCmd struct {
	svr *miniServer
}

var (
	PlayNext = &cobra.Command{
		Use:   "playnext",
		Short: "play next height of block and generate pprof report",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svr, err := NewMiniServer(MiniServerConfig())
			if err != nil {
				return err
			}
			playNextCmd := newPlayNextCmd(svr)
			if err := playNextCmd.checkSanity(); err != nil {
				return err
			}
			if err := playNextCmd.execute(); err != nil {
				return err
			}
			c := color.New(color.FgRed).Add(color.Bold)
			c.Println("cpu.profile and mem.profile is generated at current dir")
			return nil
		},
	}
)

func newPlayNextCmd(svr *miniServer) *playNextCmd {
	return &playNextCmd{
		svr: svr,
	}
}

func (cmd *playNextCmd) checkSanity() error {
	daoHeight, err := cmd.svr.BlockDao().Height()
	if err != nil {
		return err
	}
	indexerHeight, err := cmd.svr.Factory().Height()
	if err != nil {
		return err
	}
	if indexerHeight >= daoHeight {
		return errors.New("the height of indexer should be smallerr than the height of chainDB")
	}
	return nil
}

func (cmd *playNextCmd) execute() error {
	var (
		ctx     = cmd.svr.Context()
		dao     = cmd.svr.BlockDao()
		indexer = cmd.svr.Factory()
	)
	indexerHeight, err := indexer.Height()
	if err != nil {
		panic(err)
	}
	tipBlk, err := dao.GetBlockByHeight(indexerHeight)
	if err != nil {
		return err
	}
	newHeight := indexerHeight + 1
	blk, err := getBlockFromChainDB(dao, newHeight)
	if err != nil {
		return err
	}
	cpufile, _ := os.OpenFile("cpu.pprof", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	defer cpufile.Close()
	memfile, _ := os.OpenFile("mem.pprof", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	defer memfile.Close()
	pprof.StartCPUProfile(cpufile)
	defer pprof.StopCPUProfile()
	if err := indexBlock(ctx, tipBlk, blk, indexer); err != nil {
		return err
	}
	return pprof.WriteHeapProfile(memfile)
}
