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
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/tools/bot/config"
	"github.com/iotexproject/iotex-core/tools/bot/pkg/util"
	"github.com/iotexproject/iotex-core/tools/bot/pkg/util/grpcutil"
)

const (
	//000000000000000000000000da7e12ef57c236a06117c5e0d04a228e7181cf36
	paramsLen     = 64
	addressLen    = 40
	addressPrefix = "000000000000000000000000"
	transferSha3  = "a9059cbb"
)

// Xrc20 defines xrc20 struct
type Xrc20 struct {
	cfg    config.Config
	ctx    context.Context
	cancel context.CancelFunc
	name   string
	alert  Alert
}

// NewXrc20 make a new transfer
func NewXrc20(cfg config.Config, name string) (Service, error) {
	return newXrc20(cfg, name)
}

func newXrc20(cfg config.Config, name string) (Service, error) {
	svr := Xrc20{
		cfg:  cfg,
		name: name,
	}
	return &svr, nil
}

// Alert add a alert
func (s *Xrc20) Alert(a Alert) {
	s.alert = a
}

// Start starts the server
func (s *Xrc20) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	return s.startTransfer()
}

// Stop stops the server
func (s *Xrc20) Stop() error {
	s.cancel()
	return nil
}

// Name returns name
func (s *Xrc20) Name() string {
	return s.name
}

func (s *Xrc20) startTransfer() error {
	if len(s.cfg.Xrc20.From) != 2 {
		return errors.New("signer needs password")
	}
	// load keystore
	pri, err := util.GetPrivateKey(s.cfg.Wallet, s.cfg.Xrc20.From[0], s.cfg.Xrc20.From[1])
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
func (s *Xrc20) checkAndAlert(hs string) {
	d := time.Duration(s.cfg.Xrc20.AlertThreshold) * time.Second
	t := time.NewTicker(d)
	defer t.Stop()

	select {
	case <-t.C:
		err := grpcutil.GetReceiptByActionHash(s.cfg.API.URL, hs)
		if err != nil {
			log.L().Error("xrc20 transfer timeout:", zap.String("xrc20 transfer hash", hs), zap.Error(err))
			if s.alert != nil {
				s.alert.Send("xrc20 transfer timeout: " + hs + ":" + err.Error())
			}
			return
		}
		log.L().Info("xrc20 transfer success:", zap.String("xrc20 transfer hash", hs))
	}
}
func (s *Xrc20) transfer(pri crypto.PrivateKey) (txhash string, err error) {
	nonce, err := grpcutil.GetNonce(s.cfg.API.URL, s.cfg.Xrc20.From[0])
	if err != nil {
		return
	}
	gasprice := big.NewInt(0).SetUint64(s.cfg.Xrc20.GasPrice)
	amount, ok := big.NewInt(0).SetString(s.cfg.Xrc20.Amount, 10)
	if !ok {
		err = errors.New("amount convert error")
		return
	}
	amountHex := amount.Text(16)
	amountParams := strings.Repeat("0", paramsLen-len(amountHex)) + amountHex
	to, err := address.FromString(s.cfg.Xrc20.To[0])
	if err != nil {
		return
	}
	data := transferSha3 + addressPrefix + hex.EncodeToString(to.Bytes()) + amountParams
	dataBytes, err := hex.DecodeString(data)
	if err != nil {
		return
	}
	tx, err := action.NewExecution(s.cfg.Xrc20.Contract, nonce, big.NewInt(0),
		s.cfg.Xrc20.GasLimit, gasprice, dataBytes)
	if err != nil {
		return
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasLimit(s.cfg.Xrc20.GasLimit).
		SetGasPrice(gasprice).
		SetAction(tx).Build()
	selp, err := action.Sign(elp, pri)
	if err != nil {
		return
	}
	err = grpcutil.SendAction(s.cfg.API.URL, selp.Proto())
	if err != nil {
		return
	}
	shash := hash.Hash256b(byteutil.Must(proto.Marshal(selp.Proto())))
	txhash = hex.EncodeToString(shash[:])
	log.L().Info("xrc20 transfer:", zap.String("xrc20 transfer hash", txhash), zap.Uint64("nonce", nonce), zap.String("from", s.cfg.Xrc20.From[0]), zap.String("to", s.cfg.Xrc20.To[0]))
	return
}
