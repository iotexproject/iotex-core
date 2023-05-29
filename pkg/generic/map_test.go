package generic

import (
	"net/http"
	"testing"
)

func TestMap(t *testing.T) {
	tests := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
		"d": 4,
		"e": 5,
	}
	m := NewMap[string, int]()
	for k, v := range tests {
		m.Store(k, v)
	}
	m.Range(func(key string, value int) bool {
		v, ok := tests[key]
		if !ok {
			t.Fatal("keys do not match")
		}
		if value != v {
			t.Fatal("values do not match")
		}
		return true
	})
	if m.Total() != len(tests) {
		t.Fatal("total does not match")
	}

	m2 := m.Clone()
	m.Clear()

	if m.Total() != 0 {
		t.Fatal("total does not match")
	}

	m2.Range(func(key string, value int) bool {
		v, ok := tests[key]
		if !ok {
			t.Fatal("keys do not match")
		}
		if value != v {
			t.Fatal("values do not match")
		}
		return true
	})

}

func TestIntMap(t *testing.T) {
	m := NewMap[int, int]()
	m.Store(1, 2)
	_, ok := m.Load(1)
	if !ok {
		t.Fatal("value should be existed")
	}
	m.Delete(1)
	_, ok = m.Load(1)
	if ok {
		t.Fatal("value should not be existed")
	}
	r, loaded := m.LoadOrStore(1, 2)
	if loaded {
		t.Fatal("value should not be loaded")
	}
	lr, loaded := m.LoadOrStore(1, r)
	if !loaded {
		t.Fatal("value should not be loaded")
	}
	if lr != r {
		t.Fatal("loaded value should be the same")
	}
	s, _ := m.LoadOrStore(2, 3)
	kv := map[int]int{1: r, 2: s}
	m.Range(func(key, value int) bool {
		v, ok := kv[key]
		if !ok {
			t.Fatal("keys do not match")
		}
		if value != v {
			t.Fatal("values do not match")
		}
		delete(kv, key)
		return true
	})
}

func TestRequests(t *testing.T) {
	m := NewMap[string, *http.Request]()
	m.Store("r", &http.Request{})
	_, ok := m.Load("r")
	if !ok {
		t.Fatal("value should be existed")
	}
	v, ok := m.LoadAndDelete("r")
	if !ok || v == nil {
		t.Fatal("value should be existed")
	}
	_, ok = m.Load("r")
	if ok {
		t.Fatal("value should not be existed")
	}
	r, loaded := m.LoadOrStore("r", &http.Request{})
	if loaded {
		t.Fatal("value should not be loaded")
	}
	lr, loaded := m.LoadOrStore("r", r)
	if !loaded {
		t.Fatal("value should not be loaded")
	}
	if lr != r {
		t.Fatal("loaded value should be the same")
	}
	s, _ := m.LoadOrStore("s", &http.Request{})
	kv := map[string]*http.Request{"r": r, "s": s}
	m.Range(func(key string, value *http.Request) bool {
		v, ok := kv[key]
		if !ok {
			t.Fatal("keys do not match")
		}
		if value != v {
			t.Fatal("values do not match")
		}
		delete(kv, key)
		return true
	})
}
