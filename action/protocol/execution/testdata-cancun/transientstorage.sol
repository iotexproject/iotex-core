// This support was introduced with Solidity 0.8.24
pragma solidity 0.8.24;
// SPDX-License-Identifier: Unlicensed

contract TransientStorage {

    // Sets a number in the transient storage
    function getNumber() public returns (uint) {
        tstore(0, uint(33));
        return uint(tload(0));
    }

    // Sets an address in the transient storage
    function getAddress() public returns (address) {
        tstore(1, uint(uint160(address(3))));
        return address(uint160(tload(1)));
    }

    function tstore(uint location, uint value) public {
        assembly {
            tstore(location, value)
        }
    }

    function tload(uint location) public view returns (uint value) {
        assembly {
            value := tload(location)
        }
    }
}
