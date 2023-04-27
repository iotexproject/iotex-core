// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"encoding/binary"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockindex/indexpb"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// TODO (iip-13): replace with the real liquid staking contract address
	LiquidStakingContractAddress = "io1dkqh5mu9djfas3xyrmzdv9frsmmytel4mp7a64"
	// LiquidStakingContractABI is the ABI of liquid staking contract
	LiquidStakingContractABI = `[
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_amount",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_duration",
					"type": "uint256"
				}
			],
			"name": "activateBucketType",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_amount",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_duration",
					"type": "uint256"
				}
			],
			"name": "addBucketType",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [],
			"stateMutability": "nonpayable",
			"type": "constructor"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				}
			],
			"name": "AmountIncreased",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "address",
					"name": "owner",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "address",
					"name": "approved",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "Approval",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "address",
					"name": "owner",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "address",
					"name": "operator",
					"type": "address"
				},
				{
					"indexed": false,
					"internalType": "bool",
					"name": "approved",
					"type": "bool"
				}
			],
			"name": "ApprovalForAll",
			"type": "event"
		},
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "to",
					"type": "address"
				},
				{
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "approve",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "duration",
					"type": "uint256"
				}
			],
			"name": "BucketTypeActivated",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "duration",
					"type": "uint256"
				}
			],
			"name": "BucketTypeDeactivated",
			"type": "event"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_tokenId",
					"type": "uint256"
				},
				{
					"internalType": "bytes12",
					"name": "_delegate",
					"type": "bytes12"
				}
			],
			"name": "changeDelegate",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256[]",
					"name": "_tokenIds",
					"type": "uint256[]"
				},
				{
					"internalType": "bytes12",
					"name": "_delegate",
					"type": "bytes12"
				}
			],
			"name": "changeDelegates",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_amount",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_duration",
					"type": "uint256"
				}
			],
			"name": "deactivateBucketType",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "bytes12",
					"name": "newDelegate",
					"type": "bytes12"
				}
			],
			"name": "DelegateChanged",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "duration",
					"type": "uint256"
				}
			],
			"name": "DurationExtended",
			"type": "event"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_tokenId",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_newDuration",
					"type": "uint256"
				}
			],
			"name": "extendDuration",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_tokenId",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_newAmount",
					"type": "uint256"
				}
			],
			"name": "increaseAmount",
			"outputs": [],
			"stateMutability": "payable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_tokenId",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_duration",
					"type": "uint256"
				}
			],
			"name": "lock",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256[]",
					"name": "_tokenIds",
					"type": "uint256[]"
				},
				{
					"internalType": "uint256",
					"name": "_duration",
					"type": "uint256"
				}
			],
			"name": "lock",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "duration",
					"type": "uint256"
				}
			],
			"name": "Locked",
			"type": "event"
		},
		{
			"inputs": [
				{
					"internalType": "uint256[]",
					"name": "tokenIds",
					"type": "uint256[]"
				},
				{
					"internalType": "uint256",
					"name": "_newDuration",
					"type": "uint256"
				}
			],
			"name": "merge",
			"outputs": [],
			"stateMutability": "payable",
			"type": "function"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256[]",
					"name": "tokenIds",
					"type": "uint256[]"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "duration",
					"type": "uint256"
				}
			],
			"name": "Merged",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "address",
					"name": "previousOwner",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "address",
					"name": "newOwner",
					"type": "address"
				}
			],
			"name": "OwnershipTransferred",
			"type": "event"
		},
		{
			"inputs": [],
			"name": "pause",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": false,
					"internalType": "address",
					"name": "account",
					"type": "address"
				}
			],
			"name": "Paused",
			"type": "event"
		},
		{
			"inputs": [],
			"name": "renounceOwnership",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "from",
					"type": "address"
				},
				{
					"internalType": "address",
					"name": "to",
					"type": "address"
				},
				{
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "safeTransferFrom",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "from",
					"type": "address"
				},
				{
					"internalType": "address",
					"name": "to",
					"type": "address"
				},
				{
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				},
				{
					"internalType": "bytes",
					"name": "data",
					"type": "bytes"
				}
			],
			"name": "safeTransferFrom",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "operator",
					"type": "address"
				},
				{
					"internalType": "bool",
					"name": "approved",
					"type": "bool"
				}
			],
			"name": "setApprovalForAll",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_duration",
					"type": "uint256"
				},
				{
					"internalType": "bytes12",
					"name": "_delegate",
					"type": "bytes12"
				}
			],
			"name": "stake",
			"outputs": [
				{
					"internalType": "uint256",
					"name": "",
					"type": "uint256"
				}
			],
			"stateMutability": "payable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_amount",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_duration",
					"type": "uint256"
				},
				{
					"internalType": "bytes12",
					"name": "_delegate",
					"type": "bytes12"
				},
				{
					"internalType": "uint256",
					"name": "_count",
					"type": "uint256"
				}
			],
			"name": "stake",
			"outputs": [
				{
					"internalType": "uint256",
					"name": "firstTokenId_",
					"type": "uint256"
				}
			],
			"stateMutability": "payable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_amount",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_duration",
					"type": "uint256"
				},
				{
					"internalType": "bytes12[]",
					"name": "_delegates",
					"type": "bytes12[]"
				}
			],
			"name": "stake",
			"outputs": [
				{
					"internalType": "uint256",
					"name": "firstTokenId_",
					"type": "uint256"
				}
			],
			"stateMutability": "payable",
			"type": "function"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "bytes12",
					"name": "delegate",
					"type": "bytes12"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "amount",
					"type": "uint256"
				},
				{
					"indexed": false,
					"internalType": "uint256",
					"name": "duration",
					"type": "uint256"
				}
			],
			"name": "Staked",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "address",
					"name": "from",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "address",
					"name": "to",
					"type": "address"
				},
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "Transfer",
			"type": "event"
		},
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "from",
					"type": "address"
				},
				{
					"internalType": "address",
					"name": "to",
					"type": "address"
				},
				{
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "transferFrom",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "newOwner",
					"type": "address"
				}
			],
			"name": "transferOwnership",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256[]",
					"name": "_tokenIds",
					"type": "uint256[]"
				}
			],
			"name": "unlock",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_tokenId",
					"type": "uint256"
				}
			],
			"name": "unlock",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "Unlocked",
			"type": "event"
		},
		{
			"inputs": [],
			"name": "unpause",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": false,
					"internalType": "address",
					"name": "account",
					"type": "address"
				}
			],
			"name": "Unpaused",
			"type": "event"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_tokenId",
					"type": "uint256"
				}
			],
			"name": "unstake",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256[]",
					"name": "_tokenIds",
					"type": "uint256[]"
				}
			],
			"name": "unstake",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "Unstaked",
			"type": "event"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_tokenId",
					"type": "uint256"
				},
				{
					"internalType": "address payable",
					"name": "_recipient",
					"type": "address"
				}
			],
			"name": "withdraw",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256[]",
					"name": "_tokenIds",
					"type": "uint256[]"
				},
				{
					"internalType": "address payable",
					"name": "_recipient",
					"type": "address"
				}
			],
			"name": "withdraw",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": true,
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				},
				{
					"indexed": true,
					"internalType": "address",
					"name": "recipient",
					"type": "address"
				}
			],
			"name": "Withdrawal",
			"type": "event"
		},
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "owner",
					"type": "address"
				}
			],
			"name": "balanceOf",
			"outputs": [
				{
					"internalType": "uint256",
					"name": "",
					"type": "uint256"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_tokenId",
					"type": "uint256"
				}
			],
			"name": "blocksToUnstake",
			"outputs": [
				{
					"internalType": "uint256",
					"name": "",
					"type": "uint256"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_tokenId",
					"type": "uint256"
				}
			],
			"name": "blocksToWithdraw",
			"outputs": [
				{
					"internalType": "uint256",
					"name": "",
					"type": "uint256"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_tokenId",
					"type": "uint256"
				}
			],
			"name": "bucketOf",
			"outputs": [
				{
					"internalType": "uint256",
					"name": "amount_",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "duration_",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "unlockedAt_",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "unstakedAt_",
					"type": "uint256"
				},
				{
					"internalType": "bytes12",
					"name": "delegate_",
					"type": "bytes12"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_offset",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_size",
					"type": "uint256"
				}
			],
			"name": "bucketTypes",
			"outputs": [
				{
					"components": [
						{
							"internalType": "uint256",
							"name": "amount",
							"type": "uint256"
						},
						{
							"internalType": "uint256",
							"name": "duration",
							"type": "uint256"
						},
						{
							"internalType": "uint256",
							"name": "activatedAt",
							"type": "uint256"
						}
					],
					"internalType": "struct BucketType[]",
					"name": "types_",
					"type": "tuple[]"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "getApproved",
			"outputs": [
				{
					"internalType": "address",
					"name": "",
					"type": "address"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_amount",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_duration",
					"type": "uint256"
				}
			],
			"name": "isActiveBucketType",
			"outputs": [
				{
					"internalType": "bool",
					"name": "",
					"type": "bool"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "owner",
					"type": "address"
				},
				{
					"internalType": "address",
					"name": "operator",
					"type": "address"
				}
			],
			"name": "isApprovedForAll",
			"outputs": [
				{
					"internalType": "bool",
					"name": "",
					"type": "bool"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "bytes12[]",
					"name": "_delegates",
					"type": "bytes12[]"
				}
			],
			"name": "lockedVotesTo",
			"outputs": [
				{
					"internalType": "uint256[][]",
					"name": "counts_",
					"type": "uint256[][]"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [],
			"name": "name",
			"outputs": [
				{
					"internalType": "string",
					"name": "",
					"type": "string"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [],
			"name": "numOfBucketTypes",
			"outputs": [
				{
					"internalType": "uint256",
					"name": "",
					"type": "uint256"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [],
			"name": "owner",
			"outputs": [
				{
					"internalType": "address",
					"name": "",
					"type": "address"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "ownerOf",
			"outputs": [
				{
					"internalType": "address",
					"name": "",
					"type": "address"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [],
			"name": "paused",
			"outputs": [
				{
					"internalType": "bool",
					"name": "",
					"type": "bool"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "bytes4",
					"name": "interfaceId",
					"type": "bytes4"
				}
			],
			"name": "supportsInterface",
			"outputs": [
				{
					"internalType": "bool",
					"name": "",
					"type": "bool"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [],
			"name": "symbol",
			"outputs": [
				{
					"internalType": "string",
					"name": "",
					"type": "string"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "tokenId",
					"type": "uint256"
				}
			],
			"name": "tokenURI",
			"outputs": [
				{
					"internalType": "string",
					"name": "",
					"type": "string"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [],
			"name": "UINT256_MAX",
			"outputs": [
				{
					"internalType": "uint256",
					"name": "",
					"type": "uint256"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "bytes12[]",
					"name": "_delegates",
					"type": "bytes12[]"
				}
			],
			"name": "unlockedVotesTo",
			"outputs": [
				{
					"internalType": "uint256[][]",
					"name": "counts_",
					"type": "uint256[][]"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [],
			"name": "UNSTAKE_FREEZE_BLOCKS",
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
	]`

	// bucket related namespace in db
	_liquidStakingBucketInfoNS = "lsbInfo"
	_liquidStakingBucketTypeNS = "lsbType"
	_liquidStakingHeightNS     = "lsHeight"
)

