package mptrie

import (
	"bufio"
	"os"

	"github.com/pkg/errors"
)

var (
	enabledLogMptrie = false
	logFile          *os.File
	logWriter        *bufio.Writer
)

// NodeEvent is the event of node
type NodeEvent struct {
	Type        byte
	KeyLen      uint8
	Key         []byte
	PathLen     uint8
	Path        []byte
	ChildrenLen uint8
	Children    []byte
}

// Bytes returns the bytes of node event
func (e NodeEvent) Bytes() []byte {
	b := make([]byte, 0, 1+1+e.KeyLen+1+e.PathLen+1+e.ChildrenLen)
	b = append(b, e.Type)
	b = append(b, e.KeyLen)
	b = append(b, e.Key...)
	b = append(b, e.PathLen)
	b = append(b, e.Path...)
	b = append(b, e.ChildrenLen)
	b = append(b, e.Children...)
	return b
}

// ParseNodeEvent parse the node event
func ParseNodeEvent(b []byte) (NodeEvent, error) {
	if len(b) < 1 {
		return NodeEvent{}, errors.New("invalid node event")
	}
	event := NodeEvent{
		Type: b[0],
	}
	if len(b) < 2 {
		return event, nil
	}
	event.KeyLen = b[1]
	if len(b) < 2+int(event.KeyLen) {
		return event, nil
	}
	event.Key = b[2 : 2+event.KeyLen]
	if len(b) < 2+int(event.KeyLen)+1 {
		return event, nil
	}
	event.PathLen = b[2+event.KeyLen]
	if len(b) < 2+int(event.KeyLen)+1+int(event.PathLen) {
		return event, nil
	}
	event.Path = b[2+event.KeyLen+1 : 2+event.KeyLen+1+event.PathLen]
	if len(b) < 2+int(event.KeyLen)+1+int(event.PathLen)+1 {
		return event, nil
	}
	event.ChildrenLen = b[2+event.KeyLen+1+event.PathLen]
	if len(b) < 2+int(event.KeyLen)+1+int(event.PathLen)+1+int(event.ChildrenLen) {
		return event, nil
	}
	event.Children = b[2+event.KeyLen+1+event.PathLen+1 : 2+event.KeyLen+1+event.PathLen+1+event.ChildrenLen]
	return event, nil
}

// OpenLogDB open the log DB file
func OpenLogDB(dbPath string) error {
	var err error
	logFile, err = os.OpenFile(dbPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	logWriter = bufio.NewWriter(logFile)
	enabledLogMptrie = true
	return nil
}

// CloseLogDB close the log DB file
func CloseLogDB() error {
	if !enabledLogMptrie {
		return nil
	}
	if err := logWriter.Flush(); err != nil {
		return err
	}
	return logFile.Close()
}

func logNode(n node) error {
	if !enabledLogMptrie {
		return nil
	}
	nodeType, nodeKey, nodePath, nodeChildren, err := parseNode(n)
	if err != nil {
		return err
	}
	event := NodeEvent{
		Type:        nodeType,
		KeyLen:      uint8(len(nodeKey)),
		Key:         nodeKey,
		PathLen:     uint8(len(nodePath)),
		Path:        nodePath,
		ChildrenLen: uint8(len(nodeChildren)),
		Children:    nodeChildren,
	}
	// write events length
	if err = logWriter.WriteByte(byte(len(event.Bytes()))); err != nil {
		return err
	}
	// write events body
	_, err = logWriter.Write(event.Bytes())
	return err
}

func parseNode(n node) (nodeType byte, nodeKey []byte, nodePath []byte, nodeChildren []byte, err error) {
	switch n := n.(type) {
	case *hashNode:
		nodeType = 'h'
		nodeKey = n.hashVal
	case *leafNode:
		nodeType = 'l'
		nodeKey = n.key
	case *extensionNode:
		nodeType = 'e'
		nodePath = n.path
	case *branchNode:
		nodeType = 'b'
		nodeChildren = n.indices.List()
	default:
		err = errors.Errorf("unknown node type %T", n)
	}
	return
}
