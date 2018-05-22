# Copyright (c) 2018 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.

"""This module defines a Block class, which represents a block on the blockchain.
"""

class Block:
    id = 0
    
    def __init__(self, txs, next=None, id=-1, proposer=None):
        """Creates a new Block object given a list of transactions txs"""

        if id == -1:
            self.id = Block.id
            Block.id += 1
        else:
            self.id = id
        
        self.next = next # next block in blockchain
        self.txs = txs   # list of transactions

        self.proposer   = proposer # proposer of the block

    def copy(self):
        new = Block(self.txs, self.next, self.id, self.proposer)
        
        return new

    def __eq__(self, other):
        """Equality for Block objects is defined as having the same transactions"""

        if other == None:
            return False
        
        selfIds = [i.id for i in self.txs]
        otherIds = [i.id for i in other.txs]

        return set(selfIds) == set(otherIds) and self.next == other.next

    def __hash__(self):
        """Hash"""

        return self.id

    def __str__(self):
        """Str representation: block n: transaction x_i; validators: p_i
                               str(block n-1)"""
        if self.next == None:
            return "block %s: "%(self.id)+", ".join([str(i) for i in self.txs])
        return "block %s: "%(self.id) + ", ".join([str(i) for i in self.txs]) + "\n" + str(self.next)

    def __repr__(self):
        return self.__str__()



        
