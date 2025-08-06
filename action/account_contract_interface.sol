// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.24;

// Interface for the Account Protocol contract
interface IAccountProtocolContract {
    // enum representing different types of transfer events
    enum TransactionLogType {
        NATIVE_TRANSFER,
        IN_CONTRACT_TRANSFER,
        GAS_FEE,
        DEPOSIT_TO_REWARDING_FUND,
        CLAIM_FROM_REWARDING_FUND,
        PRIORITY_FEE,
        BLOB_FEE
    }

    // Event emitted when a transfer occurs
    event Transfer(
        address indexed from,
        address indexed to,
        TransactionLogType indexed logType,
        uint256 amount
    );
}
