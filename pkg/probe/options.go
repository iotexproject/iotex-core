package probe

import "net/http"

// WithReadinessHandler is an option to set a readiness handler for probe server.
func WithReadinessHandler(h http.Handler) interface{ Option } {
	return &readinessOption{h}
}

type readinessOption struct{ h http.Handler }

func (o *readinessOption) SetOption(s *Server) { s.readinessHandler = o.h }
