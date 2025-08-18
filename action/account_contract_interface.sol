// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.24;

// Interface for the Account Protocol contract
interface IAccountProtocolContract {
    // enum representing different types of transfer events
    enum TransactionLogType {
        IN_CONTRACT_TRANSFER,
        WITHDRAW_BUCKET,
        CREATE_BUCKET,
        DEPOSIT_TO_BUCKET,
        CANDIDATE_SELF_STAKE,
        CANDIDATE_REGISTRATION_FEE,
        GAS_FEE,
        NATIVE_TRANSFER,
        DEPOSIT_TO_REWARDING_FUND,
        CLAIM_FROM_REWARDING_FUND,
        BLOB_FEE,
        PRIORITY_FEE
    }

    // Event emitted when a transfer occurs
    event Transfer(
        address indexed from,
        address indexed to,
        TransactionLogType indexed logType,
        uint256 amount
    );
}
