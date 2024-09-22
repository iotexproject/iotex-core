// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

contract PointEvaluationTest {

    function evaluate() public view returns (bytes memory) {
        bytes memory input = new bytes(192);
        // call precompile contract
        (bool ok, bytes memory out) = address(0x0a).staticcall(abi.encode(input));
        require(ok);
        return out;
    }
}