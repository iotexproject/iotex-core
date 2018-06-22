/*
 * Copyright (c) 2018 IoTeX
 * This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
 * warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
 * permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
 * License 2.0 that can be found in the LICENSE file.
 */

#ifndef __SECT283K1_H__
#define __SECT283K1_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>

#define XTOY(Y,X)    Y[0] = X[0]; Y[1] = X[1]; Y[2] = X[2]; Y[3] = X[3]; Y[4] = X[4]; Y[5] = X[5]; Y[6] = X[6]; Y[7] = X[7]; Y[8] = X[8];
#define XIS0(X)	     X[0] = 0; X[1] = 0; X[2] = 0; X[3] = 0; X[4] = 0; X[5] = 0; X[6] = 0; X[7] = 0; X[8] = 0; 
#define XIS1(X)	     X[0] = 1; X[1] = 0; X[2] = 0; X[3] = 0; X[4] = 0; X[5] = 0; X[6] = 0; X[7] = 0; X[8] = 0;
#define IF1(X)	     X[0] == 1 && X[1] == 0 && X[2] == 0 && X[3] == 0 && X[4] == 0 && X[5] == 0 && X[6] == 0 && X[7] == 0 && X[8] == 0
#define IFN1(X)	     X[0] != 1 || X[1] != 0 || X[2] != 0 || X[3] != 0 || X[4] != 0 || X[5] != 0 || X[6] != 0 || X[7] != 0 || X[8] != 0
#define IF0(X)	     X[0] == 0 && X[1] == 0 && X[2] == 0 && X[3] == 0 && X[4] == 0 && X[5] == 0 && X[6] == 0 && X[7] == 0 && X[8] == 0
#define IFN0(X)	     X[0] != 0 || X[1] != 0 || X[2] != 0 || X[3] != 0 || X[4] != 0 || X[5] != 0 || X[6] != 0 || X[7] != 0 || X[8] != 0
#define IFXeqY(X,Y)  X[0] == Y[0] && X[1] == Y[1] && X[2] == Y[2] && X[3] == Y[3] && X[4] == Y[4] && X[5] == Y[5] && X[6] == Y[6] && X[7] == Y[7] && X[8] == Y[8]

#define GET_UINT32(n, b, i)                   \
{                                             \
	(n) = ( (uint32_t) (b)[(i)    ] << 24 )   \
	    | ( (uint32_t) (b)[(i) + 1] << 16 )   \
	    | ( (uint32_t) (b)[(i) + 2] <<  8 )   \
	    | ( (uint32_t) (b)[(i) + 3]       );  \
}

#define PUT_UINT32(n, b, i)                   \
{                                             \
    (b)[(i)    ] = (uint8_t) ( (n) >> 24 );   \
    (b)[(i) + 1] = (uint8_t) ( (n) >> 16 );   \
    (b)[(i) + 2] = (uint8_t) ( (n) >>  8 );   \
    (b)[(i) + 3] = (uint8_t) ( (n)       );   \
}

typedef struct
{
	uint32_t x[9];     
    uint32_t l[9];     
}ec283_point_lambda_aff;

typedef struct
{
	uint32_t X[9];     
    uint32_t L[9];     
	uint32_t Z[9];  
}ec283_point_lambda_pro;

extern ec283_point_lambda_aff G;   
 
static void point_aff_negative(ec283_point_lambda_aff *P, ec283_point_lambda_aff *negP);
void project_to_affine(ec283_point_lambda_pro *Qp, ec283_point_lambda_aff *Qa);
void affine_to_project(ec283_point_lambda_aff *Qa, ec283_point_lambda_pro *Qp);
static void point_frobenius(ec283_point_lambda_pro *P);
static void consecutive_conjugate_aff(ec283_point_lambda_aff *P, ec283_point_lambda_pro *Q5, ec283_point_lambda_pro *Q7);
static void single_conjugate_pro(ec283_point_lambda_pro *P, ec283_point_lambda_pro *Q, uint32_t *alpha);
static void doulbe_conjugate_pro(ec283_point_lambda_pro *P, uint32_t *alpha, ec283_point_lambda_pro *Q);
static void power_conjugate_pro(ec283_point_lambda_pro *P, uint32_t *Z, ec283_point_lambda_pro *Q);
void mixed_addition(ec283_point_lambda_pro *P, ec283_point_lambda_aff *Q, ec283_point_lambda_pro *R);
void point_doubling(ec283_point_lambda_pro *P, ec283_point_lambda_pro *Q);
static void montgomery_trick(uint32_t a[7][9], uint32_t b[7][9]);
static void partial_mod(uint32_t *k, uint32_t *r0, uint32_t *r1);
static void TNAF5_expansion(uint32_t *r0, uint32_t *r1, int8_t *u);
void TNAF5_fixed_scalarmul(uint32_t *k, ec283_point_lambda_aff *Q);
void TNAF5_random_scalarmul(uint32_t *k, ec283_point_lambda_aff *P, ec283_point_lambda_aff *Q);

#ifdef __cplusplus
}
#endif

#endif /* SECT283K1_H */
