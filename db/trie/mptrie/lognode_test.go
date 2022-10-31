package mptrie

import (
	"os"
	"testing"
)

func TestNodeEvent(t *testing.T) {
	tests := []struct {
		event NodeEvent
	}{
		{
			event: NodeEvent{
				Type:        1,
				KeyLen:      2,
				Key:         []byte{3, 4},
				PathLen:     5,
				Path:        []byte{6, 7, 8, 9, 10},
				ChildrenLen: 11,
				Children:    []byte{12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22},
			},
		},
		{
			event: NodeEvent{
				Type:        6,
				KeyLen:      6,
				Key:         []byte("123456"),
				PathLen:     6,
				Path:        []byte("abcdef"),
				ChildrenLen: 6,
				Children:    []byte("ABCDEF"),
			},
		},
	}

	for _, test := range tests {
		b := test.event.Bytes()
		event, err := ParseNodeEvent(b)
		if err != nil {
			t.Errorf("unexpected error %v", err)
		}
		if event.Type != test.event.Type {
			t.Errorf("unexpected type %v", event.Type)
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
	}
}

func TestLogDB(t *testing.T) {
	dbPath := "test_log_db"
	defer func() {
		_ = os.RemoveAll(dbPath)
	}()
	if err := OpenLogDB(dbPath); err != nil {
		t.Errorf("unexpected open error %v", err)
	}
	if err := CloseLogDB(); err != nil {
		t.Errorf("unexpected close error %v", err)
	}

}
