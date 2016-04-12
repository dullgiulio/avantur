package store

import (
	"errors"
	"sync"
	"time"
)

var errNotFound = errors.New("not found")

type Memory struct {
	mux  sync.RWMutex
	data map[string][]*BuildResult // stage : buildResults
}

func NewMemory() *Memory {
	return &Memory{
		data: make(map[string][]*BuildResult),
	}
}

func (b *Memory) Add(br *BuildResult) error {
	b.mux.Lock()
	defer b.mux.Unlock()

	if _, ok := b.data[br.Stage]; !ok {
		b.data[br.Stage] = make([]*BuildResult, 0)
	}
	b.data[br.Stage] = append(b.data[br.Stage], br)
	return nil
}

func (b *Memory) Get(stage string) ([]*BuildResult, error) {
	b.mux.RLock()
	defer b.mux.RUnlock()

	if _, ok := b.data[stage]; !ok {
		return nil, errNotFound
	}
	return b.data[stage], nil
}

func (b *Memory) Delete(stage string) error {
	b.mux.Lock()
	defer b.mux.Unlock()

	delete(b.data, stage)
	return nil
}

func (b *Memory) Clean(until time.Time) error {
	b.mux.Lock()
	defer b.mux.Unlock()

	for key := range b.data {
		for i := 0; i < len(b.data[key]); i++ {
			if b.data[key][i].End.Before(until) {
				b.data[key] = append(b.data[key][:i], b.data[key][i+1:]...)
				i--
			}
		}
	}

	return nil
}
