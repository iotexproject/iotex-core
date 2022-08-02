package types

import (
	abci "github.com/iotexproject/iotex-sdk/abci/types"
)

// ----------------------------------------------------------------------------
// Event Manager
// ----------------------------------------------------------------------------

// EventManager implements a simple wrapper around a slice of Event objects that
// can be emitted from.
type EventManager struct {
	events Events
}

// ----------------------------------------------------------------------------
// Events
// ----------------------------------------------------------------------------

type (
	// Event is a type alias for an ABCI Event
	Event abci.Event

	// Attribute defines an attribute wrapper where the key and value are
	// strings instead of raw bytes.
	Attribute struct {
		Key   string `json:"key"`
		Value string `json:"value,omitempty"`
	}

	// Events defines a slice of Event objects
	Events []Event
)

// EmptyEvents returns an empty slice of events.
func EmptyEvents() Events {
	return make(Events, 0)
}

// ToABCIEvents converts a slice of Event objects to a slice of abci.Event
// objects.
func (e Events) ToABCIEvents() []abci.Event {
	res := make([]abci.Event, len(e))
	for i, ev := range e {
		res[i] = abci.Event{Type: ev.Type, Attributes: ev.Attributes}
	}

	return res
}
