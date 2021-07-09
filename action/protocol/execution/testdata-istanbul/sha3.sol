pragma solidity ^0.8.4;
contract Sha3 {
    function hashArray() pure public returns(bytes32) {
        bytes8[] memory tickers = new bytes8[](4);
        tickers[0] = bytes8('BTC');
        tickers[1] = bytes8('ETH');
        tickers[2] = bytes8('LTC');
        tickers[3] = bytes8('DOGE');
        return keccak256(abi.encodePacked(tickers));
        // 0x374c0504f79c1d5e6e4ded17d488802b5656bd1d96b16a568d6c324e1c04c37b
    }

    function hashPackedArray() pure public returns(bytes32) {
        bytes8 btc = bytes8('BTC');
        bytes8 eth = bytes8('ETH');
        bytes8 ltc = bytes8('LTC');
        bytes8 doge = bytes8('DOGE');
        return keccak256(abi.encodePacked(btc, eth, ltc, doge));
        // 0xe79a6745d2205095147fd735f329de58377b2f0b9f4b81ae23e010062127f2bc
    }

    function hashAddress() pure public returns(bytes32) {
        address account = 0x6779913e982688474F710B47E1c0506c5Dca4634;
        return keccak256(abi.encodePacked(account));
        // 0x229327de236bd04ccac2efc445f1a2b63afddf438b35874b9f6fd1e6c38b0198
    }

    function testPackedArgs() pure public returns (bool) {
        return keccak256('ab') == keccak256(abi.encodePacked('a', 'b'));
    }

    function hashHex() pure public returns (bytes32) {
        bytes1 i = 0x0a;
        return keccak256(abi.encodePacked(i));
        // 0x0ef9d8f8804d174666011a394cab7901679a8944d24249fd148a6a36071151f8
    }

    function hashInt() pure public returns (bytes32) {
        return keccak256(abi.encodePacked(int(1)));
    }

    function hashNegative() pure public returns (bytes32) {
        return keccak256(abi.encodePacked(int(-1)));
    }

    function hash8() pure public returns (bytes32) {
        return keccak256(abi.encodePacked(uint8(1)));
    }

    function hash32() pure public returns (bytes32) {
        return keccak256(abi.encodePacked(uint32(1)));
    }

    function hash256() pure public returns (bytes32) {
        return keccak256(abi.encodePacked(uint(1)));
    }

    function hashEth() pure public returns (bytes32) {
        return keccak256(abi.encodePacked(uint(100 ether)));
    }

    function hashWei() pure public returns (bytes32) {
        return keccak256(abi.encodePacked(uint(100)));
    }

    function hashMultipleArgs() pure public returns (bytes32) {
        return keccak256(abi.encodePacked('a', uint(1)));
    }

    function hashString() pure public returns (bytes32) {
        return keccak256('a');
    }
}
