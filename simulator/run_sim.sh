# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

#!/bin/sh

#PLAYERS="[(1, 0), (2, 1), (1, 2), (1, 4), (1, 8), (1, 16)]"
PLAYERS="[(3, 1)]"
N_VALIDATORS=4
N_PROPOSERS=1
N_CONNECTIONS=2
N_HEARTBEATS_IN_ROUND=5
P_TRANSACTIONS=0.1
MEAN_PROP_TIME=0.1
SEED=42
N_ROUNDS=10

python main.py --players="$PLAYERS" --nvalidators=$N_VALIDATORS --nproposers=$N_PROPOSERS --nconnections=$N_CONNECTIONS --nrounds=$N_ROUNDS --nheartbeatsinround=$N_HEARTBEATS_IN_ROUND --ptransactions=$P_TRANSACTIONS --meanproptime=$MEAN_PROP_TIME --seed=$SEED


