// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

#ifndef __MNT160_H__
#define __MNT160_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include "fp6.h"
#include "mnt160twist.h"

typedef enum
{
    curve_only,
    pairing
}ec_op;

typedef struct
{
    uint32_t x[5];     
    uint32_t y[5];     
}ec160_point_aff;

typedef struct
{
	uint32_t X[5];     
    uint32_t Y[5];     
	uint32_t Z[5];     
}ec160_point_pro;

extern uint32_t mnt160_a[5];
extern uint32_t mnt160_b[5];
extern int8_t multibase_mnt160_n[142][2];
extern ec160_point_aff PREG[15];
extern ec160_point_aff PRE2EG[15];

void point_compression_mnt(ec160_point_aff *P, uint32_t *Pc);
uint32_t point_decompression_mnt(uint32_t *Pc, ec160_point_aff *P);
void point_negation_mnt(ec160_point_aff *P, ec160_point_aff *negP);
void project_to_affine_mnt(ec160_point_pro *Qp, ec160_point_aff *Qa);
void affine_to_project_mnt(ec160_point_aff *Qa, ec160_point_pro *Qp);
void mixed_addition_mnt(ec160_point_pro *T, ec160_point_aff *P, ec160_point_pro *R, uint32_t l[5][5], ec_op op);
void mixed_addition_line_eval(uint32_t l[5][5], Fp3Elm *xQ, Fp3Elm *yQ, Fp6Elm *ladd);
void point_doubling_mnt(ec160_point_pro *T, ec160_point_pro *R, uint32_t l[5][5], ec_op op);
void point_doubling_line_eval(uint32_t l[5][5], Fp3Elm *xQ, Fp3Elm *yQ, Fp6Elm *ldbl);
void point_tripling_mnt(ec160_point_pro *T, ec160_point_pro *R, uint32_t l[8][5], ec_op op);
void point_tripling_line_eval(uint32_t l[8][5], Fp3Elm *xQ, Fp3Elm *yQ, Fp6Elm *ltrl);
void multibase_expansion(uint32_t *k, int8_t kb[158][2]);
void multibase_scalarmul(uint32_t *k, ec160_point_aff *P, ec160_point_aff *Q);
void fixed_comb_scalarmul(uint32_t *k, ec160_point_aff *Q);
Fp6Elm pairing_multibase_miller(ec160_point_aff *P, ec_point_aff_twist *Q);
void hash_to_G1_swu(const uint8_t *msg, uint64_t mlen, ec160_point_aff *P);

#ifdef __cplusplus
}
#endif

#endif /* MNT160_H */
