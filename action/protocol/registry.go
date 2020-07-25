// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"reflect"
	"sync"

	"github.com/pkg/errors"
)

// Registry is the hub of all protocols deployed on the chain
type Registry struct {
	mu        sync.RWMutex
	ids       map[string]int
	protocols []Protocol
}

// NewRegistry create a new Registry
func NewRegistry() *Registry {
	return &Registry{
		ids:       make(map[string]int, 0),
		protocols: make([]Protocol, 0),
	}
}

func (r *Registry) register(id string, p Protocol, force bool) error {
	idx, loaded := r.ids[id]
	if loaded {
		if !force {
			return errors.Errorf("Protocol with ID %s is already registered", id)
		}
		r.protocols[idx] = p

		return nil
	}
	r.ids[id] = len(r.ids)
	r.protocols = append(r.protocols, p)

	return nil
}

// Register registers the protocol with a unique ID
func (r *Registry) Register(id string, p Protocol) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.register(id, p, false)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (r *Registry) ForceRegister(id string, p Protocol) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.register(id, p, true)
}

// Find finds a protocol by ID
func (r *Registry) Find(id string) (Protocol, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	idx, loaded := r.ids[id]
	if !loaded {
		return nil, false
	}

	return r.protocols[idx], true
}

// All returns all protocols
func (r *Registry) All() []Protocol {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.all()
}

func (r *Registry) all() []Protocol {
	all := make([]Protocol, len(r.protocols))
	copy(all, r.protocols)

	return all
}

// StartAll starts all protocols which are startable
func (r *Registry) StartAll(ctx context.Context, sr StateReader) (View, error) {
	if r == nil {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	allView := make(View)
	for _, p := range r.all() {
		s, ok := p.(Starter)
		if !ok {
			continue
		}
		view, err := s.Start(ctx, sr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to start protocol %s", reflect.TypeOf(p))
		}
		if view == nil {
			continue
		}
		allView[p.Name()] = view
	}
	return allView, nil
}
