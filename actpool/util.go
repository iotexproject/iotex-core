package actpool

// IsBlackListedFunc creates a function that checks if an address is blacklisted at a given height
func IsBlackListedFunc(blackList []string, activeHeight uint64) func(addr string, height uint64) bool {
	// Create blacklist map
	bl := make(map[string]bool, len(blackList))
	for _, addr := range blackList {
		bl[addr] = true
	}

	return func(addr string, height uint64) bool {
		if bl == nil || len(bl) == 0 {
			return false
		}
		if _, ok := bl[addr]; !ok {
			return false
		}
		if activeHeight == 0 {
			return true
		}
		return height >= activeHeight
	}
}
