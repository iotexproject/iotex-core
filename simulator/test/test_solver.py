import os,sys,inspect
currentdir = os.path.dirname(os.path.abspath(inspect.getfile(inspect.currentframe())))
parentdir = os.path.dirname(currentdir)
sys.path.insert(0,parentdir)

import block
import player
import solver
import transaction
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

def test_init():
    solver.Solver.N_VALIDATORS = opts["N_VALIDATORS"]
    solver.Solver.N_PROPOSERS = opts["N_PROPOSERS"]
    solver.Solver.N_CONNECTIONS = opts["N_CONNECTIONS"]
    solver.Solver.N_HEARTBEATS_IN_ROUND = opts["N_HEARTBEATS_IN_ROUND"]

    player.Player.N_TRANSACTIONS = opts["N_TRANSACTIONS"]
    player.Player.P_TRANSACTIONS = opts["P_TRANSACTIONS"]
    player.Player.MEAN_PROP_TIME = opts["MEAN_PROP_TIME"]

def test_calcPercentStake():
    player.Player.id = 0
    
    opts["PLAYERS"] = [(1, i) for i in range(10)]
    s = solver.Solver(opts)

    assert s.calcPercentStake() == [0, 1/45, 2/45, 3/45, 4/45,
                                    5/45, 6/45, 7/45, 8/45, 9/45]
