// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

abstract contract Feline {
    // This is how we write the abstract contract
    bytes32 name;

    function setname(bytes32 _name) public {
        name = _name;
    }

    function utterance() public virtual returns (bytes32);

    function Utterance() public virtual returns (bytes32);
}

// inherit the contract in cat and then override the function utterance with some full definition
contract Cat is Feline {
    function utterance() public pure override returns (bytes32) {
        return "miaow";
    }

    function Utterance() public pure override returns (bytes32) {
        return utterance();
    }
}
