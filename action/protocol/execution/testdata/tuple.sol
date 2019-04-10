pragma solidity 0.4.24;
contract A {
    function tuple() returns(uint, string) {
        return (1, "Hi");
    }

    function getOne() returns(uint) {
        uint a;
        (a,) = tuple();
        return a;
    }
}
