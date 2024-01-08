// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract BasicToken {
    mapping(address => uint256) balances;

    function transfer(address recipient, uint256 value) public {
        if (balances[msg.sender] >= value) {
            balances[msg.sender] -= value;
        }
        balances[recipient] += value;
    }

    function balanceOf(address account) public view returns (uint256) {
        return balances[account];
    }
}
