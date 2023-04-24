//SPDX-License-Identifier: UNLICENSED

pragma solidity ^0.8.0;

contract IIP15Manager {
    mapping (address => address) private contractMapping;
    mapping (address => bool) private isApproved;
    address private owner;

    event ContractRegistered(address contractAddress, address recipient);
    event ContractApproved(address contractAddress);
    event ContractUnapproved(address contractAddress);

    constructor()  {
        owner = msg.sender;
    }

    modifier onlyOwner() {
        require(msg.sender == owner, "Only the contract owner can perform this action");
        _;
    }

    function registerContract(address _contractAddress, address _recipient) public {
        require(_contractAddress != address(0));
        require(_recipient != address(0));
        require(!isApproved[_contractAddress], "Contract is approved");
        contractMapping[_contractAddress] = _recipient;
        emit ContractRegistered(_contractAddress, _recipient);
    }

    function approveContract(address _contractAddress) public onlyOwner {
        address recipient = contractMapping[_contractAddress];
        require(recipient != address(0), "This contract has not been registered");
        isApproved[_contractAddress] = true;
        emit ContractApproved(_contractAddress);
    }

    function disapproveContract(address _contractAddress) public onlyOwner {
        require(isApproved[_contractAddress], "Contract is not approved");
        isApproved[_contractAddress] = false;
        emit ContractUnapproved(_contractAddress);
    }

    function getRecipient(address _contractAddress) public view returns (address) {
        return contractMapping[_contractAddress];
    }

    function isContractApproved(address _contractAddress) public view returns (bool) {
        return isApproved[_contractAddress];
    }

}