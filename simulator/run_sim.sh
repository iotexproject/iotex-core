# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

#!/bin/sh

#PLAYERS="[(1, 0), (2, 1), (1, 2), (1, 4), (1, 8), (1, 16)]"
PLAYERS="[(10, 1)]"
N_CONNECTIONS=3
MEAN_PROP_TIME=0.1
SEED=42
N_ROUNDS=8

python main.py --players="$PLAYERS" --nconnections=$N_CONNECTIONS --nrounds=$N_ROUNDS --meanproptime=$MEAN_PROP_TIME --seed=$SEED


