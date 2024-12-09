// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

contract SelfDestructExample {
    address payable public owner;

    constructor() {
        owner = payable(msg.sender); // Set contract deployer as owner
    }

    // Function to self-destruct the contract
    function terminate() external {
        require(msg.sender == owner, "Only the owner can destroy the contract");
        selfdestruct(owner); // Send remaining funds to the owner and destroy the contract
    }

    // Fallback function to receive Ether
    receive() external payable {}

    // Function to check balance of the contract
    function getBalance() external view returns (uint256) {
        return address(this).balance;
    }
}
