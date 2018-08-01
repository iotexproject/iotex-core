/*
 * Copyright (c) 2018 IoTeX
 * This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
 * warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
 * permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
 * License 2.0 that can be found in the LICENSE file.
 */

#ifndef __GFP_H__
#define __GFP_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>

extern uint32_t ecc283_n[9];
extern uint32_t ecc283_u[10];

uint32_t add(uint32_t *a, uint32_t *b, uint32_t *aplusb);
uint32_t sub(uint32_t *a, uint32_t *b, uint32_t *asubb, uint32_t num);
static void mul(uint32_t *a, uint32_t *b, uint32_t *amulb);
static void modpdiv2(uint32_t *a, uint32_t *adiv2p);
void modpadd(uint32_t *a, uint32_t *b, uint32_t *aplusb);
void modpmul(uint32_t *a, uint32_t *b, uint32_t *amulbp);
void modpinv(uint32_t *a, uint32_t *ainvp);
void map_to_z281(uint8_t *rndnum, uint32_t *zn);

#ifdef __cplusplus
}
#endif

#endif  /* GFP_H */