type (
	// LiquidStakingIndexer is the interface of liquid staking indexer
	LiquidStakingIndexer interface {
		blockdao.BlockIndexer

		GetCandidateVotes(candidate string) *big.Int
		GetBuckets() ([]*Bucket, error)
		GetBucket(id uint64) (*Bucket, error)
	}

	liquidStakingIndexer struct {
		dirty      batch.CachedBatch   // batch for dirty data
		kvstore    db.KVStore          // persistent storage
		dirtyCache *liquidStakingCache // in-memory index for dirty data
		cleanCache *liquidStakingCache // in-memory index for clean data

		tokenOwner    map[uint64]string // token id -> owner
		blockInterval time.Duration
	}

	// BucketInfo is the bucket information
	BucketInfo struct {
		TypeIndex  uint64
		CreatedAt  time.Time
		UnlockedAt *time.Time
		UnstakedAt *time.Time
		Delegate   string
		Owner      string
	}

	// BucketType is the bucket type
	BucketType struct {
		Amount      *big.Int
		Duration    time.Duration
		ActivatedAt *time.Time
	}

	// Bucket is the bucket information including bucket type and bucket info
	Bucket struct {
		Index            uint64
		Candidate        string
		Owner            address.Address
		StakedAmount     *big.Int
		StakedDuration   time.Duration
		CreateTime       time.Time
		StakeStartTime   time.Time
		UnstakeStartTime time.Time
		AutoStake        bool
	}

	// eventParam is a struct to hold smart contract event parameters, which can easily convert a param to go type
	// TODO: this is general enough to be moved to a common package
	eventParam map[string]any

	liquidStakingCache struct {
		idBucketMap           map[uint64]*BucketInfo     // map[token]BucketInfo
		candidateBucketMap    map[string]map[uint64]bool // map[candidate]bucket
		idBucketTypeMap       map[uint64]*BucketType     // map[token]BucketType
		propertyBucketTypeMap map[int64]map[int64]uint64 // map[amount][duration]index
		height                uint64
	}
)

