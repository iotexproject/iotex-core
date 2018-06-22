/*
 * Copyright (c) 2018 IoTeX
 * This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
 * warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
 * permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
 * License 2.0 that can be found in the LICENSE file.
 */

#ifndef __GF2283_H__
#define __GF2283_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>

void modadd(uint32_t *a, uint32_t *b, uint32_t *c);
void modmul(uint32_t *a, uint32_t *b, uint32_t *c);
void modsq(uint32_t *a, uint32_t *b);
static void fastred(uint32_t *a, uint32_t *b);
void modsqrt(uint32_t *a, uint32_t *b);
void modinv(uint32_t *a, uint32_t *b);
int32_t compare(uint32_t *a, uint32_t *b, uint32_t m);

#ifdef __cplusplus
}
#endif

#endif /* GF2283_H */