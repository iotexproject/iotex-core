// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

#ifndef __BLS_H__
#define __BLS_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include "mnt160twist.h"

/**
 * BLS Signature Generation
 *
 * @param d     the private key of a signer
 * @param msg   a message
 * @param mlen  the length of the message in bytes
 * @param sig   the short signature    
 * Succeed      return 1
 * Fail         return 0
 */
uint32_t BLS_sign(uint32_t *d, const uint8_t *msg, uint64_t mlen, uint32_t *sig);

/**
 * BLS Signature Verification
 *
 * @param Q     the public key of a signer
 * @param msg   a message
 * @param mlen  the length of the message in bytes
 * @param sig   the short signature
 * Valid        return 1
 * Invalid      return 0
 */
uint32_t BLS_verify(ec_point_aff_twist *Q, const uint8_t *msg, uint64_t mlen, uint32_t *sig);

#ifdef __cplusplus
}
#endif

#endif /* BLS_H */