var (
	_liquidStakingInterface abi.ABI
	_liquidStakingHeightKey = []byte("lsHeight")

	errInvlidEventParam   = errors.New("invalid event param")
	errBucketTypeNotExist = errors.New("bucket type does not exist")

	// ErrBucketInfoNotExist is the error when bucket does not exist
	ErrBucketInfoNotExist = errors.New("bucket info does not exist")
)

func init() {
	var err error
	_liquidStakingInterface, err = abi.JSON(strings.NewReader(LiquidStakingContractABI))
	if err != nil {
		panic(err)
	}
}

// NewLiquidStakingIndexer creates a new liquid staking indexer
func NewLiquidStakingIndexer(kvStore db.KVStore, blockInterval time.Duration) LiquidStakingIndexer {
	return &liquidStakingIndexer{
		blockInterval: blockInterval,
		dirty:         batch.NewCachedBatch(),
		dirtyCache:    newLiquidStakingCache(),
		kvstore:       kvStore,
		cleanCache:    newLiquidStakingCache(),
		tokenOwner:    make(map[uint64]string),
	}
}

func (s *liquidStakingIndexer) Start(ctx context.Context) error {
	if err := s.kvstore.Start(ctx); err != nil {
		return err
	}
	return s.loadCache()
}

func (s *liquidStakingIndexer) Stop(ctx context.Context) error {
	if err := s.kvstore.Stop(ctx); err != nil {
		return err
	}
	return nil
}

