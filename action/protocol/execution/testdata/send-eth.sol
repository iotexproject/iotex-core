pragma solidity 0.4.24;
contract MyContract {
    constructor() public payable {}
    address a = 0x1e14d5373E1AF9Cc77F0032aD2cd0FBA8be5Ea2e;
    function transferTo()payable{}
    function foo() {

        // send ether with default 21,000 gas
        // likely causes OOG in callee
        a.send(1 ether);

        // send ether with all remaining gas
        // but no success check!
        a.call.value(1 ether)();

        // RECOMMENDED
        // send all remaining gas
        // explicitly handle callee throw
        if(!a.call.value(1 ether)()) throw;
    }
}
