// Copyright 2016 Giulio Iotti. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
	token int64
}

func newMergereq(notif *notif, token int64, build *build) *mergereq {
	return &mergereq{
		notif: notif,
		build: build,
		token: token,
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
	dels      chan string // stage
	srv       *server
}

func newMergebot(project string, s *server) *mergebot {
	b := &mergebot{
		project:   project,
		srv:       s,
		checkouts: make(map[string]*checkout),
		vers:      make(map[string]*buildver),
		reqs:      make(chan *mergereq),
		dels:      make(chan string),
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

func (b *mergebot) checkMerged(notif *notif, token int64, co *checkout, pjs *projects) error {
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
		// Do not attempt to remove a checked-out stage.
		if _, ok := b.checkouts[bv.build.stage]; ok {
			log.Printf("[mergebot] %s: merge to %s ignored", b.project, bv.build.stage)
			continue
		}
		if commits.contains(githash(bv.sha1)) {
			log.Printf("[mergebot] %s: can remove env %s, it was merged", b.project, bv.build.stage)
			b.srv.urls.del(bv.build.stage)
			// As we have been called by pjs, to make a request we need to wait for the current one to finish.
			// To avoid a deadlock, we must notify of the merge in the background.
			go pjs.destroy(bv.build, notif, token)
			merged = append(merged, k)
		}
	}
	for _, k := range merged {
		delete(b.vers, k)
	}
	return nil
}

func (b *mergebot) destroy(stage string) {
	b.dels <- stage
}

func (b *mergebot) send(req *mergereq) {
	b.reqs <- req
}

func (b *mergebot) run(pjs *projects) {
	for {
		select {
		case req := <-b.reqs:
			b.doReq(req, pjs)
		case stage := <-b.dels:
			delete(b.vers, stage)
		}
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
	if err := b.checkMerged(req.notif, req.token, co, pjs); err != nil {
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
