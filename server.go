package main

import (
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

type server struct {
	notifs    chan *notif
	mergebots mergebots
}

func newServer(cf *config) *server {
	s := &server{
		notifs:    make(chan *notif),
		mergebots: makeMergebots(),
	}
	for project := range cf.Envs {
		s.mergebots.add(project, cf)
	}
	return s
}

func (s *server) serveBuilds(cf *config) {
	projects := newProjects(cf, s.mergebots)
	for project := range cf.Envs {
		bot := s.mergebots.get(project)
		go bot.run(projects)
	}

	for n := range s.notifs {
		log.Printf("[server] project %s: branch %s: handling notification for %s", n.project, n.branch, n.sha1)
		bs, err := newBuilds(n, cf)
		if err != nil {
			log.Printf("%s: %s", n.branch, err)
			continue
		}
		for _, b := range bs {
			bot := s.mergebots.get(n.project)
			if bot == nil {
				log.Printf("[server] no mergebot found for %s, skipping build push", n.project)
				continue
			}
			projects.push(b, n, bot)
		}
	}
}
