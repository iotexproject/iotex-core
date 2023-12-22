// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.14;

contract Multisend {
    event Transfer(address recipient, uint256 amount);
    event Refund(uint256 refund);
    event Payload(string payload);

    function multiSend(
        address[] memory recipients,
        uint256[] memory amounts,
        string memory payload
    ) public payable {
        require(
            recipients.length <= 300,
            "number of recipients is larger than 300"
        );
        require(recipients.length == amounts.length, "parameters not match");
        uint256 totalAmount = 0;
        for (uint256 i = 0; i < recipients.length; i++) {
            totalAmount += amounts[i];
        }
        require(msg.value >= totalAmount, "not enough token");
        uint256 refund = msg.value - totalAmount;
        for (uint256 i = 0; i < recipients.length; i++) {
            payable(recipients[i]).transfer(amounts[i]);
            emit Transfer(recipients[i], amounts[i]);
        }
        if (refund > 0) {
            payable(msg.sender).transfer(refund);
            emit Refund(refund);
        }
        emit Payload(payload);
    }
}
