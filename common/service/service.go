// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package service

// Service defines the lifecycle of a service
type Service interface {
	// Init initiates the service
	Init() error
	// Start starts the service
	Start() error
	// Stop stops the service
	Stop() error
}

// AbstractService implements an empty service
type AbstractService struct {
}

// Init does nothing
func (AbstractService) Init() error {
	return nil
}

// Start does nothing
func (AbstractService) Start() error {
	return nil
}

// Stop does nothing
func (AbstractService) Stop() error {
	return nil
}

// CompositeService is a service that contains multiple child services
type CompositeService struct {
	AbstractService
	Services []Service
}

// Init initiates all child services
func (s *CompositeService) Init() error {
	for _, ss := range s.Services {
		ss.Init()
	}
	return nil
}

// Start starts all child services
func (s *CompositeService) Start() error {
	for _, ss := range s.Services {
		ss.Start()
	}
	return nil
}

// Stop stops all child services
func (s *CompositeService) Stop() error {
	for _, ss := range s.Services {
		ss.Stop()
	}
	return nil
}

// AddService adds another service as the child service of this one
func (s *CompositeService) AddService(ss Service) {
	s.Services = append(s.Services, ss)
}
