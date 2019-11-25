package action

import (
	"testing"

	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"

	"github.com/stretchr/testify/require"
)

func TestEnvelope_Destination(t *testing.T) {
	req := require.New(t)
	evlp := createEnvelope()
	res, boolT := evlp.Destination()
	req.Equal("io1jh0ekmccywfkmj7e8qsuzsupnlk3w5337hjjg2", res)
	req.Equal(true, boolT)
}
func TestEnvelope_GasPrice(t *testing.T) {
	req := require.New(t)
	evlp := createEnvelope()
	gasPrice := evlp.GasPrice()
	req.Equal("11000000000000000000", gasPrice.String())
}
func TestEnvelope_LoadProto(t *testing.T) {
	req := require.New(t)
	evlp := createEnvelope()
	proto := evlp.Proto()
	err := evlp.LoadProto(proto)
	req.Equal(nil, err)
}
func TestEnvelope_Serialize(t *testing.T) {
	req := require.New(t)
	evlp := createEnvelope()
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
func createEnvelope() Envelope {
	i := 10
	tsf, _ := NewTransfer(
		uint64(i),
		unit.ConvertIotxToRau(1000+int64(i)),
		identityset.Address(i%identityset.Size()).String(),
		nil,
		20000+uint64(i),
		unit.ConvertIotxToRau(1+int64(i)),
	)
	eb := EnvelopeBuilder{}
	evlp := eb.
		SetAction(tsf).
		SetGasLimit(tsf.GasLimit()).
		SetGasPrice(tsf.GasPrice()).
		SetNonce(tsf.Nonce()).
		SetVersion(1).
		Build()
	return evlp
}
