package store

import (
	"errors"
	"sync"
)

var errNotFound = errors.New("not found")

type Memory struct {
	mux  sync.RWMutex
	data map[string]map[int64][]*BuildResult // env, ticket, buildResults
}

func NewMemory() *Memory {
	return &Memory{
		data: make(map[string]map[int64][]*BuildResult),
	}
}

func (b *Memory) Add(env string, ticket int64, br *BuildResult) error {
	b.mux.Lock()
	defer b.mux.Unlock()

	if _, ok := b.data[env]; !ok {
		b.data[env] = make(map[int64][]*BuildResult)
	}
	if b.data[env][ticket] == nil {
		b.data[env][ticket] = make([]*BuildResult, 0)
	}
	b.data[env][ticket] = append(b.data[env][ticket], br)
	return nil
}

func (b *Memory) Get(env string, ticket int64) ([]*BuildResult, error) {
	b.mux.RLock()
	defer b.mux.RUnlock()

	if _, ok := b.data[env]; !ok {
		return nil, errNotFound
	}
	return b.data[env][ticket], nil
}

func (b *Memory) DeleteTicket(env string, ticket int64) error {
	b.mux.Lock()
	defer b.mux.Unlock()

	delete(b.data[env], ticket)
	return nil
}

func (b *Memory) DeleteEnv(env string) error {
	b.mux.Lock()
	defer b.mux.Unlock()

	delete(b.data, env)
	return nil
}
