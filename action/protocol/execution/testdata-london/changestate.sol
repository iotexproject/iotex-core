// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract ChangeState {
    uint256 public n = 0;
    event Log(uint256 n);

    function ChangeStateWithLogFail(uint256 add) public {
        n += add;
        emit Log(n);
        require(false);
        n++;
    }
}
