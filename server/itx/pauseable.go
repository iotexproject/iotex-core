package itx

import (
	"net/http"
)

type (
	Pauseable interface {
		Pause(bool)
	}
	PauseMgr struct {
		pauses []Pauseable
	}
)

func NewPauseMgr(pauses ...Pauseable) *PauseMgr {
	return &PauseMgr{
		pauses: pauses,
	}
}

func (pm *PauseMgr) Pause(pause bool) {
	for _, p := range pm.pauses {
		p.Pause(pause)
	}
}

func (pm *PauseMgr) HandlePause(w http.ResponseWriter, r *http.Request) {
	pm.Pause(true)
	w.WriteHeader(http.StatusOK)
}

func (pm *PauseMgr) HandleUnPause(w http.ResponseWriter, r *http.Request) {
	pm.Pause(false)
	w.WriteHeader(http.StatusOK)
}
