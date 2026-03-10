// Package main provides hardcoded test contract bytecode for L3 testing.
// These are pre-compiled offline — no Solidity compiler needed at runtime.
package main

import "encoding/hex"

func hexDecode(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}

// counterRuntime is the runtime bytecode for a simple counter contract.
// It always increments slot 0 regardless of calldata:
//
//	PUSH1 0x00 SLOAD  (load slot 0)
//	PUSH1 0x01 ADD    (add 1)
//	PUSH1 0x00 SSTORE (store to slot 0)
//	STOP
var counterRuntime = hexDecode("6000546001016000550" + "0")

// incrementSelector is the 4-byte function selector for increment().
var incrementSelector = hexDecode("d09de08a")

// testContracts is the list of deployed test contracts for L3 testing.
type testContract struct {
	Address  string
	Code     []byte
	InitSlot string // initial storage slot 0 value (hex)
}

var testContracts = []testContract{
	{
		Address:  "0xCCCC000000000000000000000000000000000001",
		Code:     counterRuntime,
		InitSlot: "0x0000000000000000000000000000000000000000000000000000000000000000",
	},
}
