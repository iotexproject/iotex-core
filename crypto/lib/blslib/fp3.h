// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

#ifndef __FP3_H__
#define __FP3_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>

typedef struct 
{
	uint32_t a0[5], a1[5], a2[5];
}Fp3Elm;

void setzeroFp3(Fp3Elm *m);
void setoneFp3(Fp3Elm *m);
uint32_t equalFp3(Fp3Elm *m, Fp3Elm *n);
void setFp3(Fp3Elm *m, Fp3Elm *n);
void moddiv2Fp3(Fp3Elm *m, Fp3Elm *n);
void modaddFp3(Fp3Elm *m, Fp3Elm *n, Fp3Elm *maddn);
void modsubFp3(Fp3Elm *m, Fp3Elm *n, Fp3Elm *msubn);
void modmulFp3(Fp3Elm *m, Fp3Elm *n, Fp3Elm *mmuln);
void modmulFp3Fp(Fp3Elm *m, uint32_t *n, Fp3Elm *mmulnp);
void modsqFp3(Fp3Elm *m, Fp3Elm *mm);
void modinvFp3(Fp3Elm *m, Fp3Elm *minv);

#ifdef __cplusplus
}
#endif

#endif  /* FP3_H */