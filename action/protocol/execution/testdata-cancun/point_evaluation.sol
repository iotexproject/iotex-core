// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

contract PointEvaluationTest {

    function evaluate() public view returns (bytes memory) {
        bytes memory input = new bytes(192);
        input = hex"016ba00cade4653a609ffbfb20363fbc5ec4d09f58f3598fc96677a2aaad283701020000000000000000000000000000000000000000000000000000000000004694f6339d1000d34e693cb767c18c5050fe74ec37e131d4edbe67981ab684daacf5a66c385c699499e40c9b33631bd23341229b9766dad873f04850fcea87309152dbf33f71001495d94772a596f140874a7613e31ecf82254e385162167d6a0909fd5deec5a0dba6ee660dd079a5fe863cc22d2b6dd0a4d64e6218945d8ce9";
        // call precompile contract
        (bool ok, bytes memory out) = address(0x0a).staticcall(input);
        require(ok);
        return out;
    }
}