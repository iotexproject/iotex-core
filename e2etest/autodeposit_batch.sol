// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

// AutoDepositBatch wraps the immutable mainnet AutoDeposit contract with a
// batch view. Used by BenchmarkAutoDeposit_bucket_WrapperContract in
// action/protocol/execution/protocol_iip59_bench_test.go to measure
// mitigation option (1a) — a wrapper-contract batch view — against the
// direct-storage-read mitigation.
//
// Compile:
//   solc --bin --optimize --optimize-runs 200 \
//     -o out --overwrite autodeposit_batch.sol
//
// The resulting AutoDepositBatch.bin becomes the
// e2etest/autodeposit_batch_init_bytecode fixture (init bytecode; the
// bench appends the abi-encoded AutoDeposit address as constructor arg).

interface IAutoDeposit {
    function bucket(address owner) external view returns (int256);
}

contract AutoDepositBatch {
    IAutoDeposit public immutable target;

    constructor(address _target) {
        target = IAutoDeposit(_target);
    }

    function buckets(address[] calldata owners) external view returns (int256[] memory) {
        int256[] memory out = new int256[](owners.length);
        for (uint256 i = 0; i < owners.length; i++) {
            out[i] = target.bucket(owners[i]);
        }
        return out;
    }
}
