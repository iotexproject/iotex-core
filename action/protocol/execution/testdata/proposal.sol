pragma solidity 0.4.24;

/** A proposal contract with O(1) approvals. */

contract Proposal {
    mapping (address => bool) approvals;
    bytes32 public approvalMask;
    bytes32 public approver1;
    bytes32 public approver2;
    bytes32 public target;
    
    function Proposal() public {
        approver1 = 0x00000000000000000000000000000000000000123;
        approver2 = bytes32(msg.sender);
        target = approver1 | approver2;
    }
    
    function approve(address approver) public {
        approvalMask = bytes32(approver) | bytes32(msg.sender);
        approvals[approver] = true;
    }
    
    function isApproved() public constant returns(bool) {
        return approvalMask == target;
    }
}
