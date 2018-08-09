// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

#ifndef __SHAMIR_H__
#define __SHAMIR_H__

#ifdef __cplusplus
extern "C" {
#endif

#define DEGREE 10      //Threshold
#define NUMNODES 21    //Number of delegates  
#define IDLENGTH 32

#include <stdint.h>

/**
 * Sharmir Secret Sharing -- Initialization
 *
 * @param ms      the master secret
 * @param coeffs  the coefficients of the polynomial
 * Succeed        return 1
 * Fail           return 0
 */
uint32_t ss_init(uint32_t *ms, uint32_t coeffs[DEGREE+1][5]);

/**
 * Sharmir Secret Sharing -- Share Generation
 *
 * @param id      the participant id
 * @param coeffs  the coefficients of the polynomial
 * @param share   the share of the master secret
 */
void ss_share_gen(const uint8_t *id, uint32_t coeffs[DEGREE+1][5], uint32_t *share);

/**
 * Sharmir Secret Sharing -- Lagrange Coefficients
 *
 * @param ids     the other participant ids
 * @param coeffs  the largcoefficients
 */
void ss_lagrange_coeffs(const uint8_t ids[DEGREE+1][IDLENGTH], uint32_t lambda[DEGREE+1][5]);

#ifdef __cplusplus
}
#endif

#endif /* SHAMIR_H */
