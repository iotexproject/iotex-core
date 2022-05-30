// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package probe

import "net/http"

// WithReadinessHandler is an option to set a readiness handler for probe server.
func WithReadinessHandler(h http.Handler) interface{ Option } {
	return &readinessOption{h}
}

type readinessOption struct{ h http.Handler }

func (o *readinessOption) SetOption(s *Server) { s.readinessHandler = o.h }
