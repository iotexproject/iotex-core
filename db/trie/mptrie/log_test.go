package mptrie

import (
	"testing"

	"github.com/iotexproject/iotex-core/v2/testutil"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestNodeEvent(t *testing.T) {
	require := require.New(t)
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
		require.NoError(err)
		require.Equal(test.event.NodeType, event.NodeType)
		require.Equal(test.event.ActionType, event.ActionType)
		require.Equal(test.event.KeyLen, event.KeyLen)
		require.Equal(test.event.Key, event.Key)
		require.Equal(test.event.PathLen, event.PathLen)
		require.Equal(test.event.Path, event.Path)
		require.Equal(test.event.ChildrenLen, event.ChildrenLen)
		require.Equal(test.event.Children, event.Children)
		require.Equal(test.event.HashLen, event.HashLen)
		require.Equal(test.event.HashVal, event.HashVal)
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
