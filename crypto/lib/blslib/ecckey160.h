// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

#ifndef __ECCKEY160_H__
#define __ECCKEY160_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include "mnt160.h"

typedef struct
{
	uint32_t d[5];         
    ec160_point_aff Q;
}ec160_key_pair;

/**
 * ECDSA Public key generation
 *
 * @param sk   an ECDSA private key
 * @param Q    an ECDSA public key
 * Succeed     return 1
 * Fail        return 0
 */
uint32_t ec160_pk_generation(uint32_t *sk, ec160_point_aff *Q);

/**
 * Public key validation
 *
 * @param Q   a public key
 * valid      return 1
 * invalid    return 0
 */
uint32_t pk160_validation(ec160_point_aff *Q);

/**
 * Key pair generation
 *
 * @param key   a key pair
 */
void keypair160_generation(ec160_key_pair *key);

#ifdef __cplusplus
}
#endif

#endif /* ECCKEY160_H */
