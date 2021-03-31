package action

import (
	"encoding/hex"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestEnvelope_Basic(t *testing.T) {
	req := require.New(t)
	evlp, tsf := createEnvelope()
	req.Equal(uint32(1), evlp.Version())
	req.Equal(uint64(10), evlp.Nonce())
	req.Equal(uint64(20010), evlp.GasLimit())
	req.Equal("11000000000000000000", evlp.GasPrice().String())
	c, err := evlp.Cost()
	req.NoError(err)
	req.Equal("111010000000000000000000", c.String())
	g, err := evlp.IntrinsicGas()
	req.NoError(err)
	req.Equal(uint64(10000), g)
	d, ok := evlp.Destination()
	req.True(ok)
	req.Equal("io1jh0ekmccywfkmj7e8qsuzsupnlk3w5337hjjg2", d)
	tsf2, ok := evlp.Action().(*Transfer)
	req.True(ok)
	req.Equal(tsf, tsf2)
}
func TestEnvelope_Proto(t *testing.T) {
	req := require.New(t)
	eb, tsf := createEnvelope()
	evlp, ok := eb.(*envelope)
	req.True(ok)
	req.Equal("0801100a18aa9c012214313130303030303030303030303030303030303052430a16313031303030303030303030303030303030303030301229696f316a6830656b6d63637977666b6d6a3765387173757a7375706e6c6b337735333337686a6a6732", hex.EncodeToString(evlp.serialize()))
	h := evlp.Hash()
	req.Equal("0c60f43e0d1410b282bdceb8682b8c8b11fc0f03f559825f51b55f21643447e9", hex.EncodeToString(h[:]))

	proto := evlp.Proto()
	actCore := &iotextypes.ActionCore{
		Version:  evlp.version,
		Nonce:    evlp.nonce,
		GasLimit: evlp.gasLimit,
	}
	actCore.GasPrice = evlp.gasPrice.String()
	actCore.Action = &iotextypes.ActionCore_Transfer{Transfer: tsf.Proto()}
	req.Equal(actCore, proto)

	req.NoError(evlp.LoadProto(proto))
	tsf2, ok := evlp.Action().(*Transfer)
	req.True(ok)
	req.Equal(tsf.amount, tsf2.amount)
	req.Equal(tsf.recipient, tsf2.recipient)
	req.Equal(tsf.payload, tsf2.payload)
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
