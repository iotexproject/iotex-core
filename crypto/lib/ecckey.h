/*
 * Copyright (c) 2018 IoTeX
 * This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
 * warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
 * permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
 * License 2.0 that can be found in the LICENSE file.
 */

#ifndef __ECCKEY_H__
#define __ECCKEY_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include "sect283k1.h"

typedef struct
{
	uint32_t d[9];         
    ec283_point_lambda_aff Q;
}ec283_key_pair;

/**
 * Public key validation
 *
 * @param Q   a public key
 * valid      return 1
 * invalid    return 0
 */
uint32_t pk_validation(ec283_point_lambda_aff *Q);

/**
 * Key generation
 *
 * @param key   a key pair
 */
void key_generation(ec283_key_pair *key);

#ifdef __cplusplus
}
#endif

#endif /* ECCKEY_H */