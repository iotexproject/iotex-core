package p2p

import (
	"container/heap"
	"encoding/base64"
	"math/rand"
	"time"

	"github.com/iotexproject/iotex-core/protogen/iotexrpc"
)

/*
libp2p doesn't allow broadcast message which size is bigger than 1M size.
These messages should be fragmented to ensure no more than 1M size.
*/

const (
	/*
	   When broadcasting a data packet, the size of each data packet cannot exceed 1M, but the size of the block is
	   likely to exceed 1M, so it must be fragmented. Due to security concerns, the size of each block cannot be
	   infinitely large, and is directly limited to approximately 20M.
	   Each broadcast packet can be up to about 1M (maxMessageBodySize), and each block can be broadcast with up to
	   20 slices (maxIndexOfFragment), so each block is up to about 20M.
	   Considering the size of the buffer, there should be no more than 20 (maxItemForBroadcast) blocks assembled
	   	at the same time.
	*/
	maxItemForBroadcast = 20          //Up to 20 unfinished fragmented at the same time
	maxMessageBodySize  = 1047552     //1*1024*1024-1024
	maxIndexOfFragment  = 20          //about 20M
	maxItemLifeTime     = time.Minute // if cannot receive the whole message in one minute,discard the message
)

func generateMessageID() string {
	//RandSrc random source from math
	var RandSrc = rand.NewSource(time.Now().UnixNano())
	b := make([]byte, 20)
	rand.New(RandSrc).Read(b)
	return base64.StdEncoding.EncodeToString(b)
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

//order message by arriving time
type broadcastHelper struct {
	broadcast *iotexrpc.BroadcastMsg
	maxIndex  int
	bodies    map[int][]byte //index to data
	t         time.Time      //last data arrive time
}

type broadcastHelperHeap struct {
	bhs []*broadcastHelper
}

func (h *broadcastHelperHeap) Len() int           { return len(h.bhs) }
func (h *broadcastHelperHeap) Less(i, j int) bool { return h.bhs[i].t.Before(h.bhs[j].t) }
func (h *broadcastHelperHeap) Swap(i, j int)      { h.bhs[i], h.bhs[j] = h.bhs[j], h.bhs[i] }
func (h *broadcastHelperHeap) Push(x interface{}) {
	bh := x.(*broadcastHelper)
	h.bhs = append(h.bhs, bh)
}

func (h *broadcastHelperHeap) Pop() interface{} {
	n := len(h.bhs)
	x := h.bhs[n-1]
	h.bhs = h.bhs[0 : n-1]
	return x
}

//AddMessage add a new piece of message
func (h *broadcastHelperHeap) AddMessage(msg *iotexrpc.BroadcastMsg) *iotexrpc.BroadcastMsg {
	defer func() {
		//remove message that is too old
		for len(h.bhs) > 0 {
			bh := h.bhs[0]
			//remove item that is arrived at 1 minute ago
			if bh.t.After(time.Now().Add(0 - maxItemLifeTime)) {
				break
			}
			heap.Pop(h)
		}
		//remove message if too many items
		if len(h.bhs) > maxItemForBroadcast {
			heap.Remove(h, 0) //remove the oldest one
		}
	}()
	if msg.MessageId == "" {
		return msg
	}
	if int(msg.IndexOfFrag) > maxIndexOfFragment {
		return nil
	}
	if len(msg.MsgBody) > maxMessageBodySize {
		return nil
	}
	var index = -1
	var bh *broadcastHelper
	for i := range h.bhs {
		if h.bhs[i].broadcast.MessageId == msg.MessageId {
			index = i
			break
		}
	}
	if index < 0 {
		//new broadcast message
		bh = &broadcastHelper{
			broadcast: msg,
			bodies:    make(map[int][]byte),
			t:         time.Now(),
		}
		bh.bodies[int(msg.IndexOfFrag)] = msg.MsgBody
		h.bhs = append(h.bhs, bh)
	} else {
		bh = h.bhs[index]
		ol := len(bh.bodies)
		bh.bodies[int(msg.IndexOfFrag)] = msg.MsgBody
		//ignore duplicate message time update
		if len(bh.bodies) <= ol {
			return nil
		}
		if !msg.HasMore {
			bh.maxIndex = int(msg.IndexOfFrag)
		}
		bh.t = time.Now()
		//all the fragment arrive
		if len(bh.bodies) == bh.maxIndex+1 {
			bh.broadcast.MsgBody = nil
			for i := 0; i < len(bh.bodies); i++ {
				//The other party may maliciously send  a message numbered [0, 2, 3, 4],
				// which will only cause a message error and will not cause the program to crash.
				bh.broadcast.MsgBody = append(bh.broadcast.MsgBody, bh.bodies[i]...)
			}
			heap.Remove(h, index)
			return bh.broadcast
		} else {
			heap.Fix(h, index)
		}
	}
	return nil
}
