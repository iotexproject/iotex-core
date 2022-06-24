// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract tester {
    string public A;
    event logTest(uint256 n);

    function test(
        uint256 mul,
        uint256 shift,
        uint256 add,
        uint256 log
    ) public returns (uint256 a) {
        a = 7;
        for (uint256 i = 0; i < mul; i++) {
            a = (a * 10007) % 100000007;
        }
        for (uint256 i = 0; i < shift; i++) {
            a = i << 7;
        }
        for (uint256 i = 0; i < add; i++) {
            a = (a + 100000009) % 10007;
        }
        for (uint256 i = 0; i < log; i++) {
            emit logTest(i);
        }
    }

    function storeString(string memory a) public {
        A = a;
    }
}
