# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

#!/bin/sh

# for instructions on how to run this program run `python main.py --help`

PLAYERS="[(10, 0), (0, 1), (0, 2)]"
N_CONNECTIONS=4
MEAN_PROP_TIME=0.1
STD_PROP_TIME=0.01
SEED=20
TIME_TO_SIM=3

# python main.py --players="$PLAYERS" --nconnections=$N_CONNECTIONS --timetosim=$TIME_TO_SIM --meanproptime=$MEAN_PROP_TIME --stdproptime=$STD_PROP_TIME --seed=$SEED
pyinstrument -o pyprofile.html --html main.py --players="$PLAYERS" --nconnections=$N_CONNECTIONS --timetosim=$TIME_TO_SIM --meanproptime=$MEAN_PROP_TIME --stdproptime=$STD_PROP_TIME --seed=$SEED
