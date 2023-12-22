// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract MyContract {
    // naive recursion
    function sum(uint256 n) public returns (uint256) {
        return n == 0 ? 0 : n + sum(n - 1);
    }

    // loop
    function sumloop(uint256 n) public pure returns (uint256) {
        uint256 total = 0;
        for (uint256 i = 1; i <= n; i++) {
            total += i;
        }
        return total;
    }

    // tail-recursion
    function sumtailHelper(uint256 n, uint256 acc) private returns (uint256) {
        return n == 0 ? acc : sumtailHelper(n - 1, acc + n);
    }

    function sumtail(uint256 n) public returns (uint256) {
        return sumtailHelper(n, 0);
    }
}
