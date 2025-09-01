package itx

import (
	"net/http"
	"path"
	"strconv"
	"sync/atomic"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type (
	// Pauseable interface defines components that can be paused
	Pauseable interface {
		Pause(bool)
	}
	// PauseMgr manages pause operations for blockchain components
	PauseMgr struct {
		pauses        []Pauseable
		height        uint64
		pauseAtHeight uint64
	}
)

// NewPauseMgr creates a new pause manager
func NewPauseMgr(pauses ...Pauseable) *PauseMgr {
	return &PauseMgr{
		pauses: pauses,
	}
}

// Pause pauses or unpauses all pauseable components
func (pm *PauseMgr) Pause(pause bool) {
	for _, p := range pm.pauses {
		p.Pause(pause)
	}
}

// HandlePause handles the immediate pause request
func (pm *PauseMgr) HandlePause(w http.ResponseWriter, _ *http.Request) {
	pm.Pause(true)
	w.WriteHeader(http.StatusOK)
}

// HandleUnPause handles the unpause request
func (pm *PauseMgr) HandleUnPause(w http.ResponseWriter, _ *http.Request) {
	pm.Pause(false)
	w.WriteHeader(http.StatusOK)
}

// ReceiveBlock receives block notifications and checks for pause conditions
func (pm *PauseMgr) ReceiveBlock(block *block.Block) error {
	currentHeight := block.Height()
	atomic.StoreUint64(&pm.height, currentHeight)

	// Check if we should pause at the target height
	pauseAtHeight := atomic.LoadUint64(&pm.pauseAtHeight)
	if pauseAtHeight > 0 && currentHeight >= pauseAtHeight {
		pm.Pause(true)
		// Reset the pause height after pausing
		atomic.StoreUint64(&pm.pauseAtHeight, 0)
	}
	return nil
}

// HandlePauseAtHeight handles the pause at specific height request
func (pm *PauseMgr) HandlePauseAtHeight(w http.ResponseWriter, r *http.Request) {
	// Extract height from URL path like /pause/12345
	pathStr := r.URL.Path
	heightStr := path.Base(pathStr)

	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid height parameter", http.StatusBadRequest)
		return
	}

	currentHeight := atomic.LoadUint64(&pm.height)
	if height <= currentHeight {
		http.Error(w, "Height must be greater than current block height", http.StatusBadRequest)
		return
	}

	// Check if pauseAtHeight was previously set
	oldPauseAtHeight := atomic.LoadUint64(&pm.pauseAtHeight)

	// Set the target pause height
	atomic.StoreUint64(&pm.pauseAtHeight, height)

	// Return different status codes based on whether this is a new setting or an update
	if oldPauseAtHeight == 0 {
		// First time setting pause height - return 201 Created
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Pause scheduled at height " + heightStr))
	} else {
		// Updating existing pause height - return 200 OK
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Pause height updated to " + heightStr))
	}
}
