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
	notifs chan *notif
}

func newServer() *server {
	return &server{
		notifs: make(chan *notif),
	}
}

func (s *server) serveBuilds(cf *config) {
	builds := makeBuilds(cf)

	for n := range s.notifs {
		log.Printf("[server] env %s: branch %s: handling notification for %s", n.env, n.branch, n.sha1)
		b, err := newBuild(n.env, n.branch, n.sha1, cf)
		if err != nil {
			log.Printf("%s: %s", n.branch, err)
			continue
		}
		builds.push(b)
	}
}
