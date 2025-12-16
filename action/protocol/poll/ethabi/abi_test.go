package ethabi

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/vote"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestCandidateABI(t *testing.T) {
	r := require.New(t)

	check := func(calldata []byte, method *abi.Method, expectParam *protocol.Parameters) {
		sc, err := BuildReadStateRequest(append(method.ID, calldata...))
		r.NoError(err)
		r.EqualValues(expectParam.MethodName, sc.Parameters().MethodName)
		r.ElementsMatch(expectParam.Arguments, sc.Parameters().Arguments)
		height := uint64(12345)
		cands := state.CandidateList{
			{
				Address:       identityset.Address(1).String(),
				Votes:         big.NewInt(100),
				RewardAddress: identityset.Address(2).String(),
				BLSPubKey:     []byte("bls-pub-key-1"),
			},
			{
				Address:       identityset.Address(3).String(),
				Votes:         big.NewInt(200),
				RewardAddress: identityset.Address(4).String(),
				BLSPubKey:     []byte("bls-pub-key-2"),
			},
		}
		candData, err := cands.Serialize()
		r.NoError(err)
		expect, err := ConvertCandidateListFromState(cands)
		r.NoError(err)
		out, err := sc.EncodeToEth(&iotexapi.ReadStateResponse{
			Data: candData,
			BlockIdentifier: &iotextypes.BlockIdentifier{
				Height: height,
			},
		})
		r.NoError(err)

		var getResult struct {
			Candidates []Candidate
			Height     *big.Int
		}
		data, err := hex.DecodeString(out)
		r.NoError(err)
		err = pollABI.UnpackIntoInterface(&getResult, method.Name, data)
		r.NoError(err)

		r.Equal(height, getResult.Height.Uint64())
		r.ElementsMatch(expect, getResult.Candidates)
	}

	t.Run("Candidates", func(t *testing.T) {
		data, err := pollCandidatesMethod.Inputs.Pack()
		r.NoError(err)
		check(data, pollCandidatesMethod, &protocol.Parameters{MethodName: []byte("CandidatesByEpoch"), Arguments: nil})
	})
	t.Run("CandidatesByEpoch", func(t *testing.T) {
		epoch := big.NewInt(100)
		data, err := pollCandidatesByEpochMethod.Inputs.Pack(epoch)
		r.NoError(err)
		args := [][]byte{[]byte(epoch.Text(10))}
		check(data, pollCandidatesByEpochMethod, &protocol.Parameters{MethodName: []byte("CandidatesByEpoch"), Arguments: args})
	})
	t.Run("BlockProducers", func(t *testing.T) {
		data, err := pollBlockProducersMethod.Inputs.Pack()
		r.NoError(err)
		check(data, pollBlockProducersMethod, &protocol.Parameters{MethodName: []byte("BlockProducersByEpoch"), Arguments: nil})
	})
	t.Run("BlockProducersByEpoch", func(t *testing.T) {
		data, err := pollBlockProducersMethod.Inputs.Pack()
		r.NoError(err)
		check(data, pollBlockProducersMethod, &protocol.Parameters{MethodName: []byte("BlockProducersByEpoch"), Arguments: nil})
	})
	t.Run("ActiveBlockProducers", func(t *testing.T) {
		data, err := pollActiveBlockProducersMethod.Inputs.Pack()
		r.NoError(err)
		check(data, pollActiveBlockProducersMethod, &protocol.Parameters{MethodName: []byte("ActiveBlockProducersByEpoch"), Arguments: nil})
	})
	t.Run("ActiveBlockProducersByEpoch", func(t *testing.T) {
		epoch := big.NewInt(100)
		data, err := pollActiveBlockProducersByEpochMethod.Inputs.Pack(epoch)
		r.NoError(err)
		args := [][]byte{[]byte(epoch.Text(10))}
		check(data, pollActiveBlockProducersByEpochMethod, &protocol.Parameters{MethodName: []byte("ActiveBlockProducersByEpoch"), Arguments: args})
	})
}

func TestProbationABI(t *testing.T) {
	r := require.New(t)

	check := func(calldata []byte, method *abi.Method, expectParam *protocol.Parameters) {
		sc, err := BuildReadStateRequest(append(method.ID, calldata...))
		r.NoError(err)
		r.EqualValues(expectParam.MethodName, sc.Parameters().MethodName)
		r.ElementsMatch(expectParam.Arguments, sc.Parameters().Arguments)
		height := uint64(12345)
		pl := vote.ProbationList{
			ProbationInfo: map[string]uint32{
				identityset.Address(1).String(): 3,
				identityset.Address(2).String(): 5,
			},
			IntensityRate: 10,
		}
		plData, err := pl.Serialize()
		r.NoError(err)
		expect, err := ConvertProbationListFromState(&pl)
		r.NoError(err)
		out, err := sc.EncodeToEth(&iotexapi.ReadStateResponse{
			Data: plData,
			BlockIdentifier: &iotextypes.BlockIdentifier{
				Height: height,
			},
		})
		r.NoError(err)

		var getResult struct {
			Probation ProbationList `json:"probation"`
			Height    *big.Int      `json:"height"`
		}
		data, err := hex.DecodeString(out)
		r.NoError(err)
		err = pollABI.UnpackIntoInterface(&getResult, method.Name, data)
		r.NoError(err)

		r.Equal(height, getResult.Height.Uint64())
		r.EqualValues(expect.IntensityRate, getResult.Probation.IntensityRate)
		r.ElementsMatch(expect.ProbationInfo, getResult.Probation.ProbationInfo)
	}

	t.Run("ProbationList", func(t *testing.T) {
		data, err := pollProbationListMethod.Inputs.Pack()
		r.NoError(err)
		check(data, pollProbationListMethod, &protocol.Parameters{MethodName: []byte("ProbationListByEpoch"), Arguments: nil})
	})
	t.Run("ProbationListByEpoch", func(t *testing.T) {
		epoch := big.NewInt(100)
		data, err := pollProbationListByEpochMethod.Inputs.Pack(epoch)
		r.NoError(err)
		args := [][]byte{[]byte(epoch.Text(10))}
		check(data, pollProbationListByEpochMethod, &protocol.Parameters{MethodName: []byte("ProbationListByEpoch"), Arguments: args})
	})
}
