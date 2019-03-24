package p2p

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/protogen/iotexrpc"
)

// sliceShuffle shuffles a slice.
func sliceShuffle(slice []interface{}) []interface{} {
	for i := 0; i < len(slice); i++ {
		a := rand.Intn(len(slice))
		b := rand.Intn(len(slice))
		slice[a], slice[b] = slice[b], slice[a]
	}
	return slice
}
func TestBroadcastHelperHeap_AddMessage(t *testing.T) {
	ast := assert.New(t)
	expected := []byte{1, 2, 3, 4, 5, 6}
	msgBase := iotexrpc.BroadcastMsg{
		ChainId:   0,
		PeerId:    "foo",
		MessageId: "a",
	}
	var frags = make([]interface{}, len(expected))
	for i, b := range expected {
		msg := msgBase
		msg.IndexOfFrag = uint32(i)
		msg.HasMore = i != len(expected)-1
		msg.MsgBody = []byte{b}
		frags[i] = msg
	}
	frags = sliceShuffle(frags)
	h := broadcastHelperHeap{}
	var result *iotexrpc.BroadcastMsg
	for i := range frags {
		msg := frags[i].(iotexrpc.BroadcastMsg)
		result = h.AddMessage(&msg)
		if result != nil {
			break
		}
	}
	ast.NotNil(result)
	ast.EqualValues(result.MsgBody, expected)

	//maxItermTest
	for i := 0; i < maxItemForBroadcast+1; i++ {
		msg := msgBase
		msg.PeerId = fmt.Sprintf("%d", i)
		msg.MessageId = fmt.Sprintf("%d", i)
		h.AddMessage(&msg)
	}
	ast.EqualValues(len(h.bhs), maxItemForBroadcast)

	//body size
	msg := msgBase
	msg.MsgBody = make([]byte, maxMessageBodySize+1)
	result = h.AddMessage(&msg)
	ast.Nil(result)

	//msg fragment
	msg = msgBase
	msg.IndexOfFrag = maxIndexOfFragment + 1
	result = h.AddMessage(&msg)
	ast.Nil(result)

	//malicious message,message body has error,and errors will be deteced by upper call.
	//frag 0
	msg = msgBase
	msg.MessageId = "test3"
	msg.MsgBody = make([]byte, 3)
	msg.HasMore = true
	msg.IndexOfFrag = 0
	result = h.AddMessage(&msg)
	ast.Nil(result)

	//frag 2
	msg2 := msg
	msg2.IndexOfFrag = 2
	msg2.HasMore = false
	result = h.AddMessage(&msg2)
	ast.Nil(result)

	//frag 3
	msg3 := msg
	msg3.IndexOfFrag = 3
	msg3.HasMore = true
	result = h.AddMessage(&msg3)
	ast.NotNil(result)

}
