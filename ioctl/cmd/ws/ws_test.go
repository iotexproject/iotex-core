package ws

import (
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
)

func Test_getEventInputsByName(t *testing.T) {
	var (
		eventName  = "ProjectUpserted"
		eventTopic = []byte{136, 157, 97, 20, 123, 253, 22, 248, 153, 111, 242, 89, 226, 42, 196, 66, 136, 167, 214, 250, 9, 87, 132, 77, 185, 136, 240, 113, 183, 136, 60, 35}
	)

	r := require.New(t)

	inputs := wsProjectRegisterContractABI.Events[eventName].Inputs.NonIndexed()
	data, err := inputs.Pack("http://a.b.c", [32]byte{})
	r.NoError(err)

	_, err = inputs.Unpack(data)
	r.NoError(err)

	t.Run("FailedToGetEventABI_InvalidTopic", func(t *testing.T) {
		_, err = getEventInputsByName([]*iotextypes.Log{{
			Topics: [][]byte{{}},
		}}, "any")
		r.Error(err)
	})
	t.Run("NotParsedTargetEvent_EmptyLogs", func(t *testing.T) {
		_, err := getEventInputsByName([]*iotextypes.Log{}, "any")
		r.Error(err)
	})
	t.Run("FailedToUnpackIntoMap_InvalidLogData", func(t *testing.T) {
		_, err = getEventInputsByName([]*iotextypes.Log{{
			Topics: [][]byte{eventTopic},
			Data:   make([]byte, 10),
		}}, eventName)
		r.Error(err)
	})
	t.Run("FailedToParseTopicsIntoMap", func(t *testing.T) {
		_, err = getEventInputsByName([]*iotextypes.Log{{
			Topics: [][]byte{eventTopic},
			Data:   data,
		}}, eventName)
		r.Error(err)
	})
	t.Run("Succeed", func(t *testing.T) {
		_, err = getEventInputsByName([]*iotextypes.Log{{
			Topics: [][]byte{eventTopic, append(make([]byte, 31), byte(11))},
			Data:   data,
		}}, eventName)
		r.NoError(err)
	})
}
