package action

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestEnvelope_Basic(t *testing.T) {
	req := require.New(t)
	evlp, tsf := createEnvelope()
	req.Equal(uint32(1), evlp.Version())
	req.Equal(uint64(10), evlp.Nonce())
	req.Equal(uint64(20010), evlp.Gas())
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

	proto := evlp.Proto()
	actCore := &iotextypes.ActionCore{
		Version:  evlp.version,
		Nonce:    evlp.nonce,
		GasLimit: evlp.gasLimit,
		ChainID:  evlp.chainID,
	}
	actCore.GasPrice = evlp.gasPrice.String()
	actCore.Action = &iotextypes.ActionCore_Transfer{Transfer: tsf.Proto()}
	req.Equal(actCore, proto)

	evlp2 := &envelope{}
	req.NoError(evlp2.LoadProto(proto))
	req.Equal(evlp.version, evlp2.version)
	req.Equal(evlp.chainID, evlp2.chainID)
	req.Equal(evlp.nonce, evlp2.nonce)
	req.Equal(evlp.gasLimit, evlp2.gasLimit)
	req.Equal(evlp.gasPrice, evlp2.gasPrice)
	tsf2, ok := evlp2.Action().(*Transfer)
	req.True(ok)
	req.Equal(tsf.amount, tsf2.amount)
	req.Equal(tsf.recipient, tsf2.recipient)
	req.Equal(tsf.payload, tsf2.payload)
}

func TestEnvelope_Actions(t *testing.T) {
	require := require.New(t)
	candidates := state.CandidateList{}
	putPollResult := NewPutPollResult(10001, candidates)

	createStake, err := NewCreateStake(_addr2, "100", uint32(10000), true, _payload)
	require.NoError(err)

	depositToStake, err := NewDepositToStake(2, big.NewInt(10).String(), _payload)
	require.NoError(err)

	changeCandidate := NewChangeCandidate(_candidate1Name, 2, _payload)
	unstake := NewUnstake(2, _payload)
	withdrawStake := NewWithdrawStake(2, _payload)

	restake := NewRestake(_index, _duration, _autoStake, _payload)
	require.NoError(err)

	transferStake, err := NewTransferStake(_cand1Addr, 2, _payload)
	require.NoError(err)

	candidateRegister, err := NewCandidateRegister(_candidate1Name, _cand1Addr, _cand1Addr, _cand1Addr, big.NewInt(10).String(), 91, true, _payload)
	require.NoError(err)

	candidateUpdate, err := NewCandidateUpdate(_candidate1Name, _cand1Addr, _cand1Addr)
	require.NoError(err)

	grantReward := NewGrantReward(BlockReward, 2)
	claimFromRewardingFund := NewClaimFromRewardingFund(big.NewInt(1), nil, nil)
	depositToRewardingFund := NewDepositToRewardingFund(big.NewInt(1), nil)

	tests := []actionPayload{
		putPollResult,
		createStake,
		depositToStake,
		changeCandidate,
		unstake,
		withdrawStake,
		restake,
		transferStake,
		candidateRegister,
		candidateUpdate,
		grantReward,
		claimFromRewardingFund,
		depositToRewardingFund,
	}

	for _, test := range tests {
		bd := &EnvelopeBuilder{}
		elp := bd.SetNonce(1).SetGasLimit(_gasLimit).SetGasPrice(_gasPrice).
			SetAction(test).SetChainID(1).Build()
		evlp, ok := elp.(*envelope)
		require.True(ok)
		require.NoError(evlp.LoadProto(evlp.Proto()))
		require.Equal(elp.Version(), evlp.Version())
		require.Equal(elp.Nonce(), evlp.Nonce())
		require.Equal(elp.ChainID(), evlp.ChainID())
		require.Equal(elp.GasPrice(), evlp.GasPrice())
		require.Equal(elp.Gas(), evlp.Gas())
	}
}

func createEnvelope() (Envelope, *Transfer) {
	tsf := NewTransfer(unit.ConvertIotxToRau(1000+int64(10)),
		identityset.Address(10%identityset.Size()).String(),
		nil)
	evlp := (&EnvelopeBuilder{}).SetAction(tsf).SetGasLimit(20010).
		SetGasPrice(unit.ConvertIotxToRau(11)).
		SetNonce(10).SetVersion(1).SetChainID(1).Build()
	return evlp, tsf
}
