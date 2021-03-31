package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestEnvelope_Version(t *testing.T) {
	req := require.New(t)
	evlp, _ := createEnvelope()
	req.Equal(uint32(1), evlp.Version())
}
func TestEnvelope_Nonce(t *testing.T) {
	req := require.New(t)
	evlp, _ := createEnvelope()
	req.Equal(uint64(10), evlp.Nonce())
}
func TestEnvelope_GasLimit(t *testing.T) {
	req := require.New(t)
	evlp, _ := createEnvelope()
	req.Equal(uint64(20010), evlp.GasLimit())
}
func TestEnvelope_Destination(t *testing.T) {
	req := require.New(t)
	evlp, _ := createEnvelope()
	res, boolT := evlp.Destination()
	req.Equal("io1jh0ekmccywfkmj7e8qsuzsupnlk3w5337hjjg2", res)
	req.True(boolT)
}
func TestEnvelope_GasPrice(t *testing.T) {
	req := require.New(t)
	evlp, _ := createEnvelope()
	gasPrice := evlp.GasPrice()
	expPrice, _ := new(big.Int).SetString("11000000000000000000", 10)
	req.Equal(0, gasPrice.Cmp(expPrice))
}
func TestEnvelope_Cost(t *testing.T) {
	req := require.New(t)
	evlp, _ := createEnvelope()
	c, err := evlp.Cost()
	expC, _ := new(big.Int).SetString("111010000000000000000000", 10)
	req.Equal(0, c.Cmp(expC))
	req.NoError(err)
}
func TestEnvelope_IntrinsicGas(t *testing.T) {
	req := require.New(t)
	evlp, _ := createEnvelope()
	g, err := evlp.IntrinsicGas()
	req.Equal(uint64(10000), g)
	req.NoError(err)
}
func TestEnvelope_Action(t *testing.T) {
	req := require.New(t)
	evlp, tsf := createEnvelope()
	a := evlp.Action()
	req.Equal(tsf, a)
}
func TestEnvelope_Proto(t *testing.T) {
	req := require.New(t)
	eb, tsf := createEnvelope()
	evlp, ok := eb.(*envelope)
	req.True(ok)
	proto := evlp.Proto()
	actCore := &iotextypes.ActionCore{
		Version:  evlp.version,
		Nonce:    evlp.nonce,
		GasLimit: evlp.gasLimit,
	}
	actCore.GasPrice = evlp.gasPrice.String()
	actCore.Action = &iotextypes.ActionCore_Transfer{Transfer: tsf.Proto()}
	req.Equal(actCore, proto)
}
func TestEnvelope_LoadProto(t *testing.T) {
	req := require.New(t)
	evlp, _ := createEnvelope()
	proto := evlp.Proto()
	req.NoError(evlp.LoadProto(proto))
}
func TestEnvelope_Serialize(t *testing.T) {
	req := require.New(t)
	eb, _ := createEnvelope()
	evlp, ok := eb.(*envelope)
	req.True(ok)
	req.Equal("0801100a18aa9c012214313130303030303030303030303030303030303052430a16313031303030303030303030303030303030303030301229696f316a6830656b6d63637977666b6d6a3765387173757a7375706e6c6b337735333337686a6a6732", hex.EncodeToString(evlp.serialize()))
}
func TestEnvelope_Hash(t *testing.T) {
	req := require.New(t)
	evlp, _ := createEnvelope()
	h := evlp.Hash()
	req.Equal("0c60f43e0d1410b282bdceb8682b8c8b11fc0f03f559825f51b55f21643447e9", hex.EncodeToString(h[:]))
}
func createEnvelope() (Envelope, *Transfer) {
	tsf, _ := NewTransfer(
		uint64(10),
		unit.ConvertIotxToRau(1000+int64(10)),
		identityset.Address(10%identityset.Size()).String(),
		nil,
		20000+uint64(10),
		unit.ConvertIotxToRau(1+int64(10)),
	)
	eb := EnvelopeBuilder{}
	evlp := eb.
		SetAction(tsf).
		SetGasLimit(tsf.GasLimit()).
		SetGasPrice(tsf.GasPrice()).
		SetNonce(tsf.Nonce()).
		SetVersion(1).
		Build()
	return evlp, tsf
}
