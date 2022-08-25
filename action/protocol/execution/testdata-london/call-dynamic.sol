// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract MyContract {
    uint256 x = 0;

    function foo(uint256 _x) public {
        x = 10 + _x;
    }

    function bar() public returns (uint256) {
        address(this).call(abi.encodeWithSignature("foo(uint256)", 1));
        return x; // returns 11
    }
}
