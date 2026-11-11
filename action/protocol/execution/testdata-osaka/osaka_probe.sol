// SPDX-License-Identifier: MIT
pragma solidity ^0.8.31;

// Probes for the Osaka EVM changes (compiled with solc 0.8.33, --evm-version osaka):
//   EIP-7939 CLZ opcode
//   EIP-7823 / EIP-7883 MODEXP (0x05) input bound and repricing
//   EIP-7951 P256VERIFY precompile at 0x100
// and for IoTeX's own precompiles 0x8001 (secp256r1) / 0x8002 (scrypt), which
// must behave under Osaka exactly as they do under Prague.
contract OsakaProbe {
    uint256 public lastClz;

    function clz(uint256 x) public pure returns (uint256 r) {
        assembly {
            r := clz(x)
        }
    }

    function storeClz(uint256 x) public {
        uint256 r;
        assembly {
            r := clz(x)
        }
        lastClz = r;
    }

    // probe calls target with input and reports whether the call succeeded,
    // the gas the call consumed (as seen by the caller) and its output
    function probe(address target, bytes memory input) public view returns (bool ok, uint256 gasUsed, bytes memory out) {
        uint256 g = gasleft();
        (ok, out) = target.staticcall(input);
        gasUsed = g - gasleft();
    }

    // modexpLens calls MODEXP with the given base / exponent / modulus
    // lengths, building the input here so the transaction data stays small.
    // base = 0x02.., exponent = 0x03.., modulus = 0x07.. (last byte of each).
    function modexpLens(uint256 bl, uint256 el, uint256 ml) public view returns (bool ok, uint256 gasUsed, bytes memory out) {
        bytes memory b = new bytes(bl);
        bytes memory e = new bytes(el);
        bytes memory m = new bytes(ml);
        if (bl > 0) b[bl - 1] = 0x02;
        if (el > 0) e[el - 1] = 0x03;
        if (ml > 0) m[ml - 1] = 0x07;
        bytes memory input = abi.encodePacked(bl, el, ml, b, e, m);
        uint256 g = gasleft();
        (ok, out) = address(0x05).staticcall(input);
        gasUsed = g - gasleft();
    }
}
