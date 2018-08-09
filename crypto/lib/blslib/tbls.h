// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

#ifndef __TBLS_H__
#define __TBLS_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include "shamir.h"
#include "mnt160twist.h"

/**
 * BLS Threshold Signature Generation
 *
 * @param ds    the private key share of a signer
 * @param msg   a message
 * @param mlen  the length of the message in bytes
 * @param sigs  the short signature share    
 * Succeed      return 1
 * Fail         return 0
 */
uint32_t TBLS_sign_share(uint32_t *ds, const uint8_t *msg, uint64_t mlen, uint32_t *sigs);

/**
 * BLS Threshold Signature Share Verification
 *
 * @param Qt    the public key share of a signer
 * @param msg   a message
 * @param mlen  the length of the message in bytes
 * @param sigs  the short signature share
 * Valid        return 1
 * Invalid      return 0
 */
uint32_t TBLS_verify_share(ec_point_aff_twist *Qt, const uint8_t *msg, uint64_t mlen, uint32_t *sigs);

/**
 * BLS Threshold Signature Aggregation (Same Message)
 *
 * @param ids     the other participant ids
 * @param sigs    the short signature share from other participants
 * @param aggsig  the aggregated short signature 
 * Succeed        return 1
 * Fail           return 0  
 */
uint32_t TBLS_sign_aggregate(uint8_t ids[DEGREE + 1][IDLENGTH], uint32_t sigs[DEGREE + 1][5], uint32_t *aggsig);

/**
 * BLS Threshold Signature Verification
 *
 * @param ids      the other participant ids
 * @param Qts      the public key shares from other participants
 * @param msg      a message
 * @param mlen     the length of the message in bytes
 * @param aggsig   the aggregated short signature
 * Valid           return 1
 * Invalid         return 0
 */
uint32_t TBLS_verify_aggregate(uint8_t ids[DEGREE + 1][IDLENGTH], ec_point_aff_twist Qts[DEGREE + 1], const uint8_t *msg, uint64_t mlen, uint32_t *aggsig);

#ifdef __cplusplus
}
#endif

#endif /* BLS_H */