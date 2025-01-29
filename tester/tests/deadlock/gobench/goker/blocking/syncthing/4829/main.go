package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

func init() {
	fmt.Println("Starting run...")
}

type Address int

type Mapping struct {
	mut sync.RWMutex

	extAddresses map[string]Address
}

func (m *Mapping) clearAddresses() {
	m.mut.Lock() // First locking
	var removed []Address
	for id, addr := range m.extAddresses {
		removed = append(removed, addr)
		delete(m.extAddresses, id)
	}
	if len(removed) > 0 {
		m.notify(nil, removed)
	}
	m.mut.Unlock()
}

func (m *Mapping) notify(added, remove []Address) {
	m.mut.RLock()
	m.mut.RUnlock()
}

type Service struct {
	mut sync.RWMutex

	mappings []*Mapping
}

func (s *Service) NewMapping() *Mapping {
	mapping := &Mapping{
		extAddresses: make(map[string]Address),
	}
	s.mut.Lock()
	s.mappings = append(s.mappings, mapping)
	s.mut.Unlock()
	return mapping
}

func (s *Service) RemoveMapping(mapping *Mapping) {
	s.mut.Lock()
	defer s.mut.Unlock()
	for _, existing := range s.mappings {
		if existing == mapping {
			mapping.clearAddresses()
		}
	}
}

func NewService() *Service {
	return &Service{}
}

func main() {
	defer func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()

	for i := 0; i < 100; i++ {
		go func() {
			// deadlocks: x > 0
			natSvc := NewService()
			m := natSvc.NewMapping()
			m.extAddresses["test"] = 0

			natSvc.RemoveMapping(m)
		}()
	}
}
