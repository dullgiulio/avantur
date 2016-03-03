package main

import (
	"sync"
)

type buildRepository struct {
	mux  sync.RWMutex
	data map[string]map[int64][]*buildResult // env, ticket, buildResults
}

func newBuildRepository() *buildRepository {
	return &buildRepository{
		data: make(map[string]map[int64][]*buildResult),
	}
}

func (b *buildRepository) add(env string, ticket int64, br *buildResult) {
	b.mux.Lock()
	defer b.mux.Unlock()

	if _, ok := b.data[env]; !ok {
		b.data[env] = make(map[int64][]*buildResult)
	}
	if b.data[env][ticket] == nil {
		b.data[env][ticket] = make([]*buildResult, 0)
	}
	b.data[env][ticket] = append(b.data[env][ticket], br)
}

func (b *buildRepository) get(env string, ticket int64) []*buildResult {
	b.mux.RLock()
	defer b.mux.RUnlock()

	if _, ok := b.data[env]; !ok {
		return nil
	}
	return b.data[env][ticket]
}
