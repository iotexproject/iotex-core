// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

#ifndef __MNT160TWIST_H__
#define __MNT160TWIST_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include "fp3.h"

typedef struct
{
    Fp3Elm x;     
    Fp3Elm y;     
}ec_point_aff_twist;

typedef struct
{
    Fp3Elm X;     
    Fp3Elm Y;
    Fp3Elm Z;     
}ec_point_pro_twist;

extern Fp3Elm mnt160twist_a;
extern Fp3Elm mnt160twist_b;
extern ec_point_aff_twist PREGTWIST[15];
extern ec_point_aff_twist PRE2EGTWIST[15];

void project_to_affine_twist(ec_point_pro_twist *Qp, ec_point_aff_twist *Qa);
void mixed_addition_twist(ec_point_pro_twist *P, ec_point_aff_twist *Q, ec_point_pro_twist *R);
void phi(ec_point_aff_twist *Q, Fp3Elm *xr, Fp3Elm *yi);
void fixed_comb_scalarmul_twist(uint32_t *k, ec_point_aff_twist *Q);
void NAF5_random_scalarmul_twist(uint32_t *k, ec_point_aff_twist *P, ec_point_aff_twist *Q);

#ifdef __cplusplus
}
#endif

#endif /* MNT160TWIST_H */