func (s *liquidStakingIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	actionMap := make(map[hash.Hash256]*action.SealedEnvelope)
	for _, act := range blk.Actions {
		h, err := act.Hash()
		if err != nil {
			return err
		}
		actionMap[h] = &act
	}

	s.dirtyCache.putHeight(blk.Height())
	for _, receipt := range blk.Receipts {
		if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		act, ok := actionMap[receipt.ActionHash]
		if !ok {
			return errors.Errorf("action %x not found", receipt.ActionHash)
		}
		for _, log := range receipt.Logs() {
			if log.Address != LiquidStakingContractAddress {
				continue
			}
			if err := s.handleEvent(ctx, blk, act, log); err != nil {
				return err
			}
		}
	}
	return s.commit()
}

func (s *liquidStakingIndexer) DeleteTipBlock(context.Context, *block.Block) error {
	return errors.New("not implemented")
}

func (s *liquidStakingIndexer) Height() (uint64, error) {
	return s.cleanCache.getHeight(), nil
}

func (s *liquidStakingIndexer) GetCandidateVotes(candidate string) *big.Int {
	return s.cleanCache.getCandidateVotes(candidate)
}

func (s *liquidStakingIndexer) GetBuckets() ([]*Bucket, error) {
	vbs := []*Bucket{}
	for id, bi := range s.cleanCache.idBucketMap {
		bt := s.cleanCache.mustGetBucketType(bi.TypeIndex)
		vb, err := convertToVoteBucket(id, bi, bt)
		if err != nil {
			return nil, err
		}
		vbs = append(vbs, vb)
	}
	return vbs, nil
}

func (s *liquidStakingIndexer) GetBucket(id uint64) (*Bucket, error) {
	bi, ok := s.cleanCache.idBucketMap[id]
	if !ok {
		return nil, errors.Wrapf(ErrBucketInfoNotExist, "id %d", id)
	}
	bt := s.cleanCache.mustGetBucketType(bi.TypeIndex)
	vb, err := convertToVoteBucket(id, bi, bt)
	if err != nil {
		return nil, err
	}
	return vb, nil
}

func (s *liquidStakingIndexer) handleEvent(ctx context.Context, blk *block.Block, act *action.SealedEnvelope, log *action.Log) error {
	// get event abi
	abiEvent, err := _liquidStakingInterface.EventByID(common.Hash(log.Topics[0]))
	if err != nil {
		return errors.Wrapf(err, "get event abi from topic %v failed", log.Topics[0])
	}

	// unpack event data
	event, err := unpackEventParam(abiEvent, log)
	if err != nil {
		return err
	}

	// handle different kinds of event
	timestamp := blk.Timestamp()
	switch abiEvent.Name {
	case "BucketTypeActivated":
		err = s.handleBucketTypeActivatedEvent(event, timestamp)
	case "BucketTypeDeactivated":
		err = s.handleBucketTypeDeactivatedEvent(event)
	case "Staked":
		err = s.handleStakedEvent(event, timestamp)
	case "Locked":
		err = s.handleLockedEvent(event)
	case "Unlocked":
		err = s.handleUnlockedEvent(event, timestamp)
	case "Unstaked":
		err = s.handleUnstakedEvent(event, timestamp)
	case "Merged":
		err = s.handleMergedEvent(event)
	case "DurationExtended":
		err = s.handleDurationExtendedEvent(event)
	case "AmountIncreased":
		err = s.handleAmountIncreasedEvent(event)
	case "DelegateChanged":
		err = s.handleDelegateChangedEvent(event)
	case "Withdrawal":
		err = s.handleWithdrawalEvent(event)
	case "Transfer":
		err = s.handleTransferEvent(event)
	default:
		err = nil
	}
	return err
}

