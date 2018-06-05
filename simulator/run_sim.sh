# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

#!/bin/sh

#PLAYERS="[(1, 0), (2, 1), (1, 2), (1, 4), (1, 8), (1, 16)]"
PLAYERS="[(5, 1)]"
N_CONNECTIONS=2
MEAN_PROP_TIME=0.2
STD_PROP_TIME=0.01
SEED=42
TIME_TO_SIM=5

python main.py --players="$PLAYERS" --nconnections=$N_CONNECTIONS --timetosim=$TIME_TO_SIM --meanproptime=$MEAN_PROP_TIME --stdproptime=$STD_PROP_TIME --seed=$SEED


