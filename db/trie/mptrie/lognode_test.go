package mptrie

import (
	"testing"

	"github.com/iotexproject/iotex-core/testutil"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestNodeEvent(t *testing.T) {
	tests := []struct {
		event nodeEvent
	}{
		{
			event: nodeEvent{
				NodeType:    _nodeTypeBranch,
				ActionType:  _actionTypeNew,
				KeyLen:      2,
				Key:         []byte{3, 4},
				PathLen:     5,
				Path:        []byte{6, 7, 8, 9, 10},
				ChildrenLen: 11,
				Children:    []byte{12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22},
				HashLen:     23,
				HashVal:     []byte{24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46},
			},
		},
		{
			event: nodeEvent{
				NodeType:    _nodeTypeLeaf,
				ActionType:  _actionTypeSearch,
				KeyLen:      6,
				Key:         []byte("123456"),
				PathLen:     6,
				Path:        []byte("abcdef"),
				ChildrenLen: 6,
				Children:    []byte("ABCDEF"),
				HashLen:     6,
				HashVal:     []byte("abcdef"),
			},
		},
	}

	for _, test := range tests {
		b := test.event.Bytes()
		event, err := parseNodeEvent(b)
		if err != nil {
			t.Errorf("unexpected error %v", err)
		}
		if event.NodeType != test.event.NodeType {
			t.Errorf("unexpected NodeType %v", event.NodeType)
		}
		if event.ActionType != test.event.ActionType {
			t.Errorf("unexpected ActionType %v", event.ActionType)
		}
		if event.KeyLen != test.event.KeyLen {
			t.Errorf("unexpected key length %v", event.KeyLen)
		}
		if event.Key != nil && string(event.Key) != string(test.event.Key) {
			t.Errorf("unexpected key %v", event.Key)
		}
		if event.PathLen != test.event.PathLen {
			t.Errorf("unexpected path length %v", event.PathLen)
		}
		if event.Path != nil && string(event.Path) != string(test.event.Path) {
			t.Errorf("unexpected path %v", event.Path)
		}
		if event.ChildrenLen != test.event.ChildrenLen {
			t.Errorf("unexpected children length %v", event.ChildrenLen)
		}
		if event.Children != nil && string(event.Children) != string(test.event.Children) {
			t.Errorf("unexpected children %v", event.Children)
		}
		if event.HashLen != test.event.HashLen {
			t.Errorf("unexpected hash length %v", event.HashLen)
		}
		if event.HashVal != nil && string(event.HashVal) != string(test.event.HashVal) {
			t.Errorf("unexpected hash %v", event.HashVal)
		}
	}
}

// parseNodeEvent parse the node event
func parseNodeEvent(b []byte) (nodeEvent, error) {
	if len(b) < 1 {
		return nodeEvent{}, errors.New("invalid node event")
	}
	event := nodeEvent{
		NodeType: nodeType(b[0]),
	}
	if len(b) < 2 {
		return event, nil
	}
	event.ActionType = actionType(b[1])
	if len(b) < 3 {
		return event, nil
	}
	event.KeyLen = uint8(b[2])
	if len(b) < 3+int(event.KeyLen) {
		return event, nil
	}
	event.Key = b[3 : 3+event.KeyLen]
	if len(b) < 3+int(event.KeyLen)+1 {
		return event, nil
	}
	event.PathLen = b[3+event.KeyLen]
	if len(b) < 3+int(event.KeyLen)+1+int(event.PathLen) {
		return event, nil
	}
	event.Path = b[3+event.KeyLen+1 : 3+event.KeyLen+1+event.PathLen]
	if len(b) < 3+int(event.KeyLen)+1+int(event.PathLen)+1 {
		return event, nil
	}
	event.ChildrenLen = b[3+event.KeyLen+1+event.PathLen]
	if len(b) < 3+int(event.KeyLen)+1+int(event.PathLen)+1+int(event.ChildrenLen) {
		return event, nil
	}
	event.Children = b[3+event.KeyLen+1+event.PathLen+1 : 3+event.KeyLen+1+event.PathLen+1+event.ChildrenLen]
	if len(b) < 3+int(event.KeyLen)+1+int(event.PathLen)+1+int(event.ChildrenLen)+1 {
		return event, nil
	}
	event.HashLen = b[3+event.KeyLen+1+event.PathLen+1+event.ChildrenLen]
	if len(b) < 3+int(event.KeyLen)+1+int(event.PathLen)+1+int(event.ChildrenLen)+1+int(event.HashLen) {
		return event, nil
	}
	event.HashVal = b[3+event.KeyLen+1+event.PathLen+1+event.ChildrenLen+1 : 3+event.KeyLen+1+event.PathLen+1+event.ChildrenLen+1+event.HashLen]
	return event, nil
}

func TestLogDB(t *testing.T) {
	require := require.New(t)
	testPath, err := testutil.PathOfTempFile("test-log-db")
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()
	require.NoError(OpenLogDB(testPath))
	require.NoError(CloseLogDB())
	enabledLogMptrie = false
}
