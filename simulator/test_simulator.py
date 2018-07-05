import os
import random
import time

import driver
import player
import solver

def test_e2e():
    opts = {'PLAYERS': [(10, 0), (0, 1), (0, 2)],
            'N_CONNECTIONS': 3,
            'TIME_TO_SIM': 2,
            'MEAN_PROP_TIME': 0.1,
            'STD_PROP_TIME': 0.001,
            'SEED': 20,
            'GRAPH': False
    }

    os.system('./run_server.sh &')
    time.sleep(5) # allow the server to boot up
    
    sol = driver.drive(opts)
    
    correctHashes = player.Player.correctHashes
    for i in sol.players:
        assert len(i.blockchain) > 0
        for j in range(len(i.blockchain)):
            assert i.blockchain[j] == correctHashes[j]
    



    
