// Copyright 2016 Giulio Iotti. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package umarell

import (
	"fmt"
	"regexp"
	"time"

	"github.com/dullgiulio/umarell/store"
)

type notifType int

const (
	notifPush notifType = iota
	notifDelete
)

type notif struct {
	project string
	sha1    string
	branch  string
	ntype   notifType
}

func newNotif(project, sha1, branch string, ntype notifType) *notif {
	return &notif{
		project: project,
		sha1:    sha1,
		branch:  branch,
		ntype:   ntype,
	}
}

func (n *notif) String() string {
	return fmt.Sprintf("%s: %s: %s", n.project, n.branch, n.sha1)
}

func (n *notif) equal(o *notif) bool {
	return n.project == o.project && n.branch == o.branch && n.sha1 == o.sha1
}

type server struct {
	notifs      chan *notif
	conf        *config
	regexBranch *regexp.Regexp
	// Limit the number of concurrent builds that can be performed
	limitBuilds chan struct{}
	storage     store.Store
	urls        *urls
	log         logger
	cleanup     chan struct{}
}

func NewServer(c *config) *server {
	s := &server{
		notifs: make(chan *notif),
		conf:   c,
		log:    newStdLogger(),
	}
	s.regexBranch = regexp.MustCompile(c.BranchRegexp)
	if c.LimitBuilds > 0 {
		s.limitBuilds = make(chan struct{}, c.LimitBuilds)
		for i := 0; i < c.LimitBuilds; i++ {
			s.limitBuilds <- struct{}{}
		}
	}
	var err error
	if c.Database != "" && c.Table != "" {
		if s.storage, err = store.NewMysql(c.Database, c.Table); err != nil {
			s.log.Printf("[error] cannot start database storage: %s", err)
		}
	}
	if s.storage == nil {
		s.log.Printf("[info] no database configured, using memory storage")
		s.storage = store.NewMemory()
	}
	s.urls = newUrls()
	if c.ResultsDuration > 0 && c.ResultsCleanup > 0 {
		s.cleanup = make(chan struct{})
		go s.cleaner(s.cleanup, time.Duration(c.ResultsCleanup))
	}
	return s
}

func (s *server) ServeReqs() {
	bots := makeMergebots()
	pros := newProjects(s, bots)

	for n := range s.notifs {
		s.handleNotif(n, bots, pros)
		if s.cleanup != nil {
			s.cleanup <- struct{}{}
		}
	}
}

func (s *server) cleaner(wakeup <-chan struct{}, duration time.Duration) {
	for range wakeup {
		before := time.Now().Add(-duration)
		s.log.Printf("[server] results cleaner: cleaning jobs before %s", before.Format("2006-02-01 15:04:05"))
		if err := s.storage.Clean(before); err != nil {
			s.log.Printf("[error] results cleaner: %s", err)
		}
	}
}

func (s *server) startBuild() {
	if s.limitBuilds != nil {
		<-s.limitBuilds
	}
}

func (s *server) stopBuild() {
	if s.limitBuilds != nil {
		s.limitBuilds <- struct{}{}
	}
}

func (s *server) handleNotif(n *notif, bots mergebots, pros *projects) {
	s.log.Printf("[server] %s: handling notification", n)
	bs, err := newBuilds(n, s)
	if err != nil {
		s.log.Printf("[server] %s: no builds created: %s", n, err)
		return
	}
	bot := bots.get(n.project)
	if bot == nil {
		s.log.Printf("[server] no mergebot found for %s, skipping build push", n.project)
		return
	}
	for _, b := range bs {
		switch n.ntype {
		case notifPush:
			pros.push(b, n, bot)
		case notifDelete:
			pros.destroy(b, n, -1)
			bot.destroy(b.stage)
		}
	}
}
