package umarell

import (
	"fmt"
	"log"
)

type buildver struct {
	// sha1 records the previous version compared to build.sha1.
	sha1  string
	build *build
}

type mergereq struct {
	notif *notif
	build *build
}

func newMergereq(notif *notif, build *build) *mergereq {
	return &mergereq{
		notif: notif,
		build: build,
	}
}

type checkout struct {
	stage string
	dir   string
	ver   buildver
}

func newCheckout(stage, dir string, ver buildver) *checkout {
	return &checkout{
		dir:   dir,
		stage: stage,
		ver:   ver,
	}
}

type mergebot struct {
	project   string
	checkouts map[string]*checkout // stage : checkout
	vers      map[string]*buildver // stage : version
	reqs      chan *mergereq
	srv       *server
}

func newMergebot(project string, s *server) *mergebot {
	b := &mergebot{
		project:   project,
		srv:       s,
		checkouts: make(map[string]*checkout),
		vers:      make(map[string]*buildver),
		reqs:      make(chan *mergereq),
	}
	return b
}

func (b *mergebot) addCheckout(dir string, notif *notif, build *build) {
	bv := buildver{
		sha1:  notif.sha1,
		build: build,
	}
	b.checkouts[build.stage] = newCheckout(build.stage, dir, bv)
	log.Printf("[mergebot] %s: init %s to %s using stage %s", b.project, notif.branch, notif.sha1, build.stage)
}

func (b *mergebot) registerBuild(req *mergereq) {
	bv := b.vers[req.build.stage]
	if bv == nil {
		bv = &buildver{
			build: req.build,
		}
	}
	bv.sha1 = req.notif.sha1
	b.vers[req.build.stage] = bv
	log.Printf("[mergebot] %s: set latest revision to %s stage %s", b.project, req.notif.sha1, req.build.stage)
}

func (b *mergebot) checkMerged(notif *notif, co *checkout, pjs *projects) error {
	ver := co.ver
	// Do not attempt to merge a checked out stage.
	if _, ok := b.checkouts[ver.build.stage]; ok {
		log.Printf("[mergebot] %s: merge to %s ignored", b.project, ver.build.stage)
		return nil
	}
	log.Printf("[mergebot] %s: checking that %s from %s has been merged to %s", b.project, ver.sha1, ver.build.stage, co.stage)
	commits := newGitcommits()
	if ver.sha1 == "" {
		return fmt.Errorf("cannot fetch commits since last build, last SHA1 is empty", b.project)
	}
	if err := commits.since(ver.sha1, co.dir); err != nil {
		return fmt.Errorf("%s: can't fetch commits since %s: %s", co.dir, ver.sha1, err)
	}
	merged := make([]string, 0) // merged stages to remove
	for k, bv := range b.vers {
		if commits.contains(githash(bv.sha1)) {
			log.Printf("[mergebot] %s: can remove env %s, it was merged", b.project, bv.build.stage)
			b.srv.urls.del(bv.build.stage)
			// As we have been called by pjs, to make a request we need to wait for the current one to finish.
			// To avoid a deadlock, we must notify of the merge in the background.
			go pjs.merge(bv.build, notif)
			merged = append(merged, k)
		}
	}
	for _, k := range merged {
		delete(b.vers, k)
	}
	return nil
}

func (b *mergebot) send(req *mergereq) {
	b.reqs <- req
}

func (b *mergebot) run(pjs *projects) {
	for req := range b.reqs {
		b.doReq(req, pjs)
	}
}

func (b *mergebot) doReq(req *mergereq, pjs *projects) {
	co, hasCheckout := b.checkouts[req.build.stage]
	if !hasCheckout {
		// normally update some tracked version
		b.registerBuild(req)
		return
	}
	// It's a push to a checked out stage, trigger the delete etc
	if err := b.checkMerged(req.notif, co, pjs); err != nil {
		log.Printf("[mergebot] %s: failed merge check: %s", b.project, err)
	}
	co.ver.sha1 = req.notif.sha1
	log.Printf("[mergebot] %s: merge check done, set latest revision to %s stage %s", b.project, req.notif.sha1, co.ver.build.stage)
}

type mergebots map[string]*mergebot // project : mergebots

func makeMergebots() mergebots {
	return mergebots(make(map[string]*mergebot))
}

func (m mergebots) get(project string) *mergebot {
	return m[project]
}

func (m mergebots) create(project string, s *server) *mergebot {
	b := newMergebot(project, s)
	m[project] = b
	return b
}
