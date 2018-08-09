// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

#ifndef __FP_H__
#define __FP_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>

#define XTOY(Y,X)    Y[0] = X[0]; Y[1] = X[1]; Y[2] = X[2]; Y[3] = X[3]; Y[4] = X[4]; 
#define XIS0(X)	     X[0] = 0; X[1] = 0; X[2] = 0; X[3] = 0; X[4] = 0;  
#define XIS1(X)	     X[0] = 1; X[1] = 0; X[2] = 0; X[3] = 0; X[4] = 0;  
#define IF1(X)	     X[0] == 1 && X[1] == 0 && X[2] == 0 && X[3] == 0 && X[4] == 0 
#define IFN1(X)	     X[0] != 1 || X[1] != 0 || X[2] != 0 || X[3] != 0 || X[4] != 0  
#define IF0(X)	     X[0] == 0 && X[1] == 0 && X[2] == 0 && X[3] == 0 && X[4] == 0  
#define IFN0(X)	     X[0] != 0 || X[1] != 0 || X[2] != 0 || X[3] != 0 || X[4] != 0  
#define IFXeqY(X,Y)  X[0] == Y[0] && X[1] == Y[1] && X[2] == Y[2] && X[3] == Y[3] && X[4] == Y[4]

#define GET_UINT32(n, b, i)                   \
{                                             \
	(n) = ( (uint32_t) (b)[(i)    ] << 24 )   \
	    | ( (uint32_t) (b)[(i) + 1] << 16 )   \
	    | ( (uint32_t) (b)[(i) + 2] <<  8 )   \
	    | ( (uint32_t) (b)[(i) + 3]       );  \
}

uint32_t mnt160_p[5], mnt160_n[5];
uint32_t mnt160_pu[6], mnt160_nu[6];

void add160(uint32_t *a, uint32_t *b, uint32_t *aplusb);
void sub160(uint32_t *a, uint32_t *b, uint32_t *asubb, uint32_t num);
int32_t cmp(uint32_t *a, uint32_t *b, uint32_t num);
void modp160add(uint32_t *a, uint32_t *b, uint32_t *aplusbp, uint32_t *mp);
void modp160sub(uint32_t *a, uint32_t *b, uint32_t *asubbp, uint32_t *mp);
void modp160div2(uint32_t *a, uint32_t *adiv2p, uint32_t *mp);
void modp160mul(uint32_t *a, uint32_t *b, uint32_t *amulbp, uint32_t *mp);
void modp160sq(uint32_t *a, uint32_t *aap, uint32_t *mp);
void modpfixedpow(uint32_t *a, uint32_t *apow);
uint32_t modp160sqrt(uint32_t *a, uint32_t *asqrtp);
void modp160inv(uint32_t *a, uint32_t *ainvp, uint32_t *mp);
void map_to_z158(uint8_t *rndnum, uint32_t *zn);

#ifdef __cplusplus
}
#endif

#endif  /* FP_H */
