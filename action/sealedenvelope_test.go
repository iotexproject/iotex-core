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
	_publicKey = "04403d3c0dbd3270ddfc248c3df1f9aafd60f1d8e7456961c9ef262" +
		"92262cc68f0ea9690263bef9e197a38f06026814fc70912c2b98d2e90a68f8ddc5328180a01"
	_evmNetworkID uint32 = 4689
)

var (
	_signByte    = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	_validSig, _ = hex.DecodeString("15e73ad521ec9e06600c59e49b127c9dee114ad64fb2fcbe5e0d9f4c8d2b766e73d708cca1dc050dd27b20f2ee607f30428bf035f45d4da8ec2fb04a90c2c30901")
)

func TestSealedEnvelope_Basic(t *testing.T) {
	req := require.New(t)
	for _, v := range []struct {
		id   uint32
		hash string
	}{
		{0, "322884fb04663019be6fb461d9453827487eafdd57b4de3bd89a7d77c9bf8395"},
		{1, "80af7840d73772d3022d8bdc46278fb755352e5e9d5f2a1f12ee7ec4f1ea98e9"},
	} {
		se, err := createSealedEnvelope(v.id)
		req.NoError(err)
		rHash, err := se.Hash()
		req.NoError(err)
		req.Equal(v.id, se.ChainID())
		req.Equal(v.hash, hex.EncodeToString(rHash[:]))
		req.Equal(_publicKey, se.SrcPubkey().HexString())
		req.Equal(_signByte, se.Signature())
		req.Zero(se.Encoding())

		var se1 SealedEnvelope
		se.signature = _validSig
		req.NoError(se1.loadProto(se.Proto(), _evmNetworkID))
		req.Equal(se.Envelope, se1.Envelope)
	}
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
	selp.encoding = iotextypes.Encoding_ETHEREUM_EIP155
	hash1, err := selp.envelopeHash()
	require.Equal(hash1, hash.ZeroHash256)
	require.Contains(err.Error(), "invalid action type")
}

func TestSealedEnvelope_Actions(t *testing.T) {
	require := require.New(t)

	createStake, err := NewCreateStake(uint64(10), _addr2, "100", uint32(10000), true, _payload, _gasLimit, _gasPrice)
	require.NoError(err)

	depositToStake, err := NewDepositToStake(1, 2, big.NewInt(10).String(), _payload, _gasLimit, _gasPrice)
	require.NoError(err)

	changeCandidate, err := NewChangeCandidate(1, _candidate1Name, 2, _payload, _gasLimit, _gasPrice)
	require.NoError(err)

	unstake, err := NewUnstake(_nonce, 2, _payload, _gasLimit, _gasPrice)
	require.NoError(err)

	withdrawStake, err := NewWithdrawStake(_nonce, 2, _payload, _gasLimit, _gasPrice)
	require.NoError(err)

	restake, err := NewRestake(_nonce, _index, _duration, _autoStake, _payload, _gasLimit, _gasPrice)
	require.NoError(err)

	transferStake, err := NewTransferStake(_nonce, _cand1Addr, 2, _payload, _gasLimit, _gasPrice)
	require.NoError(err)

	candidateRegister, err := NewCandidateRegister(_nonce, _candidate1Name, _cand1Addr, _cand1Addr, _cand1Addr, big.NewInt(10).String(), 91, true, _payload, _gasLimit, _gasPrice)
	require.NoError(err)

	candidateUpdate, err := NewCandidateUpdate(_nonce, _candidate1Name, _cand1Addr, _cand1Addr, _gasLimit, _gasPrice)
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
		act, ok := selp.Action().(EthCompatibleAction)
		require.True(ok)
		rlp, err := act.ToEthTx(0)
		require.NoError(err)

		require.Equal(elp.Nonce(), rlp.Nonce())
		require.Equal(elp.GasPrice(), rlp.GasPrice())
		require.Equal(elp.GasLimit(), rlp.Gas())
	}
}

func TestSealedEnvelope_Proto(t *testing.T) {
	req := require.New(t)
	se, err := createSealedEnvelope(0)
	req.NoError(err)
	tsf, ok := se.Envelope.Action().(*Transfer)
	req.True(ok)
	proto := se.Proto()
	ac := &iotextypes.Action{
		Core:         se.Envelope.Proto(),
		SenderPubKey: se.srcPubkey.Bytes(),
		Signature:    se.signature,
	}
	req.Equal(ac, proto)

	se2 := &SealedEnvelope{}
	for _, v := range []struct {
		encoding iotextypes.Encoding
		sig      []byte
		err      string
	}{
		{0, _signByte, "invalid signature length ="},
		{iotextypes.Encoding_ETHEREUM_UNPROTECTED + 1, _validSig, "unknown encoding type"},
	} {
		se.encoding = v.encoding
		se.signature = v.sig
		req.Contains(se2.loadProto(se.Proto(), _evmNetworkID).Error(), v.err)
	}

	for _, v := range []struct {
		enc  iotextypes.Encoding
		hash string
	}{
		{0, "0562e100b057804ee3cb4fa906a897852aa8075013a02ef1e229360f1e5ee339"},
		{1, "d5dc789026c12cc69f1ea7997fbe0aa1bcc02e85176848c7b2ecf4da6b4560d0"},
	} {
		se, err = createSealedEnvelope(0)
		se.signature = _validSig
		se.encoding = v.enc
		req.NoError(se2.loadProto(se.Proto(), _evmNetworkID))
		if v.enc > 0 {
			se.evmNetworkID = _evmNetworkID
		}
		h, _ := se.Hash()
		req.Equal(v.hash, hex.EncodeToString(h[:]))
		se.SenderAddress()
		_, _ = se2.Hash()
		se2.SenderAddress()
		req.Equal(se, se2)
		tsf2, ok := se2.Envelope.Action().(*Transfer)
		req.True(ok)
		req.Equal(tsf, tsf2)
	}
}

func createSealedEnvelope(chainID uint32) (*SealedEnvelope, error) {
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
		SetChainID(chainID).Build()

	cPubKey, err := crypto.HexStringToPublicKey(_publicKey)
	se := &SealedEnvelope{}
	se.Envelope = evlp
	se.srcPubkey = cPubKey
	se.signature = _signByte
	return se, err
}
