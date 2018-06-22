# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""This module defines a failure stop consensus client"""

import consensus_client

class ConsensusFS(consensus_client.Consensus):
    def __init__(self):
        super()

    def processMessage(self, msg):
        return list()
