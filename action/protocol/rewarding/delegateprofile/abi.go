// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package delegateprofile

// abiJSON is the minimal ABI subset needed to read commission portions from
// the existing DelegateProfile contract deployed at:
//
//	mainnet:  0xfa7f50866ac45d84adf54bc767c885f92750e258
//	testnet:  0xd19ffB48a5C18B77c541D32c1B1ac2440c287774
//
// The bridge only ever performs read-only view calls, so nothing beyond
// getProfileByField(address,string) is included. The full contract ABI lives
// off-chain; expanding this subset requires a matching audit of the caller.
const abiJSON = `[
	{
		"constant": true,
		"inputs": [
			{ "name": "_delegate", "type": "address" },
			{ "name": "_field", "type": "string" }
		],
		"name": "getProfileByField",
		"outputs": [
			{ "name": "", "type": "bytes" }
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}
]`
