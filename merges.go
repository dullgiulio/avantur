package main

import (
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

type mergebot struct {
	conf    *config
	project string
	master  struct {
		stage string
		dir   string
		ver   *buildver
	}
	vers map[string]*buildver // stage : version
	reqs chan *mergereq
}

func newMergebot(project string, cf *config) *mergebot {
	b := &mergebot{
		project: project,
		conf:    cf,
		vers:    make(map[string]*buildver),
		reqs:    make(chan *mergereq),
	}
	return b
}

func (b *mergebot) initMaster(notif *notif, build *build) {
	b.master.stage = build.stage
	b.master.ver = &buildver{build: build}
	cf, ok := b.conf.Envs[b.project]
	if !ok {
		log.Printf("[mergebot] %s: cannot find a directory to run git in", b.project)
		return
	}
	b.master.dir = cf.Dir
	log.Printf("[mergebot] %s: init master to %s using stage %s", b.project, notif.sha1, build.stage)
}

func (b *mergebot) send(req *mergereq) {
	b.reqs <- req
}

func (b *mergebot) run(projects *projects) {
	for req := range b.reqs {
		if req.build.stage != b.master.stage {
			// normally update some tracked version
			bv := b.vers[req.build.stage]
			if bv == nil {
				bv = &buildver{
					build: req.build,
				}
			}
			bv.sha1 = req.notif.sha1
			b.vers[req.build.stage] = bv
			log.Printf("[mergebot] %s: set latest revision to %s stage %s", b.project, req.notif.sha1, req.build.stage)
			continue
		}
		// It's a push to the master stage, trigger the delete etc
		bv := b.master.ver
		commits := newGitcommits()
		if bv.sha1 != "" {
			if err := commits.since(bv.sha1, b.master.dir); err != nil {
				log.Printf("[mergebot] %s: %s: can't fetch commits since %s: %s", b.project, b.master.dir, bv.sha1, err)
				continue
			}
			log.Printf("[mergebot] %s: got commits since %s from master", bv.sha1)
		} else {
			if err := commits.last(20, b.master.dir); err != nil {
				log.Printf("[mergebot] %s: %s: can't fetch last 20 commits: %s", b.project, b.master.dir, err)
				continue
			}
			log.Printf("[mergebot] %s: got last 20 commits from master", b.project)
		}
		for _, bv := range b.vers {
			if commits.isMerged(githash(bv.sha1)) {
				log.Printf("[mergebot] %s: can remove env %s, it was merged", b.project, bv.build.stage)
				projects.merge(bv.build, req.notif)
			} else {
				log.Printf("[mergebot] %s: %s not found in %s", bv.build.stage, bv.sha1, commits)
			}
		}
		log.Printf("[mergebot] %s: set latest revision to %s stage %s", b.project, req.notif.sha1, bv.build.stage)
		bv.sha1 = req.notif.sha1
	}
}

type mergebots map[string]*mergebot // project : mergebot

func makeMergebots() mergebots {
	return mergebots(make(map[string]*mergebot))
}

func (m mergebots) get(project string) *mergebot {
	return m[project]
}

func (m mergebots) add(project string, cf *config) {
	m[project] = newMergebot(project, cf)
}
