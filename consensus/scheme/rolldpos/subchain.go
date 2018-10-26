package rolldpos

import (
	"encoding/hex"
	"math/big"
	"sort"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	explorerapi "github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

func putBlockToParentChain(rootChainAPI explorerapi.Explorer, subChainAddr string, sender *iotxaddress.Address, b *blockchain.Block) {
	if err := putBlockToParentChainTask(rootChainAPI, subChainAddr, sender, b); err != nil {
		logger.Error().
			Str("subChainAddress", subChainAddr).
			Str("senderAddress", sender.RawAddress).
			Uint64("height", b.Height()).
			Err(err).
			Msg("Failed to put block merkel roots to parent chain.")
	}
}

func putBlockToParentChainTask(rootChainAPI explorerapi.Explorer, subChainAddr string, sender *iotxaddress.Address, b *blockchain.Block) error {
	// get current pending nonce
	addrDetails, err := rootChainAPI.GetAddressDetails(subChainAddr)
	if err != nil {
		return errors.Wrap(err, "fail to get address details")
	}

	rootm := make(map[string]hash.Hash32B)
	rootm["state"] = b.StateRoot()
	rootm["tx"] = b.TxRoot()
	pb := action.NewPutBlock(
		uint64(addrDetails.PendingNonce),
		subChainAddr,
		sender.RawAddress,
		b.Height(),
		rootm,
		1000000,        // gas limit
		big.NewInt(10), //gas price
	)
	// sign action
	if err := action.Sign(pb, sender.PrivateKey); err != nil {
		return errors.Wrap(err, "fail to sign put block action")
	}

	req := explorerapi.PutSubChainBlockRequest{
		Version:         int64(pb.Version()),
		Nonce:           int64(pb.Nonce()),
		SenderAddress:   pb.SrcAddr(),
		SenderPubKey:    hex.EncodeToString(sender.PublicKey[:]),
		GasLimit:        int64(pb.GasLimit()),
		GasPrice:        pb.GasPrice().String(),
		SubChainAddress: pb.SubChainAddress(),
		Height:          int64(pb.Height()),
		Roots:           make([]explorerapi.PutSubChainBlockMerkelRoot, 0, 2),
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

	// submit action
	if _, err := rootChainAPI.PutSubChainBlock(req); err != nil {
		return errors.Wrap(err, "fail to call explorerapi to put block")
	}
	return nil
}
