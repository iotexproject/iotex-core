pragma solidity ^0.5.3;

contract Feline {
    // This is how we write the abstract contract 
    bytes32 name; 
    function setname (bytes32 _name) public {
        name = _name; 
    }
    function utterance() public returns (bytes32);
    function Utterance() public returns (bytes32);
}

// inherit the contract in cat and then override the function utterance with some full definition
contract Cat is Feline {
    function utterance() public returns (bytes32) { return "miaow"; }
    function Utterance() public returns (bytes32) {
        return utterance();
    }
}
