package p2p

import (
	"container/heap"
	"encoding/base64"
	"math/rand"
	"sort"
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
	   20 slices (maxIndexOfPiece), so each block is up to about 20M.
	   Considering the size of the buffer, there should be no more than 20 (maxItemForBroadcast) blocks assembled
	   	at the same time.
	*/
	maxItemForBroadcast = 20      //Up to 20 unfinished fragmented at the same time
	maxMessageBodySize  = 1047552 //1*1024*1024-1024
	maxIndexOfPiece     = 20      //about 20M
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
	bhs                []*broadcastHelper
	id2BroadcastHelper map[string]*broadcastHelper
}

func (h *broadcastHelperHeap) Len() int           { return len(h.bhs) }
func (h *broadcastHelperHeap) Less(i, j int) bool { return h.bhs[i].t.Before(h.bhs[j].t) }
func (h *broadcastHelperHeap) Swap(i, j int)      { h.bhs[i], h.bhs[j] = h.bhs[j], h.bhs[i] }
func (h *broadcastHelperHeap) Push(x interface{}) {
	bh := x.(*broadcastHelper)
	h.bhs = append(h.bhs, bh)
	h.id2BroadcastHelper[bh.broadcast.MessageId] = bh
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
			if bh.t.After(time.Now().Add(0 - time.Minute)) {
				break
			}
			delete(h.id2BroadcastHelper, bh.broadcast.MessageId)
			heap.Pop(h)
		}
		//remove message if too many items
		if len(h.bhs) > maxItemForBroadcast {
			bh := heap.Remove(h, 0) //移除第一个,时间最久的那个
			delete(h.id2BroadcastHelper, bh.(*broadcastHelper).broadcast.MessageId)
		}
	}()
	if msg.MessageId == "" {
		return msg
	}
	if int(msg.IndexOfPiece) > maxIndexOfPiece {
		return nil
	}
	bh := h.id2BroadcastHelper[msg.MessageId]
	if bh == nil {
		bh = &broadcastHelper{
			broadcast: msg,
			bodies:    make(map[int][]byte),
			t:         time.Now(),
		}
		if !msg.HasMore {
			bh.maxIndex = int(msg.IndexOfPiece)
		}
		bh.bodies[int(msg.IndexOfPiece)] = msg.MsgBody
		h.bhs = append(h.bhs, bh) //it must be the latest
		h.id2BroadcastHelper[msg.MessageId] = bh
	} else {
		ol := len(bh.bodies)
		bh.bodies[int(msg.IndexOfPiece)] = msg.MsgBody
		//ignore duplicate message time update
		if len(bh.bodies) <= ol {
			return nil
		}
		if !msg.HasMore {
			bh.maxIndex = int(msg.IndexOfPiece)
		}
		bh.t = time.Now()
		sort.Sort(h)
		//all the fragment arrive
		if len(bh.bodies) == bh.maxIndex+1 {
			bh.broadcast.MsgBody = nil
			for i := 0; i < len(bh.bodies); i++ {
				bh.broadcast.MsgBody = append(bh.broadcast.MsgBody, bh.bodies[i]...)
			}
			delete(h.id2BroadcastHelper, bh.broadcast.MessageId)
			return bh.broadcast
		}
	}
	return nil
}
