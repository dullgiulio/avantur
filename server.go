package main

import (
	"log"
)

type notif struct {
	env    string
	sha1   string
	branch string
}

func newNotif(env, sha1, branch string) *notif {
	return &notif{
		env:    env,
		sha1:   sha1,
		branch: branch,
	}
}

type server struct {
	notifs    chan *notif
	mergebots mergebots
}

func newServer() *server {
	return &server{
		notifs:    make(chan *notif),
		mergebots: makeMergebots(),
	}
}

func (s *server) serveBuilds(cf *config) {
	builds := makeBuilds(cf)

	for n := range s.notifs {
		log.Printf("[server] env %s: branch %s: handling notification for %s", n.env, n.branch, n.sha1)
		bs, err := newBuilds(n, cf)
		if err != nil {
			log.Printf("%s: %s", n.branch, err)
			continue
		}
		for _, b := range bs {
			bot := s.mergebots.get(n.env)
			if bot == nil {
				log.Printf("[server] no mergebot found for %s, skipping build push", n.env)
				continue
			}
			builds.push(b, n, bot)
		}
	}
}
