package api

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

func TestBlockListener_Respond(t *testing.T) {
	steam := &iotexapi.APIService_StreamBlocksServer
	errChan := make(chan error)
	bl := NewBlockListener(steam, errChan)
	for i := 0; i < 10; i++ {
		block1, _ := makeBlock2(i)
		err := bl.Respond(block1)
		fmt.Print(err)
	}
}

func makeBlock2(n int) (*block.Block, error) {
	rand.Seed(time.Now().Unix())
	sevlps := make([]action.SealedEnvelope, 0)
	for j := 1; j <= n; j++ {
		i := rand.Int()
		tsf, _ := action.NewTransfer(
			uint64(i),
			unit.ConvertIotxToRau(1000+int64(i)),
			identityset.Address(i%identityset.Size()).String(),
			nil,
			20000+uint64(i),
			unit.ConvertIotxToRau(1+int64(i)),
		)
		eb := action.EnvelopeBuilder{}
		evlp := eb.
			SetAction(tsf).
			SetGasLimit(tsf.GasLimit()).
			SetGasPrice(tsf.GasPrice()).
			SetNonce(tsf.Nonce()).
			SetVersion(1).
			Build()
		sevlp, _ := action.Sign(evlp, identityset.PrivateKey((i+1)%identityset.Size()))
		sevlps = append(sevlps, sevlp)
	}
	rap := block.RunnableActionsBuilder{}
	ra := rap.
		SetHeight(1).
		SetTimeStamp(time.Now()).
		AddActions(sevlps...).
		Build(identityset.PrivateKey(0).PublicKey())
	blk, err := block.NewBuilder(ra).
		SetVersion(1).
		SetReceiptRoot(hash.Hash256b([]byte("hello, world!"))).
		SetDeltaStateDigest(hash.Hash256b([]byte("world, hello!"))).
		SetPrevBlockHash(hash.Hash256b([]byte("hello, block!"))).
		SignAndBuild(identityset.PrivateKey(0))
	return &blk, err
}
