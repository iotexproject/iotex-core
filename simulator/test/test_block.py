# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

import os,sys,inspect
currentdir = os.path.dirname(os.path.abspath(inspect.getfile(inspect.currentframe())))
parentdir = os.path.dirname(currentdir)
sys.path.insert(0,parentdir)

import block
import player
import solver
import transaction

import random

opts = {"PLAYERS": [(100, 1)],        # list of tuples; [(number of players, stake per player)]
        "N_VALIDATORS": 5,            # number of validators in the system
        "N_PROPOSERS": 1,             # number of proposers in the system
        "N_CONNECTIONS": 8,           # number of connections per player
        "N_HEARTBEATS_IN_ROUND": 5,   # number of heartbeats (dt) in a round
        "N_ROUNDS": 5,                # number of rounds of proposal/validation/commit
        "N_TRANSACTIONS": 3,          # number of transactions per block
        "P_TRANSACTIONS": 0.1,        # probability of transaction per player per heartbeat
        "MEAN_PROP_TIME": 0.1         # mean propagation time of messages (exponential distribution)
}

def test_blockEquals():
    transaction.Transaction.id = 0
    block.Block.id = 0

    t = [transaction.Transaction(i, i+1, 0) for i in range(10)]

    a = block.Block(t[:3])
    b = block.Block([t[2], t[1], t[0]])
    c = block.Block(t[-3:])

    assert a == b
    assert a != c
    assert b != c

