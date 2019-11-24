package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/iotexproject/iotex-antenna-go/v2/iotex"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"
	glog "log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
)

func init() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr,
			"usage: server -config-path=[string]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
}

func main() {
	go listenBlockSync()
	startServer()
}

func listenBlockSync() {
	// wait server started
	time.Sleep(time.Minute * 5)
	conn, err := iotex.NewDefaultGRPCConn("localhost:14014")
	if err != nil {
		glog.Fatal(err)
	}
	defer conn.Close()

	c := iotex.NewReadOnlyClient(iotexapi.NewAPIServiceClient(conn))
	meta, err := c.API().GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{})
	if err != nil {
		glog.Fatal(err)
	}
	start := meta.ChainMeta.Height

	for {
		time.Sleep(time.Minute * 5)
		meta, err = c.API().GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{})
		if err != nil {
			glog.Fatal(err)
		}
		if start >= meta.ChainMeta.Height {
			glog.Fatalf("blockchain stop sync at %d height", start)
		}
	}
}

func startServer() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	signal.Notify(stop, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	livenessCtx, livenessCancel := context.WithCancel(context.Background())

	genesisCfg, err := genesis.New()
	if err != nil {
		glog.Fatalln("Failed to new genesis config.", zap.Error(err))
	}

	cfg, err := config.New()
	if err != nil {
		glog.Fatalln("Failed to new config.", zap.Error(err))
	}
	initLogger(cfg)

	cfg.Genesis = genesisCfg
	cfgToLog := cfg
	cfgToLog.Chain.ProducerPrivKey = ""
	log.S().Infof("Config in use: %+v", cfgToLog)

	// liveness start
	probeSvr := probe.New(cfg.System.HTTPStatsPort)
	if err := probeSvr.Start(ctx); err != nil {
		log.L().Fatal("Failed to start probe server.", zap.Error(err))
	}
	go func() {
		<-stop
		// start stopping
		cancel()
		<-stopped

		// liveness end
		if err := probeSvr.Stop(livenessCtx); err != nil {
			log.L().Error("Error when stopping probe server.", zap.Error(err))
		}
		livenessCancel()
	}()

	// create and start the node
	svr, err := itx.NewServer(cfg)
	if err != nil {
		log.L().Fatal("Failed to create server.", zap.Error(err))
	}

	cfgsub, err := config.NewSub()
	if err != nil {
		log.L().Fatal("Failed to new sub chain config.", zap.Error(err))
	}
	if cfgsub.Chain.ID != 0 {
		if err := svr.NewSubChainService(cfgsub); err != nil {
			log.L().Fatal("Failed to new sub chain.", zap.Error(err))
		}
	}

	itx.StartServer(ctx, svr, probeSvr, cfg)
	close(stopped)
	<-livenessCtx.Done()
}

func initLogger(cfg config.Config) {
	addr := cfg.ProducerAddress()
	if err := log.InitLoggers(cfg.Log, cfg.SubLogs, zap.Fields(
		zap.String("ioAddr", addr.String()),
		zap.String("networkAddr", fmt.Sprintf("%s:%d", cfg.Network.Host, cfg.Network.Port)),
	)); err != nil {
		glog.Println("Cannot config global logger, use default one: ", err)
	}
}
