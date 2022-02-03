package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
)

const (
	publicKey = "04403d3c0dbd3270ddfc248c3df1f9aafd60f1d8e7456961c9ef262" +
		"92262cc68f0ea9690263bef9e197a38f06026814fc70912c2b98d2e90a68f8ddc5328180a01"
)

var (
	signByte    = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	validSig, _ = hex.DecodeString("15e73ad521ec9e06600c59e49b127c9dee114ad64fb2fcbe5e0d9f4c8d2b766e73d708cca1dc050dd27b20f2ee607f30428bf035f45d4da8ec2fb04a90c2c30901")
)

func TestSealedEnvelope_Basic(t *testing.T) {
	req := require.New(t)
	se, err := createSealedEnvelope()
	req.NoError(err)
	rHash, err := se.Hash()
	req.NoError(err)
	req.Equal("322884fb04663019be6fb461d9453827487eafdd57b4de3bd89a7d77c9bf8395", hex.EncodeToString(rHash[:]))
	req.Equal(publicKey, se.SrcPubkey().HexString())
	req.Equal(signByte, se.Signature())
	req.Zero(se.Encoding())

}

func TestSealedEnvelope_InvalidType(t *testing.T) {
	require := require.New(t)
	candidates := state.CandidateList{}
	r := NewPutPollResult(1, 10001, candidates)

	bd := &EnvelopeBuilder{}
	elp := bd.SetNonce(1).
		SetAction(r).
		SetGasLimit(100000).Build()
	selp := FakeSeal(elp, identityset.PrivateKey(27).PublicKey())
	selp.encoding = iotextypes.Encoding_ETHEREUM_RLP
	hash1, err := selp.envelopeHash()
	require.Equal(hash1, hash.ZeroHash256)
	require.Contains(err.Error(), "invalid action type")
}

func TestSealedEnvelope_Actions(t *testing.T) {
	t.Skip()
	require := require.New(t)

	createStake, err := NewCreateStake(uint64(10), addr2, "100", uint32(10000), true, payload, gasLimit, gasPrice)
	require.NoError(err)

	depositToStake, err := NewDepositToStake(1, 2, big.NewInt(10).String(), payload, gasLimit, gasPrice)
	require.NoError(err)

	changeCandidate, err := NewChangeCandidate(1, candidate1Name, 2, payload, gasLimit, gasPrice)
	require.NoError(err)

	unstake, err := NewUnstake(nonce, 2, payload, gasLimit, gasPrice)
	require.NoError(err)

	withdrawStake, err := NewWithdrawStake(nonce, 2, payload, gasLimit, gasPrice)
	require.NoError(err)

	restake, err := NewRestake(nonce, index, duration, autoStake, payload, gasLimit, gasPrice)
	require.NoError(err)

	transferStake, err := NewTransferStake(nonce, cand1Addr, 2, payload, gasLimit, gasPrice)
	require.NoError(err)

	candidateRegister, err := NewCandidateRegister(nonce, candidate1Name, cand1Addr, cand1Addr, cand1Addr, big.NewInt(10).String(), 91, true, payload, gasLimit, gasPrice)
	require.NoError(err)

	candidateUpdate, err := NewCandidateUpdate(nonce, candidate1Name, cand1Addr, cand1Addr, gasLimit, gasPrice)
	require.NoError(err)

	tests := []actionPayload{
		createStake,
		depositToStake,
		changeCandidate,
		unstake,
		withdrawStake,
		restake,
		transferStake,
		candidateRegister,
		candidateUpdate,
	}

	for _, test := range tests {
		bd := &EnvelopeBuilder{}
		elp := bd.SetNonce(1).
			SetAction(test).
			SetGasLimit(100000).Build()
		selp := FakeSeal(elp, identityset.PrivateKey(27).PublicKey())
		rlp, err := actionToRLP(selp.Action())

		require.NoError(err)

		require.Equal(elp.Nonce(), rlp.Nonce())
		require.Equal(elp.GasPrice(), rlp.GasPrice())
		require.Equal(elp.GasLimit(), rlp.GasLimit())
	}
}

func TestSealedEnvelope_Proto(t *testing.T) {
	req := require.New(t)
	se, err := createSealedEnvelope()
	req.NoError(err)
	tsf, ok := se.Envelope.Action().(*Transfer)
	req.True(ok)
	tsf.srcPubkey = se.SrcPubkey()
	proto := se.Proto()
	ac := &iotextypes.Action{
		Core:         se.Envelope.Proto(),
		SenderPubKey: se.srcPubkey.Bytes(),
		Signature:    se.signature,
	}
	req.Equal(ac, proto)

	se2 := SealedEnvelope{}
	for _, v := range []struct {
		encoding iotextypes.Encoding
		sig      []byte
		err      string
	}{
		{0, signByte, "invalid signature length ="},
		{3, validSig, "unknown encoding type"},
	} {
		se.encoding = v.encoding
		se.signature = v.sig
		req.Contains(se2.LoadProto(se.Proto()).Error(), v.err)
	}

	se.encoding = 1
	se.signature = validSig
	req.NoError(se.LoadProto(se.Proto()))
	tsf2, ok := se.Envelope.Action().(*Transfer)
	req.True(ok)
	req.Equal(tsf, tsf2)
}

func createSealedEnvelope() (SealedEnvelope, error) {
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

	cPubKey, err := crypto.HexStringToPublicKey(publicKey)
	se := SealedEnvelope{}
	se.Envelope = evlp
	se.srcPubkey = cPubKey
	se.signature = signByte
	return se, err
}
