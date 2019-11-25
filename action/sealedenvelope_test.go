package action

import (
	"testing"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
)

const (
	publicKey = "04403d3c0dbd3270ddfc248c3df1f9aafd60f1d8e7456961c9ef262" +
		"92262cc68f0ea9690263bef9e197a38f06026814fc70912c2b98d2e90a68f8ddc5328180a01"
)

var signByte = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}

func TestSealedEnvelope_Hash(t *testing.T) {
	req := require.New(t)
	se, err := createSealedEnvelope()
	req.NoError(err)
	rHash := se.Hash()
	exlByte := []byte{50, 40, 132, 251, 4, 102, 48, 25, 190, 111, 180, 97, 217, 69, 56, 39, 72,
		126, 175, 221, 87, 180, 222, 59, 216, 154, 125, 119, 201, 191, 131, 149}
	exlpHash := hash.BytesToHash256(exlByte)
	req.Equal(exlpHash, rHash)
}
func TestSealedEnvelope_Signature(t *testing.T) {
	req := require.New(t)
	se, err := createSealedEnvelope()
	req.NoError(err)
	res := se.Signature()
	req.Equal(signByte, res)
}
func TestSealedEnvelope_LoadProto(t *testing.T) {
	req := require.New(t)
	se, err := createSealedEnvelope()
	req.NoError(err)
	rAction := se.Proto()
	err2 := se.LoadProto(rAction)
	req.Equal(nil, err2)
}

func createSealedEnvelope() (SealedEnvelope, error) {
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

	cPubKey, err := crypto.HexStringToPublicKey(publicKey)
	se := SealedEnvelope{}
	se.Envelope = evlp
	se.srcPubkey = cPubKey
	se.signature = signByte
	return se, err
}
