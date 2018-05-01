// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/config"
	ta "github.com/iotexproject/iotex-core/test/testaddress"
)

func TestUTXO(t *testing.T) {
	defer os.Remove(testDBPath)

	// create chain
	totalSupply := uint64(100000000)
	Gen.TotalSupply = totalSupply
	Gen.BlockReward = uint64(0)
	bc := CreateBlockchain(ta.Addrinfo["miner"].RawAddress,
		&config.Config{Chain: config.Chain{ChainDBPath: testDBPath}}, Gen)
	assert.NotNil(t, bc)
	fmt.Println("Create blockchain pass")

	defer bc.Close()

	assert.Equal(t, 0, int(bc.height))
	assert.Nil(t, addTestingBlocks(bc))

	// check all UTXO
	total := bc.BalanceOf(ta.Addrinfo["alfa"].RawAddress)
	fmt.Printf("Alfa balance = %d\n", total)

	beta := bc.BalanceOf(ta.Addrinfo["bravo"].RawAddress)
	fmt.Printf("Bravo balance = %d\n", beta)
	total += beta

	beta = bc.BalanceOf(ta.Addrinfo["charlie"].RawAddress)
	fmt.Printf("Charlie balance = %d\n", beta)
	total += beta

	beta = bc.BalanceOf(ta.Addrinfo["delta"].RawAddress)
	fmt.Printf("Delta balance = %d\n", beta)
	total += beta

	beta = bc.BalanceOf(ta.Addrinfo["echo"].RawAddress)
	fmt.Printf("Echo balance = %d\n", beta)
	total += beta

	beta = bc.BalanceOf(ta.Addrinfo["foxtrot"].RawAddress)
	fmt.Printf("Foxtrot balance = %d\n", beta)
	total += beta

	beta = bc.BalanceOf(ta.Addrinfo["miner"].RawAddress)
	fmt.Printf("test balance = %d\n", beta)
	utxo, _ := bc.Utk.UtxoEntries(ta.Addrinfo["miner"].RawAddress, beta)
	assert.NotNil(t, utxo)
	total += beta

	assert.Equal(t, totalSupply, total)
	fmt.Println("Total balance match")

	/*
		pool := bc.UtxoEntries()
		fmt.Print("UTXO pool")
		for hash, out := range pool {
			fmt.Printf("Hash = %x", hash)
			for _, entry := range out {
				fmt.Printf("Value = %d, Index = %d, script = %x", entry.Value, entry.outIndex, entry.LockScript)
			}
		}
	*/

	/*
		sum := float64(6)
		count := 0
		solution := make(map[float64]bool)

		for y := float64(1); y < 10000; y++ {
			for z := y; z < 10000; z++ {
				ys := y*y
				zs := z*z
				yz := y*z
				delta := math.Pow(ys - sum*yz, 2) - 4*yz*zs

				if _, frac := math.Modf(math.Sqrt(delta)); frac < 0.000000000001 {
					x1 := (sum*yz - ys + math.Sqrt(delta)) / (2*z)
					x2 := (sum*yz - ys - math.Sqrt(delta)) / (2*z)

					if _, frac := math.Modf(x1); x1 > 0 && frac < 0.000000000000001 {
						if VerifySol(x1, y, z, &solution) {
							count++
							fmt.Printf("Found root = %f %f %f", x1, y, z)
						}
					}

					if _, frac := math.Modf(x2); x2 > 0 && frac < 0.000000000000001 {
						if VerifySol(x2, y, z, &solution) {
							count++
							fmt.Printf("Found root = %f %f %f", x2, y, z)
						}
					}
				} else {
					delta := math.Pow(zs - sum*yz, 2) - 4*yz*ys

					if _, frac := math.Modf(math.Sqrt(delta)); frac < 0.000000000001 {
						x1 := (sum*yz - zs + math.Sqrt(delta)) / (2*y)
						x2 := (sum*yz - zs - math.Sqrt(delta)) / (2*y)

						if _, frac := math.Modf(x1); x1 > 0 && frac < 0.000000000000001 {
							if VerifySol(x1, y, z, &solution) {
								count++
								fmt.Printf("Found root = %f %f %f", x1, z, y)
							}
						}

						if _, frac := math.Modf(x2); x2 > 0 && frac < 0.000000000000001 {
							if VerifySol(x2, y, z, &solution) {
								count++
								fmt.Printf("Found root = %f %f %f", x2, z, y)
							}
						}
					}
				}
			}
		}
		fmt.Printf("Has %d solutions\n", count)
	*/
}

func VerifySol(x, y, z float64, sol *map[float64]bool) bool {

	invar1 := y*z + x*y + x*z
	invar2 := x * y * z

	if _, exist := (*sol)[invar1+invar2]; !exist {
		(*sol)[invar1+invar2] = true

		// set multiple of (x,y,z)
		for k := float64(2); k*z <= 10000; k++ {
			invar := k * k * (invar1 + k*invar2)
			(*sol)[invar] = true
		}

		return true
	}

	return false
}
