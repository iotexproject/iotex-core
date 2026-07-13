// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package autodeposit

// abiJSON is a trimmed subset of the AutoDeposit contract's ABI. Only the
// entries the bridge actually calls are kept — currently the read-only
// bucket(address) getter. The full ABI (register/unregister/pause/paused
// /registrants/buckets/owner) lives in iotex-analyser under
// plugins/hermes/hermes_abi.go's AutoDepositABI and is what Hermes uses
// off-chain; this bridge only needs to consult the compound-preference
// mapping, so we keep the on-chain surface minimal.
const abiJSON = `[
	{
		"constant": true,
		"inputs": [
			{
				"internalType": "address",
				"name": "owner",
				"type": "address"
			}
		],
		"name": "bucket",
		"outputs": [
			{
				"internalType": "int256",
				"name": "",
				"type": "int256"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}
]`
