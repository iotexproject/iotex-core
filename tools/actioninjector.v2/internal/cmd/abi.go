package cmd

var _abiStr = `
[
	{
		"constant": true,
		"inputs": [
			{
				"name": "_member",
				"type": "address"
			},
			{
				"name": "_timestamp",
				"type": "uint64"
			}
		],
		"name": "hash",
		"outputs": [
			{
				"name": "",
				"type": "string"
			}
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": false,
		"inputs": [
			{
				"name": "_timestamp",
				"type": "uint64"
			},
			{
				"name": "_hash",
				"type": "string"
			}
		],
		"name": "addHash",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}
]
`
