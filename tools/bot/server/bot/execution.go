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

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/tools/bot/config"
	"github.com/iotexproject/iotex-core/tools/bot/pkg/log"
	"github.com/iotexproject/iotex-core/tools/bot/pkg/util"
	"github.com/iotexproject/iotex-core/tools/bot/pkg/util/grpcutil"
)

// Execution
type Execution struct {
	cfg    config.Config
	ctx    context.Context
	cancel context.CancelFunc
	name   string
	alert  Alert
}

// NewTransfer
func NewExecution(cfg config.Config, name string) (Service, error) {
	return newExecution(cfg, name)
}

func newExecution(cfg config.Config, name string) (Service, error) {
	svr := Execution{
		cfg:  cfg,
		name: name,
	}
	return &svr, nil
}

func (s *Execution) Alert(a Alert) {
	s.alert = a
}

// Start starts the server
func (s *Execution) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	return s.start()
}

// Stop stops the server
func (s *Execution) Stop() error {
	s.cancel()
	return nil
}

// Name
func (s *Execution) Name() string {
	return s.name
}

func (s *Execution) start() error {
	if len(s.cfg.Execution.From) != 2 {
		return errors.New("signer needs password")
	}
	// load keystore
	pri, err := util.GetPrivateKey(s.cfg.Wallet, s.cfg.Execution.From[0], s.cfg.Execution.From[1])
	if err != nil {
		return err
	}
	hs, err := s.exec(pri)
	if err != nil {
		return err
	}
	// check if timeout
	s.checkAndAlert(hs)
	return nil
}
func (s *Execution) checkAndAlert(hs string) {
	d := time.Duration(s.cfg.Execution.AlertThreshold) * time.Second
	t := time.NewTicker(d)
	defer t.Stop()

	select {
	case <-t.C:
		err := grpcutil.GetReceiptByActionHash(s.cfg.API.Url, false, hs)
		if err != nil {
			log.L().Error("Execution timeout:", zap.String("Execution hash", hs), zap.Error(err))
			if s.alert != nil {
				s.alert.Send("Execution timeout: " + hs + ":" + err.Error())
			}
			return
		}
		log.L().Info("Execution success:", zap.String("Execution hash", hs))
	}
}
func (s *Execution) exec(pri crypto.PrivateKey) (txhash string, err error) {
	nonce, err := grpcutil.GetNonce(s.cfg.API.Url, false, s.cfg.Execution.From[0])
	if err != nil {
		return
	}
	gasprice := big.NewInt(0).SetUint64(s.cfg.Execution.GasPrice)

	dataBytes, err := hex.DecodeString(s.cfg.Execution.Data)
	if err != nil {
		return
	}
	amount, ok := big.NewInt(0).SetString(s.cfg.Execution.Amount, 10)
	if !ok {
		err = errors.New("amount convert error")
		return
	}
	tx, err := action.NewExecution(s.cfg.Execution.Contract, nonce, amount,
		s.cfg.Execution.GasLimit, gasprice, dataBytes)
	if err != nil {
		return
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasLimit(s.cfg.Execution.GasLimit).
		SetGasPrice(gasprice).
		SetAction(tx).Build()
	selp, err := action.Sign(elp, pri)
	if err != nil {
		return
	}
	err = grpcutil.SendAction(s.cfg.API.Url, false, selp.Proto())
	if err != nil {
		return
	}
	shash := hash.Hash256b(byteutil.Must(proto.Marshal(selp.Proto())))
	txhash = hex.EncodeToString(shash[:])
	log.L().Info("Execution: ", zap.String("Execution hash", txhash), zap.Uint64("nonce", nonce), zap.String("from", s.cfg.Execution.From[0]))
	return
}
