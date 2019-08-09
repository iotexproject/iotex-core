// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bot

import (
	"context"
	"encoding/hex"
	"math/big"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-antenna-go/v2/account"
	"github.com/iotexproject/iotex-antenna-go/v2/iotex"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/tools/bot/config"
	"github.com/iotexproject/iotex-core/tools/bot/pkg/util"
	"github.com/iotexproject/iotex-core/tools/bot/pkg/util/grpcutil"
)

// Transfer defines transfer struct
type Transfer struct {
	cfg    config.Config
	ctx    context.Context
	cancel context.CancelFunc
	name   string
	alert  Alert
}

// NewTransfer make new transfer
func NewTransfer(cfg config.Config, name string) (Service, error) {
	return newTransfer(cfg, name)
}

func newTransfer(cfg config.Config, name string) (Service, error) {
	svr := Transfer{
		cfg:  cfg,
		name: name,
	}
	return &svr, nil
}

// Alert add a alert
func (s *Transfer) Alert(a Alert) {
	s.alert = a
}

// Start starts the server
func (s *Transfer) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	return s.startTransfer()
}

// Stop stops the server
func (s *Transfer) Stop() {
	s.cancel()
}

// Name returns name
func (s *Transfer) Name() string {
	return s.name
}

func (s *Transfer) startTransfer() error {
	// load keystore
	if len(s.cfg.Transfer.Sender) != 2 {
		return errors.New("signer needs password")
	}
	pri, err := util.GetPrivateKey(s.cfg.Wallet, s.cfg.Transfer.Sender[0], s.cfg.Transfer.Sender[1])
	if err != nil {
		return err
	}
	hs, err := s.transfer(pri)
	if err != nil {
		return err
	}
	// check if timeout
	s.checkAndAlert(hs)
	return nil
}
func (s *Transfer) checkAndAlert(hs string) {
	d := time.Duration(s.cfg.AlertThreshold) * time.Second
	t := time.NewTicker(d)
	defer t.Stop()

	select {
	case <-t.C:
		err := grpcutil.GetReceiptByActionHash(s.cfg.API.URL, hs)
		if err != nil {
			log.L().Fatal("transfer timeout:", zap.String("transfer hash", hs), zap.Error(err))
			return
		}
		log.L().Info("transfer success:", zap.String("transfer hash", hs))
	case <-s.ctx.Done():
		return
	}
}
func (s *Transfer) transfer(pri crypto.PrivateKey) (txhash string, err error) {
	gasprice := big.NewInt(0).SetUint64(s.cfg.GasPrice)
	nonce, err := grpcutil.GetNonce(s.cfg.API.URL, s.cfg.Transfer.Sender[0])
	if err != nil {
		return
	}
	amount, ok := big.NewInt(0).SetString(s.cfg.Transfer.AmountInRau, 10)
	if !ok {
		err = errors.New("amount convert error")
		return
	}
	conn, err := grpcutil.ConnectToEndpoint(s.cfg.API.URL)
	if err != nil {
		return
	}
	defer conn.Close()
	acc, err := account.PrivateKeyToAccount(pri)
	if err != nil {
		return
	}
	cli := iotex.NewAuthedClient(iotexapi.NewAPIServiceClient(conn), acc)
	addr, err := address.FromString(s.cfg.Transfer.Sender[0])
	if err != nil {
		return
	}
	shash, err := cli.Transfer(addr, amount).SetNonce(nonce).SetGasLimit(s.cfg.GasLimit).SetGasPrice(gasprice).Call(context.Background())
	if err != nil {
		return
	}
	txhash = hex.EncodeToString(shash[:])
	log.L().Info("transfer:", zap.String("transfer hash", txhash), zap.Uint64("nonce", nonce), zap.String("from", s.cfg.Transfer.Sender[0]), zap.String("to", s.cfg.Transfer.Sender[0]))
	return
}
