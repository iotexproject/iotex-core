// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

#ifndef __DKG_H__
#define __DKG_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include "mnt160.h"
#include "shamir.h"

/**
 * Private key generation
 *
 * @param sk  a private key
 */
void dkg_sk_generation(uint32_t *sk);

/**
 * Distributed Key Generation -- Initialization
 *
 * @param ms          the master secret
 * @param coeffs      the coefficients of the polynomial
 * @param ids         other delegate ids
 * @param shares      the secret shares for other delegates
 * @param witnesses   the witnesses for the polynomial coefficients 
 * 
 * Succeed        return 1
 * Fail           return 0
 */
uint32_t dkg_init(uint32_t *ms, uint32_t coeffs[DEGREE+1][5], uint8_t ids[NUMNODES][IDLENGTH], uint32_t shares[NUMNODES][5], ec160_point_aff witnesses[DEGREE+1]);

/**
 * Distributed Key Generation -- Share Verification
 *
 * @param id            the peers' identifier
 * @param share         the received secret share
 * @param witnesses     the witnesses for id's polynomial coefficients
 * 
 * Valid                return 1
 * Invalid              return 0
 * 
 */
uint8_t dkg_share_verify(const uint8_t *id, uint32_t *share, ec160_point_aff witnesses[DEGREE+1]);
  
/**
 * Distributed Key Generation -- Share Collection
 *
 * @param ids           the peers' identifiers
 * @param shares        the secret shares received from different peers
 * @param witnesses     the witnesses for id's polynomial coefficients
 * @param sharestatus   the status of secret shares (1 -- valid  0 -- invalid) 
 * 
 */  
void dkg_shares_collect(uint8_t *id, uint32_t shares[NUMNODES][5], ec160_point_aff witnesses[NUMNODES][DEGREE+1], uint8_t sharestatus[NUMNODES]);

/**
 * Distributed Key Generation -- Share Update
 *
 * @param id          the peer's identifier
 * @param newshare    the new secret share received from id
 * @param witnesses   the witnesses for id's polynomial coefficients
 * Valid              return 1
 * Invalid            return 0 
 * 
 */
uint8_t dkg_share_update(uint8_t *id, uint32_t *newshare, ec160_point_aff witnesses[DEGREE+1]);

/**
 * Distributed Key Generation -- Key Pair Generation
 *
 * @param shares               the secret shares received from the peers
 * @param sharesstatusmatrix   the verification status for all the shares
 * @param sk                   the private-key share of a group private key
 * @param Qs                   the short-lived ECDSA public key
 * @param Qt                   the public-key share of a group public key
 * 
 */
void dkg_keypair_gen(uint32_t shares[NUMNODES][5], uint8_t sharestatusmatrix[NUMNODES][NUMNODES], uint32_t *sk, ec160_point_aff *Qs, ec_point_aff_twist *Qt);

#ifdef __cplusplus
}
#endif

#endif /* DKG_H */
