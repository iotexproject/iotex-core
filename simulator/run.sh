#!/bin/sh

#PLAYERS="[(1, 0), (2, 1), (1, 2), (1, 4), (1, 8), (1, 16)]"
PLAYERS="[(3, 1)]"
N_VALIDATORS=4
N_PROPOSERS=1
N_CONNECTIONS=2
N_HEARTBEATS_IN_ROUND=3
P_TRANSACTIONS=0.1
MEAN_PROP_TIME=0.1
SEED=42
N_ROUNDS=2

python main.py --players="$PLAYERS" --nvalidators=$N_VALIDATORS --nproposers=$N_PROPOSERS --nconnections=$N_CONNECTIONS --nrounds=$N_ROUNDS --nheartbeatsinround=$N_HEARTBEATS_IN_ROUND --ptransactions=$P_TRANSACTIONS --meanproptime=$MEAN_PROP_TIME --seed=$SEED


