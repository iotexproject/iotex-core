// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// EIP-2537: Precompile for BLS12-381 curve operations
// Tests for BLS12-381 precompiles at addresses 0x0b-0x13
contract BLS12381Test {
    // EIP-2537 BLS12-381 Precompile Addresses
    address constant BLS12_G1ADD = address(0x0b);
    address constant BLS12_G1MULTIEXP = address(0x0c);
    address constant BLS12_G2ADD = address(0x0d);
    address constant BLS12_G2MULTIEXP = address(0x0e);
    address constant BLS12_PAIRING = address(0x0f);
    address constant BLS12_MAP_FP_TO_G1 = address(0x10);
    address constant BLS12_MAP_FP2_TO_G2 = address(0x11);

    function testG1Add() public view returns (bytes memory) {
        bytes memory input = hex"000000000000000000000000000000000572cbea904d67468808c8eb50a9450c9721db309128012543902d0ac358a62ae28f75bb8f1c7c42c39a8c5529bf0f4e00000000000000000000000000000000166a9d8cabc673a322fda673779d8e3822ba3ecb8670e461f73bb9021d5fd76a4c56d9d4cd16bd1bba86881979749d280000000000000000000000000000000009ece308f9d1f0131765212deca99697b112d61f9be9a5f1f3780a51335b3ff981747a0b2ca2179b96d2c0c9024e522400000000000000000000000000000000032b80d3a6f5b09f8a84623389c5f80ca69a0cddabc3097f9d9c27310fd43be6e745256c634af45ca3473b0590ae30d1";
        
        (bool success, bytes memory result) = BLS12_G1ADD.staticcall(input);
        require(success, "G1ADD failed");
        return result;
    }

    function testG1Mul() public view returns (bytes memory) {
        bytes memory input = hex"0000000000000000000000000000000017f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb0000000000000000000000000000000008b3f481e3aaa0f1a09e30ed741d8ae4fcf5e095d5d00af600db18cb2c04b3edd03cc744a2888ae40caa232946c5e7e10000000000000000000000000000000000000000000000000000000000000002";
        
        (bool success, bytes memory result) = BLS12_G1MULTIEXP.staticcall(input);
        require(success, "G1MUL failed");
        return result;
    }

    function testMapFpToG1() public view returns (bytes memory) {
        bytes memory input = hex"0000000000000000000000000000000014406e5bfb9209256a3820879a29ac2f62d6aca82324bf3ae2aa7d3c54792043bd8c791fccdb080c1a52dc68b8b69350";
        
        (bool success, bytes memory result) = BLS12_MAP_FP_TO_G1.staticcall(input);
        require(success, "MAP_FP_TO_G1 failed");
        return result;
    }
}
