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
	conf   *config
}

func newServer(conf *config) *server {
	return &server{
		notifs: make(chan *notif),
		conf:   conf,
	}
}

func (s *server) serveReqs(cf *config) {
	bots := makeMergebots()
	pros := newProjects(cf, bots)

	for n := range s.notifs {
		s.handleNotif(cf, n, bots, pros)
	}
}

func (s *server) handleNotif(cf *config, n *notif, bots mergebots, pros *projects) {
	log.Printf("[server] %s: handling notification", n)
	bs, err := newBuilds(n, cf)
	if err != nil {
		log.Printf("[server] %s: no builds created: %s", n, err)
		return
	}
	bot := bots.get(n.project)
	if bot == nil {
		log.Printf("[server] no mergebot found for %s, skipping build push", n.project)
		return
	}
	for _, b := range bs {
		pros.push(b, n, bot)
	}
}
