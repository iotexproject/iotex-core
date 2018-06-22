/*
 * Copyright (c) 2018 IoTeX
 * This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
 * warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
 * permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
 * License 2.0 that can be found in the LICENSE file.
 */

#ifndef __SCHNORR_H__
#define __SCHNORR_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include "sect283k1.h"

typedef struct
{        
    uint32_t s[9];
    ec283_point_lambda_aff R;
}schnorr_signature;

/**
 * Schnorr signature generation
 *
 * @param d     the private key of a signer
 * @param msg   a message
 * @param mlen  the length of the message in bytes
 * @param sig   the signature pair (s, R)    
 * Succeed      return 1
 * Fail         return 0
 */
uint32_t schnorr_sign(uint32_t *d, const uint8_t *msg, uint64_t mlen, schnorr_signature *sig);

/**
 * Schnorr signature verification
 *
 * @param Q     the public key of a signer
 * @param msg   a message
 * @param mlen  the length of the message in bytes
 * @param sig   the signature pair (r, s)
 * Valid        return 1
 * Invalid      return 0
 */
uint32_t schnorr_verify(ec283_point_lambda_aff *Q, const uint8_t *msg, uint64_t mlen, schnorr_signature *sig);

#ifdef __cplusplus
}
#endif

#endif /* SCHNORR_H */