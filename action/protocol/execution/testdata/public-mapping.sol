pragma solidity 0.4.24;
contract AA {
    mapping(uint => uint) public objects;

    function Set() {
        objects[0] = 42;
    }
}

contract BB {
    // insert address of deployed First here
    AA a = AA(0x675f1057f81e9e768e33faddbd5609c09f4c0a5c);

    function get() returns(uint) {
        a.Set();
        return a.objects(0);
    }
}