func (s *liquidStakingIndexer) handleTransferEvent(event eventParam) error {
	to, err := event.indexedFieldAddress("to")
	if err != nil {
		return err
	}
	tokenID, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	s.tokenOwner[tokenID.Uint64()] = to.String()
	return nil
}

func (s *liquidStakingIndexer) handleBucketTypeActivatedEvent(event eventParam, timeStamp time.Time) error {
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	bt := BucketType{
		Amount:      amountParam,
		Duration:    s.blockHeightToDuration(durationParam.Uint64()),
		ActivatedAt: &timeStamp,
	}
	id, ok := s.getBucketTypeIndex(amountParam, bt.Duration)
	if !ok {
		id = s.getBucketTypeCount()
	}
	s.putBucketType(id, &bt)
	return nil
}

func (s *liquidStakingIndexer) handleBucketTypeDeactivatedEvent(event eventParam) error {
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	id, ok := s.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	bt, ok := s.getBucketType(id)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", id)
	}
	bt.ActivatedAt = nil
	s.putBucketType(id, bt)
	return nil
}

func (s *liquidStakingIndexer) handleStakedEvent(event eventParam, timestamp time.Time) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	delegateParam, err := event.fieldBytes12("delegate")
	if err != nil {
		return err
	}
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	btIdx, ok := s.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}

	bucket := BucketInfo{
		TypeIndex: btIdx,
		Delegate:  delegateParam,
		Owner:     s.tokenOwner[tokenIDParam.Uint64()],
		CreatedAt: timestamp,
	}
	s.putBucketInfo(tokenIDParam.Uint64(), &bucket)
	return nil
}

func (s *liquidStakingIndexer) handleLockedEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	b, ok := s.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := s.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := s.getBucketTypeIndex(bt.Amount, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %v, duration %d", bt.Amount, durationParam.Uint64())
	}
	b.TypeIndex = newBtIdx
	b.UnlockedAt = nil
	s.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleUnlockedEvent(event eventParam, timestamp time.Time) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	b, ok := s.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.UnlockedAt = &timestamp
	s.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleUnstakedEvent(event eventParam, timestamp time.Time) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	b, ok := s.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.UnstakedAt = &timestamp
	s.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleMergedEvent(event eventParam) error {
	tokenIDsParam, err := event.fieldUint256Slice("tokenIds")
	if err != nil {
		return err
	}
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	// merge to the first bucket
	btIdx, ok := s.getBucketTypeIndex(amountParam, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	b, ok := s.getBucketInfo(tokenIDsParam[0].Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDsParam[0].Uint64())
	}
	b.TypeIndex = btIdx
	b.UnlockedAt = nil
	for i := 1; i < len(tokenIDsParam); i++ {
		s.burnBucket(tokenIDsParam[i].Uint64())
	}
	s.putBucketInfo(tokenIDsParam[0].Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleDurationExtendedEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	b, ok := s.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := s.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := s.getBucketTypeIndex(bt.Amount, s.blockHeightToDuration(durationParam.Uint64()))
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", bt.Amount.Int64(), durationParam.Uint64())
	}
	b.TypeIndex = newBtIdx
	s.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleAmountIncreasedEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}

	b, ok := s.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := s.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := s.getBucketTypeIndex(amountParam, bt.Duration)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), bt.Duration)
	}
	b.TypeIndex = newBtIdx
	s.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleDelegateChangedEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	delegateParam, err := event.fieldBytes12("newDelegate")
	if err != nil {
		return err
	}

	b, ok := s.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.Delegate = string(delegateParam[:])
	s.putBucketInfo(tokenIDParam.Uint64(), b)
	return nil
}

func (s *liquidStakingIndexer) handleWithdrawalEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	s.burnBucket(tokenIDParam.Uint64())
	return nil
}

