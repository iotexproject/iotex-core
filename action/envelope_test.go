package action

import (
	"testing"

	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"

	"github.com/stretchr/testify/require"
)

func TestEnvelope_LoadProto(t *testing.T) {
	req := require.New(t)
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

	proto := evlp.Proto()
	err := evlp.LoadProto(proto)
	req.Equal(nil, err)
}
