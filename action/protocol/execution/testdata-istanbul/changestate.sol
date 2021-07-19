pragma solidity ^0.8.4;

contract ChangeState {
    uint public n = 0;
    event Log(uint n);
    function ChangeStateWithLogFail(uint add) public {
        n += add;
        emit Log(n);
        require(false);
        n++;
    }
}
