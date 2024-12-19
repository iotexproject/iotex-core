package gasstation

import "math/big"

type blockFee struct {
	baseFee      *big.Int
	gasUsedRatio float64
	blobBaseFee  *big.Int
	blobGasRatio float64
}

type blockPercents struct {
	ascEffectivePriorityFees []*big.Int
}
