package main

import (
	"sync"
)

type urls struct {
	entries map[string]string // stage : url
	mux     sync.RWMutex
}

func newUrls() *urls {
	return &urls{
		entries: make(map[string]string),
	}
}

func (u *urls) get() []string {
	u.mux.RLock()
	defer u.mux.RUnlock()

	list := make([]string, len(u.entries))
	i := 0
	for _, v := range u.entries {
		list[i] = v
		i++
	}
	return list
}

func (u *urls) set(k, v string) {
	u.mux.Lock()
	defer u.mux.Unlock()

	u.entries[k] = v
}

func (u *urls) del(k string) {
	u.mux.Lock()
	defer u.mux.Unlock()

	delete(u.entries, k)
}
