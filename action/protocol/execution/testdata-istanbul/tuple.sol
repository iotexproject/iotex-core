pragma solidity ^0.8.4;
contract A {
    function tuple() pure public returns(uint, string memory) {
        return (1, "Hi");
    }

    function getOne() pure public returns(uint) {
        uint a;
        (a,) = tuple();
        return a;
    }
}
