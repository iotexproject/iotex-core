// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
)

func putBlockToParentChain(
	subChainAddr string,
	senderPrvKey keypair.PrivateKey,
	senderAddr string,
	b *block.Block,
) {
	if err := putBlockToParentChainTask(subChainAddr, senderPrvKey, b); err != nil {
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
	subChainAddr string,
	senderPrvKey keypair.PrivateKey,
	b *block.Block,
) error {
	err := constructPutSubChainBlockRequest(subChainAddr, senderPrvKey.PublicKey(), senderPrvKey, b)
	if err != nil {
		return errors.Wrap(err, "fail to construct PutSubChainBlockRequest")
	}

	return nil
}

func constructPutSubChainBlockRequest(
	subChainAddr string,
	senderPubKey keypair.PublicKey,
	senderPriKey keypair.PrivateKey,
	b *block.Block,
) error {

	return nil
}
