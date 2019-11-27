package action

import (
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
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
	evlp, tsf := createEnvelope()
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
	evlp, _ := createEnvelope()
	s := evlp.Serialize()
	pS := []byte{8, 1, 16, 10, 24, 170, 156, 1, 34, 20, 49, 49, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48,
		48, 48, 48, 48, 82, 67, 10, 22,
		49, 48, 49, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 48, 18, 41, 105, 111, 49,
		106, 104, 48, 101, 107,
		109, 99, 99, 121, 119, 102, 107, 109, 106, 55, 101, 56, 113, 115, 117, 122, 115, 117, 112, 110, 108, 107, 51,
		119, 53, 51, 51, 55,
		104, 106, 106, 103, 50}
	req.Equal(pS, s)
}
func TestEnvelope_Hash(t *testing.T) {
	req := require.New(t)
	evlp, _ := createEnvelope()
	h := evlp.Hash()
	exp := []byte{12, 96, 244, 62, 13, 20, 16, 178, 130, 189, 206, 184, 104, 43, 140, 139, 17, 252, 15, 3, 245, 89,
		130, 95, 81, 181, 95, 33, 100, 52, 71, 233}
	expH := hash.BytesToHash256(exp)
	req.Equal(expH, h)
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
