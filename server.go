package main

import (
	"fmt"
	"log"
)

type notif struct {
	project string
	sha1    string
	branch  string
}

func newNotif(project, sha1, branch string) *notif {
	return &notif{
		project: project,
		sha1:    sha1,
		branch:  branch,
	}
}

func (n *notif) String() string {
	return fmt.Sprintf("%s: %s: %s", n.project, n.branch, n.sha1)
}

type server struct {
	notifs chan *notif
}

func newServer(cf *config) *server {
	return &server{
		notifs: make(chan *notif),
	}
}

func (s *server) serveBuilds(cf *config) {
	mergebots := makeMergebots()
	projects := newProjects(cf, mergebots)

	for n := range s.notifs {
		log.Printf("[server] %s: handling notification", n)
		bs, err := newBuilds(n, cf)
		if err != nil {
			log.Printf("[server] %s: no builds created: %s", n, err)
			continue
		}
		bot := mergebots.get(n.project)
		if bot == nil {
			log.Printf("[server] no mergebot found for %s, skipping build push", n.project)
			continue
		}
		for _, b := range bs {
			projects.push(b, n, bot)
		}
	}
}
