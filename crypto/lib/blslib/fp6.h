// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

#ifndef __FP6_H__
#define __FP6_H__

#ifdef __cplusplus
extern "C" {
#endif

#include "fp3.h"

typedef struct 
{
	Fp3Elm c, d;
}Fp6Elm;	

void setoneFp6(Fp6Elm *m);
uint32_t equalFp6(Fp6Elm *m, Fp6Elm *n);
void setFp6(Fp6Elm *m, Fp6Elm *n);
void setFp6Fp3(Fp6Elm *m, Fp3Elm *n1, Fp3Elm *n2);
void modconjFp6(Fp6Elm *m, Fp6Elm *n);
void frobeniusFp6(Fp6Elm *m, Fp6Elm *n); 
void modaddFp6(Fp6Elm *m, Fp6Elm *n, Fp6Elm *maddn);
void modsubFp6(Fp6Elm *m, Fp6Elm *n, Fp6Elm *msubn);
void modmulFp6(Fp6Elm *m, Fp6Elm *n, Fp6Elm *mmuln);
void modsqFp6(Fp6Elm *m, Fp6Elm *mm);
void modinvFp6(Fp6Elm *m, Fp6Elm *minv);
void modexpzFp6(Fp6Elm *m, Fp6Elm *n);

#ifdef __cplusplus
}
#endif

#endif  /* FP6_H */