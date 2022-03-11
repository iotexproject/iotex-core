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

	evlp.SetNonce(nonce)
	req.Equal(nonce, evlp.Nonce())
	evlp.SetChainID(tsf.chainID)
	req.Equal(tsf.chainID, evlp.ChainID())
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
	putPollResult := NewPutPollResult(1, 10001, candidates)

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

	gb := GrantRewardBuilder{}
	grantReward := gb.Build()

	cb := ClaimFromRewardingFundBuilder{}
	claimFromRewardingFund := cb.SetAmount(big.NewInt(1)).Build()

	cf := DepositToRewardingFundBuilder{}
	depositToRewardingFund := cf.SetAmount(big.NewInt(1)).Build()

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
		&grantReward,
		&claimFromRewardingFund,
		&depositToRewardingFund,
	}

	for _, test := range tests {
		bd := &EnvelopeBuilder{}
		elp := bd.SetNonce(1).
			SetAction(test).
			SetGasLimit(100000).
			Build()
		evlp, ok := elp.(*envelope)
		require.True(ok)
		err = evlp.LoadProto(evlp.Proto())

		require.NoError(err)
		require.Equal(elp.Version(), evlp.Version())
		require.Equal(elp.Nonce(), evlp.Nonce())
		require.Equal(elp.ChainID(), evlp.ChainID())
		require.Equal(elp.GasPrice(), evlp.GasPrice())
		require.Equal(elp.GasLimit(), evlp.GasLimit())
	}
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
		SetChainID(1).
		Build()
	return evlp, tsf
}
