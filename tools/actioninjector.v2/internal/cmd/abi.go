package cmd

/*
pragma solidity ^0.8.0;
// SPDX-License-Identifier: MIT

	contract GasConsumer {
	    uint[] public results;

	    function consumeGas(uint size) public {
	        uint[] memory array = new uint[](size);

	        for(uint i = 0; i < size; i++) {
	            array[i] = i * 2;
	            results.push(array[i]);
	        }
	    }
	}

//5 10W gas
*/
var _abiStr = `
[
	{
		"inputs": [
			{
				"internalType": "uint256",
				"name": "iterations",
				"type": "uint256"
			}
		],
		"name": "consumeGas",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "count",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]
`