func (s *liquidStakingIndexer) loadCache() error {
	// load height
	var height uint64
	h, err := s.kvstore.Get(_liquidStakingHeightNS, _liquidStakingHeightKey)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
		height = 0
	} else {
		height = deserializeUint64(h)
	}
	s.cleanCache.putHeight(height)

	// load bucket info
	ks, vs, err := s.kvstore.Filter(_liquidStakingBucketInfoNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil {
		if !errors.Is(err, db.ErrBucketNotExist) {
			return err
		}
	}
	for i := range vs {
		var b BucketInfo
		if err := b.deserialize(vs[i]); err != nil {
			return err
		}
		s.cleanCache.putBucketInfo(deserializeUint64(ks[i]), &b)
	}

	// load bucket type
	ks, vs, err = s.kvstore.Filter(_liquidStakingBucketTypeNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil {
		if !errors.Is(err, db.ErrBucketNotExist) {
			return err
		}
	}
	for i := range vs {
		var b BucketType
		if err := b.deserialize(vs[i]); err != nil {
			return err
		}
		s.cleanCache.putBucketType(deserializeUint64(ks[i]), &b)
	}
	return nil
}

func (s *liquidStakingIndexer) getBucketTypeIndex(amount *big.Int, duration time.Duration) (uint64, bool) {
	id, ok := s.dirtyCache.getBucketTypeIndex(amount, duration)
	if ok {
		return id, true
	}
	id, ok = s.cleanCache.getBucketTypeIndex(amount, duration)
	return id, ok
}

func (s *liquidStakingIndexer) getBucketTypeCount() uint64 {
	base := len(s.cleanCache.idBucketTypeMap)
	add := 0
	for k, dbt := range s.dirtyCache.idBucketTypeMap {
		_, ok := s.cleanCache.idBucketTypeMap[k]
		if dbt != nil && !ok {
			add++
		} else if dbt == nil && ok {
			add--
		}
	}
	return uint64(base + add)
}

func (s *liquidStakingIndexer) getBucketType(id uint64) (*BucketType, bool) {
	bt, ok := s.dirtyCache.getBucketType(id)
	if ok {
		return bt, true
	}
	bt, ok = s.cleanCache.getBucketType(id)
	return bt, ok
}

func (s *liquidStakingIndexer) putHeight(h uint64) {
	s.dirty.Put(_liquidStakingHeightNS, _liquidStakingHeightKey, serializeUint64(h), "failed to put height")
	s.dirtyCache.putHeight(h)
}

func (s *liquidStakingIndexer) putBucketType(id uint64, bt *BucketType) {
	s.dirty.Put(_liquidStakingBucketTypeNS, serializeUint64(id), bt.serialize(), "failed to put bucket type")
	s.dirtyCache.putBucketType(id, bt)
}

func (s *liquidStakingIndexer) putBucketInfo(id uint64, bi *BucketInfo) {
	s.dirty.Put(_liquidStakingBucketInfoNS, serializeUint64(id), bi.serialize(), "failed to put bucket info")
	s.dirtyCache.putBucketInfo(id, bi)
}

func (s *liquidStakingIndexer) getBucketInfo(id uint64) (*BucketInfo, bool) {
	bi, ok := s.dirtyCache.getBucketInfo(id)
	if ok {
		return bi, bi != nil
	}
	bi, ok = s.cleanCache.getBucketInfo(id)
	return bi, ok
}

func (s *liquidStakingIndexer) burnBucket(id uint64) {
	s.dirty.Delete(_liquidStakingBucketInfoNS, serializeUint64(id), "failed to delete bucket info")
	s.dirtyCache.deleteBucketInfo(id)
}

func (s *liquidStakingIndexer) commit() error {
	if err := s.cleanCache.writeBatch(s.dirty); err != nil {
		return err
	}
	if err := s.kvstore.WriteBatch(s.dirty); err != nil {
		return err
	}
	s.dirty.Lock()
	s.dirty.ClearAndUnlock()
	s.dirtyCache = newLiquidStakingCache()
	return nil
}

func (s *liquidStakingIndexer) blockHeightToDuration(height uint64) time.Duration {
	return time.Duration(height) * s.blockInterval
}

func eventField[T any](e eventParam, name string) (T, error) {
	field, ok := e[name].(T)
	if !ok {
		return field, errors.Wrapf(errInvlidEventParam, "field %s got %#v, expect %T", name, e[name], field)
	}
	return field, nil
}

func (e eventParam) fieldUint256(name string) (*big.Int, error) {
	return eventField[*big.Int](e, name)
}

func (e eventParam) fieldBytes12(name string) (string, error) {
	data, err := eventField[[12]byte](e, name)
	if err != nil {
		return "", err
	}
	// remove trailing zeros
	tail := len(data) - 1
	for ; tail >= 0 && data[tail] == 0; tail-- {
	}
	return string(data[:tail+1]), nil
}

func (e eventParam) fieldUint256Slice(name string) ([]*big.Int, error) {
	return eventField[[]*big.Int](e, name)
}

func (e eventParam) fieldAddress(name string) (common.Address, error) {
	return eventField[common.Address](e, name)
}

func (e eventParam) indexedFieldAddress(name string) (common.Address, error) {
	return eventField[common.Address](e, name)
}

func (e eventParam) indexedFieldUint256(name string) (*big.Int, error) {
	return eventField[*big.Int](e, name)
}

func (bt *BucketType) toProto() *indexpb.BucketType {
	return &indexpb.BucketType{
		Amount:      bt.Amount.String(),
		Duration:    uint64(bt.Duration),
		ActivatedAt: timestamppb.New(*bt.ActivatedAt),
	}
}

func (bt *BucketType) loadProto(p *indexpb.BucketType) error {
	var ok bool
	bt.Amount, ok = big.NewInt(0).SetString(p.Amount, 10)
	if !ok {
		return errors.New("failed to parse amount")
	}
	bt.Duration = time.Duration(p.Duration)
	t := p.ActivatedAt.AsTime()
	bt.ActivatedAt = &t
	return nil
}

func (bt *BucketType) serialize() []byte {
	return byteutil.Must(proto.Marshal(bt.toProto()))
}

func (bt *BucketType) deserialize(b []byte) error {
	m := indexpb.BucketType{}
	if err := proto.Unmarshal(b, &m); err != nil {
		return err
	}
	return bt.loadProto(&m)
}

func (bi *BucketInfo) toProto() *indexpb.BucketInfo {
	pb := &indexpb.BucketInfo{
		TypeIndex: bi.TypeIndex,
		Delegate:  bi.Delegate,
		CreatedAt: timestamppb.New(bi.CreatedAt),
		Owner:     bi.Owner,
	}
	if bi.UnlockedAt != nil {
		pb.UnlockedAt = timestamppb.New(*bi.UnlockedAt)
	}
	if bi.UnstakedAt != nil {
		pb.UnstakedAt = timestamppb.New(*bi.UnstakedAt)
	}
	return pb
}

func (bi *BucketInfo) serialize() []byte {
	return byteutil.Must(proto.Marshal(bi.toProto()))
}

func (bi *BucketInfo) deserialize(b []byte) error {
	m := indexpb.BucketInfo{}
	if err := proto.Unmarshal(b, &m); err != nil {
		return err
	}
	return bi.loadProto(&m)
}

func (bi *BucketInfo) loadProto(p *indexpb.BucketInfo) error {
	bi.TypeIndex = p.TypeIndex
	bi.CreatedAt = p.CreatedAt.AsTime()
	if p.UnlockedAt != nil {
		t := p.UnlockedAt.AsTime()
		bi.UnlockedAt = &t
	} else {
		bi.UnlockedAt = nil
	}
	if p.UnstakedAt != nil {
		t := p.UnstakedAt.AsTime()
		bi.UnstakedAt = &t
	} else {
		bi.UnstakedAt = nil
	}
	bi.Delegate = p.Delegate
	bi.Owner = p.Owner
	return nil
}

func newLiquidStakingCache() *liquidStakingCache {
	return &liquidStakingCache{
		idBucketMap:           make(map[uint64]*BucketInfo),
		idBucketTypeMap:       make(map[uint64]*BucketType),
		propertyBucketTypeMap: make(map[int64]map[int64]uint64),
		candidateBucketMap:    make(map[string]map[uint64]bool),
	}
}

func (s *liquidStakingCache) writeBatch(b batch.KVStoreBatch) error {
	for i := 0; i < b.Size(); i++ {
		write, err := b.Entry(i)
		if err != nil {
			return err
		}
		switch write.Namespace() {
		case _liquidStakingBucketInfoNS:
			if write.WriteType() == batch.Put {
				var bi BucketInfo
				if err = bi.deserialize(write.Value()); err != nil {
					return err
				}
				id := deserializeUint64(write.Key())
				s.putBucketInfo(id, &bi)
			} else if write.WriteType() == batch.Delete {
				id := deserializeUint64(write.Key())
				s.deleteBucketInfo(id)
			}
		case _liquidStakingBucketTypeNS:
			if write.WriteType() == batch.Put {
				var bt BucketType
				if err = bt.deserialize(write.Value()); err != nil {
					return err
				}
				id := deserializeUint64(write.Key())
				s.putBucketType(id, &bt)
			}
		}
	}
	return nil
}

func (s *liquidStakingCache) putHeight(h uint64) {
	s.height = h
}

func (s *liquidStakingCache) getHeight() uint64 {
	return s.height
}

func (s *liquidStakingCache) putBucketType(id uint64, bt *BucketType) {
	amount := bt.Amount.Int64()
	s.idBucketTypeMap[id] = bt
	m, ok := s.propertyBucketTypeMap[amount]
	if !ok {
		s.propertyBucketTypeMap[amount] = make(map[int64]uint64)
		m = s.propertyBucketTypeMap[amount]
	}
	m[int64(bt.Duration)] = id
}

func (s *liquidStakingCache) putBucketInfo(id uint64, bi *BucketInfo) {
	s.idBucketMap[id] = bi
	if _, ok := s.candidateBucketMap[bi.Delegate]; !ok {
		s.candidateBucketMap[bi.Delegate] = make(map[uint64]bool)
	}
	s.candidateBucketMap[bi.Delegate][id] = true
}

func (s *liquidStakingCache) deleteBucketInfo(id uint64) {
	bi, ok := s.idBucketMap[id]
	if !ok {
		return
	}
	s.idBucketMap[id] = nil
	if _, ok := s.candidateBucketMap[bi.Delegate]; !ok {
		return
	}
	s.candidateBucketMap[bi.Delegate][id] = false
}

func (s *liquidStakingCache) getBucketTypeIndex(amount *big.Int, duration time.Duration) (uint64, bool) {
	m, ok := s.propertyBucketTypeMap[amount.Int64()]
	if !ok {
		return 0, false
	}
	id, ok := m[int64(duration)]
	return id, ok
}

func (s *liquidStakingCache) getBucketType(id uint64) (*BucketType, bool) {
	bt, ok := s.idBucketTypeMap[id]
	return bt, ok
}

func (s *liquidStakingCache) mustGetBucketType(id uint64) *BucketType {
	bt, ok := s.idBucketTypeMap[id]
	if !ok {
		panic("bucket type not found")
	}
	return bt
}

func (s *liquidStakingCache) getBucketInfo(id uint64) (*BucketInfo, bool) {
	bi, ok := s.idBucketMap[id]
	return bi, ok
}

func (s *liquidStakingCache) getCandidateVotes(name string) *big.Int {
	votes := big.NewInt(0)
	m, ok := s.candidateBucketMap[name]
	if !ok {
		return votes
	}
	for k, v := range m {
		if v {
			bi, ok := s.idBucketMap[k]
			if !ok {
				continue
			}
			if bi.UnstakedAt != nil {
				continue
			}
			bt := s.mustGetBucketType(bi.TypeIndex)
			votes.Add(votes, bt.Amount)
		}
	}
	return votes
}

func serializeUint64(v uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return b
}

func deserializeUint64(b []byte) uint64 {
	return binary.LittleEndian.Uint64(b)
}

func convertToVoteBucket(token uint64, bi *BucketInfo, bt *BucketType) (*Bucket, error) {
	var err error
	vb := Bucket{
		Index:            token,
		StakedAmount:     bt.Amount,
		StakedDuration:   bt.Duration,
		CreateTime:       bi.CreatedAt,
		StakeStartTime:   bi.CreatedAt,
		UnstakeStartTime: time.Unix(0, 0).UTC(),
		AutoStake:        bi.UnlockedAt == nil,
		Candidate:        bi.Delegate,
	}

	vb.Owner, err = address.FromHex(bi.Owner)
	if err != nil {
		return nil, err
	}
	if bi.UnlockedAt != nil {
		vb.StakeStartTime = *bi.UnlockedAt
	}
	if bi.UnstakedAt != nil {
		vb.UnstakeStartTime = *bi.UnstakedAt
	}
	return &vb, nil
}

func unpackEventParam(abiEvent *abi.Event, log *action.Log) (eventParam, error) {
	event := make(eventParam)
	// unpack non-indexed fields
	if len(log.Data) > 0 {
		if err := abiEvent.Inputs.UnpackIntoMap(event, log.Data); err != nil {
			return nil, errors.Wrap(err, "unpack event data failed")
		}
	}
	// unpack indexed fields
	args := make(abi.Arguments, 0)
	for _, arg := range abiEvent.Inputs {
		if arg.Indexed {
			args = append(args, arg)
		}
	}
	topics := make([]common.Hash, 0)
	for i, topic := range log.Topics {
		if i > 0 {
			topics = append(topics, common.Hash(topic))
		}
	}
	err := abi.ParseTopicsIntoMap(event, args, topics)
	if err != nil {
		return nil, errors.Wrap(err, "unpack event indexed fields failed")
	}
	return event, nil
}
