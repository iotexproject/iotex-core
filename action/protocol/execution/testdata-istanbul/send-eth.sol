pragma solidity ^0.8.4;
contract MyContract {
    constructor() payable {}
    address a = 0x1e14d5373E1AF9Cc77F0032aD2cd0FBA8be5Ea2e;
    function transferTo() public payable{}
    function foo() public {

        // send ether with default 21,000 gas
        // likely causes OOG in callee
        payable(a).send(1 ether);

        // send ether with all remaining gas
        // but no success check!
        payable(a).call{value: 1 ether}("");

        // RECOMMENDED
        // send all remaining gas
        // explicitly handle callee throw
        bool ok;
        bytes memory ret;
        (ok, ret) = a.call{value: 1 ether}("");
        if(!ok) revert();
    }
}
