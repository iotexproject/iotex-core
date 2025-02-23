package archive

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
)

func TestERC20(t *testing.T) {
	r := require.New(t)

	rpc = "http://34.44.119.149:15014"
	legacyRPC = "https://archive-mainnet.iotex.io"
	r.NoError(connectRPC())

	contract, err := address.FromHex("0x6fbcdc1169b5130c59e72e51ed68a84841c98cd1")
	r.NoError(err)
	addr, err := address.FromHex("0xe82b7054471d3f5cc825c50350da3bca64f7be4e")
	r.NoError(err)
	ethTo := common.BytesToAddress(contract.Bytes())
	ethAddr := common.BytesToAddress(addr.Bytes())
	data, err := abiCall(erc20ABI, "balanceOf(address)", ethAddr)
	r.NoError(err)
	height := uint64(13693959)
	for height <= 13693961 {
		t.Log("height", height)
		// get balance from erigon
		resp, err := ethcli.CallContract(context.Background(), ethereum.CallMsg{
			To:   &ethTo,
			Data: data,
		}, big.NewInt(int64(height+1)))
		r.NoError(err)
		erigonBalance := new(big.Int).SetBytes(resp)
		t.Log("erigon balance", erigonBalance)
		// get balance from archive
		resp, err = ethcliLegacy.CallContract(context.Background(), ethereum.CallMsg{
			To:   &ethTo,
			Data: data,
		}, big.NewInt(int64(height)))
		r.NoError(err)
		archiveBalance := new(big.Int).SetBytes(resp)
		t.Log("archive balance", archiveBalance)

		height++
	}
}
