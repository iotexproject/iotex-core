//SPDX-License-Identifier: UNLICENSED

pragma solidity ^0.8.0;

import "https://github.com/OpenZeppelin/openzeppelin-contracts/blob/master/contracts/access/Ownable.sol";

contract IIP15Manager is Ownable {

    struct ContractInfo {
        address recipient;
        bool isRegistered;
        bool isApproved;
    }

    mapping (address => ContractInfo) private contractRegistry;

    event ContractRegistered(address contractAddress, address recipient);
    event ContractApproved(address contractAddress);
    event ContractUnapproved(address contractAddress);
    event ContractRemoved(address contractAddress);

    function registerContract(address _contractAddress, address _recipient) public {
        require(_contractAddress != address(0), "Contract address cannot be zero");
        require(_recipient != address(0), "Recipient address cannot be zero");
        require(!contractRegistry[_contractAddress].isApproved, "Contract is already approved");
        contractRegistry[_contractAddress] = ContractInfo(_recipient, true, false);
        emit ContractRegistered(_contractAddress, _recipient);
    }

    function approveContract(address _contractAddress) public onlyOwner {
        ContractInfo storage contractInfo = contractRegistry[_contractAddress];
        require(contractInfo.isRegistered, "Contract is not registered");
        require(!contractInfo.isApproved, "Contract is already approved");
        contractInfo.isApproved = true;
        emit ContractApproved(_contractAddress);
    }

    function disapproveContract(address _contractAddress) public onlyOwner {
        ContractInfo storage contractInfo = contractRegistry[_contractAddress];
        require(contractInfo.isRegistered, "Contract is not registered");
        require(contractInfo.isApproved, "Contract is not approved");
        contractInfo.isApproved = false;
        emit ContractUnapproved(_contractAddress);
    }

    function removeContract(address _contractAddress) public onlyOwner {
        require(contractRegistry[_contractAddress].isRegistered, "Contract is not registered");
        delete contractRegistry[_contractAddress];
        emit ContractRemoved(_contractAddress);
    }

    function getContract(address _contractAddress) public view returns (ContractInfo memory) {
        return contractRegistry[_contractAddress];
    }

}
