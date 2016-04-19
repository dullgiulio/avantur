package main

import (
	"fmt"
	"log"
)

type buildver struct {
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
	conf      *config
	project   string
	checkouts map[string]*checkout // stage : checkout
	vers      map[string]*buildver // stage : version
	reqs      chan *mergereq
}

func newMergebot(project string, cf *config) *mergebot {
	b := &mergebot{
		project:   project,
		conf:      cf,
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

func (b *mergebot) checkMerged(req *mergereq, co *checkout, pjs *projects) error {
	ver := co.ver
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
			// As we have been called by pjs, to make a request we need to wait for the current one to finish.
			// To avoid a deadlock, we must notify of the merge in the background.
			go pjs.merge(bv.build, req.notif)
			merged = append(merged, k)
		}
	}
	for _, k := range merged {
		delete(b.vers, k)
	}
	log.Printf("[mergebot] %s: merge check done, set latest revision to %s stage %s", b.project, req.notif.sha1, ver.build.stage)
	ver.sha1 = req.notif.sha1
	return nil
}

func (b *mergebot) send(req *mergereq) {
	b.reqs <- req
}

func (b *mergebot) run(pjs *projects) {
	for req := range b.reqs {
		co, hasCheckout := b.checkouts[req.build.stage]
		if !hasCheckout {
			// normally update some tracked version
			b.registerBuild(req)
			continue
		}
		// It's a push to a checked out stage, trigger the delete etc
		if err := b.checkMerged(req, co, pjs); err != nil {
			log.Printf("[mergebot] %s: %s", b.project, err)
		}
	}
}

type mergebots map[string]*mergebot // project : mergebots

func makeMergebots() mergebots {
	return mergebots(make(map[string]*mergebot))
}

func (m mergebots) get(project string) *mergebot {
	return m[project]
}

func (m mergebots) create(project string, cf *config) *mergebot {
	b := newMergebot(project, cf)
	m[project] = b
	return b
}
