package state

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type linkedListNode[T any] struct {
	data     T
	nextAddr string
}

// DecodeOrderedKvList decodes a list of key-value pairs representing a linked list into an ordered slice
func DecodeOrderedKvList[T any](keys [][]byte, values []systemcontracts.GenericValue, decoder func(k []byte, v systemcontracts.GenericValue) (T, string, string, error)) ([]T, error) {
	nodeMap, err := buildCandidateMap(keys, values, decoder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build candidate map")
	}

	headAddr, err := findLinkedListHead(nodeMap)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find head of candidate list")
	}

	result, err := traverseLinkedList(headAddr, nodeMap)
	if err != nil {
		return nil, errors.Wrap(err, "failed to traverse candidate linked list")
	}

	return result, nil
}

// buildCandidateMap builds a generic map from address bytes to candidate nodes
func buildCandidateMap[T any](
	keys [][]byte,
	values []systemcontracts.GenericValue,
	decoder func(k []byte, v systemcontracts.GenericValue) (T, string, string, error),
) (map[string]*linkedListNode[T], error) {
	nodeMap := make(map[string]*linkedListNode[T], len(keys))

	for kid, gv := range values {
		data, id, next, err := decoder(keys[kid], gv)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode candidate data")
		}
		nodeMap[id] = &linkedListNode[T]{
			data:     data,
			nextAddr: next,
		}
	}

	return nodeMap, nil
}

// findLinkedListHead finds the head of the linked list (not pointed to by any node)
func findLinkedListHead[T any](nodeMap map[string]*linkedListNode[T]) (string, error) {
	// Mark all addresses that are pointed to
	pointedTo := make(map[string]bool, len(nodeMap))
	for _, node := range nodeMap {
		if len(node.nextAddr) > 0 {
			pointedTo[node.nextAddr] = true
		}
	}

	// Find the address that is not pointed to by any node
	for addrKey := range nodeMap {
		if !pointedTo[addrKey] {
			return addrKey, nil
		}
	}

	return "", errors.New("failed to find head of candidate list")
}

// traverseLinkedList traverses the linked list and returns an ordered list
func traverseLinkedList[T any](headAddr string, nodeMap map[string]*linkedListNode[T]) ([]T, error) {
	result := make([]T, 0, len(nodeMap))
	visited := make(map[string]bool, len(nodeMap))
	currentAddr := headAddr

	for currentAddr != "" {
		addrKey := currentAddr

		// Check for circular reference
		if visited[addrKey] {
			return nil, errors.New("circular reference detected in candidate list")
		}
		visited[addrKey] = true

		// Get current node
		node, exists := nodeMap[addrKey]
		if !exists {
			return nil, errors.Errorf("missing candidate for address %x in linked list", currentAddr)
		}

		result = append(result, node.data)
		currentAddr = node.nextAddr
	}

	// Verify all nodes were traversed
	if len(result) != len(nodeMap) {
		return nil, errors.Errorf("incomplete traversal: %d/%d candidates", len(result), len(nodeMap))
	}

	return result, nil
}
