pragma solidity ^0.8.4;
contract MyContract {
    struct Data {
        uint a;
        uint b;
    }
    mapping (uint => Data) public items;
    constructor() {
        items[0] = Data(1,2);
        items[1] = Data(3,4);
        items[2] = Data(5,6);
        delete items[1];
    }
}
