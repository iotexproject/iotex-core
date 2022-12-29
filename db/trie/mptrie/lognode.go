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

type nodeType byte
type actionType byte

const (
	_nodeTypeLeaf      nodeType = 'l'
	_nodeTypeExtension nodeType = 'e'
	_nodeTypeBranch    nodeType = 'b'

	_actionTypeSearch actionType = 's'
	_actionTypeUpsert actionType = 'u'
	_actionTypeDelete actionType = 'd'
	_actionTypeNew    actionType = 'n'
)

// nodeEvent is the event of node
type nodeEvent struct {
	NodeType    nodeType
	ActionType  actionType
	KeyLen      uint8
	Key         []byte
	PathLen     uint8
	Path        []byte
	ChildrenLen uint8
	Children    []byte
	HashLen     uint8
	HashVal     []byte
}

// Bytes returns the bytes of node event
func (e nodeEvent) Bytes() []byte {
	b := make([]byte, 0, 1+1+1+e.KeyLen+1+e.PathLen+1+e.ChildrenLen+1+e.HashLen)
	b = append(b, byte(e.NodeType))
	b = append(b, byte(e.ActionType))
	b = append(b, e.KeyLen)
	b = append(b, e.Key...)
	b = append(b, e.PathLen)
	b = append(b, e.Path...)
	b = append(b, e.ChildrenLen)
	b = append(b, e.Children...)
	b = append(b, e.HashLen)
	b = append(b, e.HashVal...)
	return b
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

func logNode(nt nodeType, at actionType, n node, cli client) error {
	if !enabledLogMptrie {
		return nil
	}
	nodeKey, nodePath, nodeChildren, hashvalue, err := parseNode(n, cli)
	if err != nil {
		return err
	}
	event := nodeEvent{
		NodeType:    nt,
		ActionType:  at,
		KeyLen:      uint8(len(nodeKey)),
		Key:         nodeKey,
		PathLen:     uint8(len(nodePath)),
		Path:        nodePath,
		ChildrenLen: uint8(len(nodeChildren)),
		Children:    nodeChildren,
		HashLen:     uint8(len(hashvalue)),
		HashVal:     hashvalue,
	}
	// write event length
	if err = logWriter.WriteByte(byte(len(event.Bytes()))); err != nil {
		return err
	}
	// write event body
	_, err = logWriter.Write(event.Bytes())
	return err
}

func parseNode(n node, cli client) (nodeKey, nodePath, nodeChildren, hashvalue []byte, err error) {
	switch n := n.(type) {
	case *leafNode:
		nodeKey = n.key
		hashvalue, err = n.cacheNode.Hash(cli)
	case *extensionNode:
		nodePath = n.path
		hashvalue, err = n.cacheNode.Hash(cli)
	case *branchNode:
		nodeChildren = n.indices.List()
		hashvalue, err = n.cacheNode.Hash(cli)
	default:
		err = errors.Errorf("unknown node type %T", n)
	}
	return
}
