// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

func (cb *candBase) clone() candMap {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	clone := newCandBase()
	for name, cand := range cb.nameMap {
		clone.nameMap[name] = cand.Clone()
	}
	for owner, cand := range cb.ownerMap {
		clone.ownerMap[owner] = cand.Clone()
	}
	for operator, cand := range cb.operatorMap {
		clone.operatorMap[operator] = cand.Clone()
	}
	for bucket, cand := range cb.selfStkBucketMap {
		clone.selfStkBucketMap[bucket] = cand.Clone()
	}
	if len(cb.owners) > 0 {
		for _, cand := range cb.owners {
			clone.owners = append(clone.owners, cand.Clone())
		}
	}
	return clone
}

func (cb *candBase) candsInNameMap() CandidateList {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	if len(cb.nameMap) == 0 {
		return nil
	}

	list := make(CandidateList, 0, len(cb.nameMap))
	for _, d := range cb.nameMap {
		list = append(list, d.Clone())
	}
	return list
}

func (cb *candBase) candsInOperatorMap() CandidateList {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	if len(cb.operatorMap) == 0 {
		return nil
	}

	list := make(CandidateList, 0, len(cb.operatorMap))
	for _, d := range cb.operatorMap {
		list = append(list, d.Clone())
	}
	return list
}

func (cb *candBase) ownersList() CandidateList {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	return cb.owners
}

func (cb *candBase) recordOwner(c *Candidate) {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	for i, d := range cb.owners {
		if d.Owner.String() == c.Owner.String() {
			cb.owners[i] = c.Clone()
			return
		}
	}
	// this is a new candidate
	cb.owners = append(cb.owners, c.Clone())
}

func (cb *candBase) loadNameOperatorMapOwnerList(name, op, owners CandidateList) error {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	cb.nameMap = make(map[string]*Candidate)
	for _, d := range name {
		if err := d.Validate(); err != nil {
			return err
		}
		cb.nameMap[d.Name] = d
	}
	cb.operatorMap = make(map[string]*Candidate)
	for _, d := range op {
		if err := d.Validate(); err != nil {
			return err
		}
		cb.operatorMap[d.Operator.String()] = d
	}
	cb.owners = owners
	return nil
}
