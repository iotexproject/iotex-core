// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"encoding/hex"
	"math/big"
	"sort"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
	explorerapi "github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
)

func putBlockToParentChain(
	rootChainAPI explorerapi.Explorer,
	subChainAddr string,
	senderPubKey keypair.PublicKey,
	senderPriKey keypair.PrivateKey,
	senderAddr string,
	b *block.Block,
) {
	if err := putBlockToParentChainTask(rootChainAPI, subChainAddr, senderPubKey, senderPriKey, b); err != nil {
		log.L().Error("Failed to put block merkle roots to parent chain.",
			zap.String("subChainAddress", subChainAddr),
			zap.String("senderAddress", senderAddr),
			zap.Uint64("height", b.Height()),
			zap.Error(err))
		return
	}
	log.L().Info("Succeeded to put block merkle roots to parent chain.",
		zap.String("subChainAddress", subChainAddr),
		zap.String("senderAddress", senderAddr),
		zap.Uint64("height", b.Height()))
}

func putBlockToParentChainTask(
	rootChainAPI explorerapi.Explorer,
	subChainAddr string,
	senderPubKey keypair.PublicKey,
	senderPriKey keypair.PrivateKey,
	b *block.Block,
) error {
	req, err := constructPutSubChainBlockRequest(rootChainAPI, subChainAddr, senderPubKey, senderPriKey, b)
	if err != nil {
		return errors.Wrap(err, "fail to construct PutSubChainBlockRequest")
	}

	if _, err := rootChainAPI.PutSubChainBlock(req); err != nil {
		return errors.Wrap(err, "fail to call explorerapi to put block")
	}
	return nil
}

func constructPutSubChainBlockRequest(
	rootChainAPI explorerapi.Explorer,
	subChainAddr string,
	senderPubKey keypair.PublicKey,
	senderPriKey keypair.PrivateKey,
	b *block.Block,
) (explorerapi.PutSubChainBlockRequest, error) {
	// get sender address on mainchain
	subChainAddrSt, err := address.Bech32ToAddress(subChainAddr)
	if err != nil {
		return explorerapi.PutSubChainBlockRequest{}, errors.Wrap(err, "fail to convert subChainAddr")
	}
	parentChainID := subChainAddrSt.ChainID()
	senderPKHash := keypair.HashPubKey(senderPubKey)
	senderPCAddr := address.New(parentChainID, senderPKHash[:]).Bech32()

	// get sender current pending nonce on parent chain
	senderPCAddrDetails, err := rootChainAPI.GetAddressDetails(senderPCAddr)
	if err != nil {
		return explorerapi.PutSubChainBlockRequest{}, errors.Wrap(err, "fail to get address details")
	}

	rootm := make(map[string]hash.Hash32B)
	rootm["state"] = b.StateRoot()
	rootm["tx"] = b.TxRoot()
	pb := action.NewPutBlock(
		uint64(senderPCAddrDetails.PendingNonce),
		subChainAddr,
		senderPCAddr,
		b.Height(),
		rootm,
		1000000,        // gas limit
		big.NewInt(10), //gas price
	)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(uint64(senderPCAddrDetails.PendingNonce)).
		SetDestinationAddress(subChainAddr).
		SetGasPrice(big.NewInt(10)).
		SetGasLimit(1000000).
		SetAction(pb).Build()

	// sign action
	selp, err := action.Sign(elp, senderPCAddr, senderPriKey)
	if err != nil {
		return explorerapi.PutSubChainBlockRequest{}, errors.Wrap(err, "fail to sign put block action")
	}

	req := explorerapi.PutSubChainBlockRequest{
		Version:         int64(selp.Version()),
		Nonce:           int64(selp.Nonce()),
		SenderAddress:   selp.SrcAddr(),
		SenderPubKey:    hex.EncodeToString(senderPubKey[:]),
		GasLimit:        int64(selp.GasLimit()),
		GasPrice:        selp.GasPrice().String(),
		SubChainAddress: pb.SubChainAddress(),
		Height:          int64(pb.Height()),
		Roots:           make([]explorerapi.PutSubChainBlockMerkelRoot, 0, 2),
		Signature:       hex.EncodeToString(selp.Signature()),
	}

	// put merkel roots
	keys := make([]string, 0, len(rootm))
	for k := range rootm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := rootm[k]
		req.Roots = append(req.Roots, explorerapi.PutSubChainBlockMerkelRoot{
			Name:  k,
			Value: hex.EncodeToString(v[:]),
		})
	}
	return req, nil
}
