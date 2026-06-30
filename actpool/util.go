package actpool

// IsBlackListedFunc creates a function that checks if an address is blacklisted at a given height.
// Addresses in removal are still treated as blacklisted in [activeHeight, removalHeight); from
// removalHeight onwards they are no longer considered blacklisted.
func IsBlackListedFunc(blackList []string, activeHeight uint64, removal []string, removalHeight uint64) func(addr string, height uint64) bool {
	bl := make(map[string]bool, len(blackList))
	for _, addr := range blackList {
		bl[addr] = true
	}
	rm := make(map[string]bool, len(removal))
	for _, addr := range removal {
		rm[addr] = true
	}

	return func(addr string, height uint64) bool {
		if len(bl) == 0 {
			return false
		}
		if _, ok := bl[addr]; !ok {
			return false
		}
		if activeHeight != 0 && height < activeHeight {
			return false
		}
		if rm[addr] && height >= removalHeight {
			return false
		}
		return true
	}
}
