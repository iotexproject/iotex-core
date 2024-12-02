// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

// this is the contract for "_selfdestructOnCreationContract" under blockchain/integrity

contract SelfDestructOnCreation {
    address payable public owner;

    constructor() payable {
        owner = payable(msg.sender); // Set contract deployer as owner
        selfdestruct(owner); // Self-destruct the contract immediately, sending any funds to the recipient
    }

    // Function to check balance of the contract
    function getBalance() external view returns (uint256) {
        return address(this).balance;
    }
}
