/*
 * Copyright (c) 2018 IoTeX
 * This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
 * warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
 * permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
 * License 2.0 that can be found in the LICENSE file.
 */

#ifndef __BLAKE256_H__
#define __BLAKE256_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>

typedef struct {
  uint32_t h[8];
  uint32_t s[4];
  uint32_t t[2];
  uint8_t buf[64];
  int buflen;
  int nullt;
}blake256_state;

static void blake256_init(blake256_state *S);
static void blake256_update(blake256_state *S, const uint8_t *data, uint64_t datalen);
static void blake256_final(blake256_state *S, uint8_t *digest);
void blake256_hash(uint8_t *out, const uint8_t *in, uint64_t inlen);

#ifdef __cplusplus
}
#endif

#endif /* BLAKE256_H */