// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract BlockHashQuery {
    address constant HISTORY_STORAGE_ADDRESS = 0x0000F90827F1C53a10cb7A02335B175320002935;
    
    function getBlockHash(uint256 blockNumber) public view returns (bytes32 blockHash) {

        (bool success, bytes memory result) = HISTORY_STORAGE_ADDRESS.staticcall(
            abi.encode(blockNumber)
        );
        
        require(success, "Failed to get block hash");
        
        blockHash = abi.decode(result, (bytes32));
    }
}