// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package bot

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/tools/bot/config"
	"github.com/iotexproject/iotex-core/tools/bot/pkg/util/grpcutil"
)

const (
	multiSendSha3   = "e3b48f48"
	multiSendOffset = "0000000000000000000000000000000000000000000000000000000000000060"
	prefixZero      = "000000000000000000000000"
	fixPayLoad      = "00000000000000000000000000000000000000000000000000000000000000047465737400000000000000000000000000000000000000000000000000000000"
)

// Execution defines a execution
type Execution struct {
	cfg    config.Config
	ctx    context.Context
	cancel context.CancelFunc
	name   string
}

// NewExecution make a new execution
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

// Start starts the server
func (s *Execution) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	return s.start()
}

// Stop stops the server
func (s *Execution) Stop() {
	s.cancel()
}

// Name returns name
func (s *Execution) Name() string {
	return s.name
}

func (s *Execution) start() error {
	sk, err := crypto.HexStringToPrivateKey(s.cfg.Xrc20.Signer)
	if err != nil {
		return err
	}
	hs, err := s.exec(sk)
	if err != nil {
		return err
	}
	// check if timeout
	s.checkAndAlert(hs)
	return nil
}
func (s *Execution) checkAndAlert(hs string) {
	d := time.Duration(s.cfg.AlertThreshold) * time.Second
	t := time.NewTicker(d)
	defer t.Stop()

	select {
	case <-t.C:
		err := grpcutil.GetReceiptByActionHash(s.cfg.API.URL, hs)
		if err != nil {
			log.L().Fatal("Execution timeout:", zap.String("Execution hash", hs), zap.Error(err))
			return
		}
		log.L().Info("Execution success:", zap.String("Execution hash", hs))
	case <-s.ctx.Done():
		return
	}
}
func (s *Execution) exec(pri crypto.PrivateKey) (txhash string, err error) {
	addr := pri.PublicKey().Address()
	if addr == nil {
		err = errors.New("failed to get address")
		return
	}
	nonce, err := grpcutil.GetNonce(s.cfg.API.URL, addr.String())
	if err != nil {
		return
	}
	gasprice := big.NewInt(0).SetUint64(s.cfg.GasPrice)

	if len(s.cfg.Execution.To.Address) != len(s.cfg.Execution.To.Amount) {
		err = errors.New("address len is not equal to amount len")
		return
	}
	data := multiSendSha3 + multiSendOffset
	params2Offset := 32*3 + 1*32 + len(s.cfg.Execution.To.Address)*32
	params := fmt.Sprintf("%x", params2Offset)
	data += strings.Repeat("0", 64-len(params)) + params

	params3Offset := params2Offset + 1*32 + len(s.cfg.Execution.To.Address)*32
	params = fmt.Sprintf("%x", params3Offset)
	data += strings.Repeat("0", 64-len(params)) + params

	lenOfAddress := fmt.Sprintf("%x", len(s.cfg.Execution.To.Address))
	data += strings.Repeat("0", 64-len(lenOfAddress)) + lenOfAddress
	for _, addr := range s.cfg.Execution.To.Address {
		a, errs := address.FromString(addr)
		if errs != nil {
			err = errs
			return
		}
		data += prefixZero + hex.EncodeToString(a.Bytes())
	}
	data += strings.Repeat("0", 64-len(lenOfAddress)) + lenOfAddress
	for _, amount := range s.cfg.Execution.To.Amount {
		amo, ok := new(big.Int).SetString(amount, 10)
		if !ok {
			err = errors.New("amount convert error")
			return
		}
		data += strings.Repeat("0", 64-len(amo.Text(16))) + amo.Text(16)
	}
	data += fixPayLoad
	dataBytes, err := hex.DecodeString(data)
	if err != nil {
		return
	}
	amount, ok := new(big.Int).SetString(s.cfg.Execution.Amount, 10)
	if !ok {
		err = errors.New("amount convert error")
		return
	}
	tx, err := action.NewExecution(s.cfg.Execution.Contract, nonce, amount,
		s.cfg.GasLimit, gasprice, dataBytes)
	if err != nil {
		return
	}
	tx, err = grpcutil.FixGasLimit(s.cfg.API.URL, addr.String(), tx)
	if err != nil {
		return
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasLimit(tx.GasLimit()).
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
	log.L().Info("Execution: ", zap.String("Execution hash", txhash), zap.Uint64("nonce", nonce), zap.String("from", addr.String()))
	return
}
