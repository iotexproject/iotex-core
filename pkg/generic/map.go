package generic

import "sync"

// Map is a generic map that is thread safe
type Map[K comparable, V any] struct {
	maps map[K]V
	mu   sync.RWMutex
}

// NewMap creates a new map
func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{
		maps: make(map[K]V),
	}
}

// LoadOrStore loads the value or stores the value if it does not exist
func (m *Map[K, V]) LoadOrStore(key K, value V) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.maps[key]
	if ok {
		return v, ok
	}
	m.maps[key] = value
	return value, ok
}

// LoadAndDelete loads the value and deletes the key from the map
func (m *Map[K, V]) LoadAndDelete(key K) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.maps[key]
	if ok {
		delete(m.maps, key)
	}
	return v, ok
}

// Range iterates over the map
func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.maps {
		if !f(k, v) {
			break
		}
	}
}

// Store stores the value in the map
func (m *Map[K, V]) Store(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maps[key] = value
}

// Load returns the value stored in the map
func (m *Map[K, V]) Load(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.maps[key]
	return v, ok
}

// Total returns the total number of items in the map
func (m *Map[K, V]) Total() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.maps)
}

// Clear clears the map
func (m *Map[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maps = make(map[K]V)
}

// Delete deletes the key from the map
func (m *Map[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.maps, key)
}

// Clone creates a new map with the same content
func (m *Map[K, V]) Clone() *Map[K, V] {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := &Map[K, V]{
		maps: make(map[K]V),
	}
	for k, v := range m.maps {
		n.maps[k] = v
	}
	return n
}
