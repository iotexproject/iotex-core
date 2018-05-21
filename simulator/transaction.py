"""This module defines the Transaction class, which represents a transaction made between two players.
"""

class Transaction:
    id = 0
    
    def __init__(self, senderId, recipId, fee):
        """Creates a new Transaction object, given senderId and recipId"""
        
        self.senderId = senderId # sender id
        self.recipId  = recipId  # recipient id
        self.fee      = fee      # transaction fee
        
        self.id = Transaction.id
        Transaction.id += 1

    def __eq__(self, other):
        if other == None:
            return False
        
        return self.id == other.id

    def __str__(self):
        return "transaction %s, fee %s" % (self.id, round(self.fee, 2))

    def __repr__(self):
        return self.__str__()

    def __hash__(self):
        return self.id
