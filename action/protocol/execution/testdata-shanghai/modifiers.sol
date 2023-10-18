// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract MyContract {
    bool locked = false;

    modifier validAddress(address account) {
        if (account == address(0x0)) {
            revert();
        }
        _;
    }

    modifier greaterThan(uint256 value, uint256 limit) {
        if (value <= limit) {
            revert();
        }
        _;
    }

    modifier lock() {
        if (locked) {
            locked = true;
            _;
            locked = false;
        }
    }

    function f(address account) public validAddress(account) {}

    function g(uint256 a) public greaterThan(a, 10) {}

    function refund() public payable {
        payable(msg.sender).transfer(0);
    }
}
