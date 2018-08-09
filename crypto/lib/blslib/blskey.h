// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

#ifndef __BLSKEY_H__
#define __BLSKEY_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include "mnt160twist.h"

/**
 * BLS Public key generation
 *
 * @param sk   a BLS private key
 * @param Qt   a BLS public key
 * Succeed     return 1
 * Fail        return 0
 */
uint32_t bls_pk_generation(uint32_t *sk, ec_point_aff_twist *Qt);

/**
 * BLS Public key validation
 *
 * @param Qt   a BLS public key
 * valid       return 1
 * invalid     return 0
 */
uint32_t bls_pk_validation(ec_point_aff_twist *Qt);

#ifdef __cplusplus
}
#endif

#endif /* BLSKEY_H */
