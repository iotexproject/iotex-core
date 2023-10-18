// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract tester {
    string[] public A;

    function storeStrings(string memory a, int256 n) public {
        for (int256 i = 0; i < n; i++) {
            A.push(a);
        }
    }
}
