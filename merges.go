package main

import (
	"log"
)

type buildver struct {
	sha1    string
	build   *build
	commits *gitcommits
}

type mergereq struct {
	trigger bool
	notif   *notif
	build   *build
}

func newMergereq(notif *notif, build *build, trigger bool) *mergereq {
	return &mergereq{
		trigger: trigger,
		notif:   notif,
		build:   build,
	}
}

type mergebot struct {
	conf *config
	name string
	vers map[string]*buildver
	reqs chan *mergereq
}

func newMergebot(name string, cf *config) *mergebot {
	b := &mergebot{
		name: name,
		conf: cf,
		vers: make(map[string]*buildver),
		reqs: make(chan *mergereq),
	}
	go b.run()
	return b
}

func (b *mergebot) send(req *mergereq) {
	b.reqs <- req
}

func (b *mergebot) run() {
	for req := range b.reqs {
		bv := b.vers[req.notif.env]
		if bv == nil {
			bv = &buildver{
				build: req.build,
			}
		}
		bv.sha1 = req.notif.sha1
		b.vers[req.notif.env] = bv
		// If this request is not a trigger, we are done
		if !req.trigger {
			continue
		}
		envcf, ok := b.conf.Envs[req.notif.env]
		if !ok {
			log.Printf("[mergebot] %s: cannot find a directory to run git in", b.name)
			continue
		}
		dir := envcf.Dir
		if bv.commits == nil {
			bv.commits = newGitcommits()
		}
		if bv.sha1 != "" {
			if err := bv.commits.since(bv.sha1, dir); err != nil {
				log.Printf("[mergebot] %s: %s: can't fetch commits since %s: %s", b.name, dir, bv.sha1, err)
				continue
			}
		} else {
			if err := bv.commits.last(20, dir); err != nil {
				log.Printf("[mergebot] %s: %s: can't fetch last 20 commits: %s", b.name, dir, err)
				continue
			}
		}
		for stage, bv := range b.vers {
			// TODO: Should skip dev and probably others
			if stage == "dev" {
				continue
			}
			if bv.commits.isMerged(githash(bv.sha1)) {
				log.Printf("[mergebot] can remove env %s.%s, it was merged", b.name, stage)
			}
		}
	}
}

type mergebots map[string]*mergebot // env : mergebot

func makeMergebots() mergebots {
	return mergebots(make(map[string]*mergebot))
}

func (m mergebots) get(env string) *mergebot {
	return m[env]
}

func (m mergebots) add(env string, cf *config) {
	m[env] = newMergebot(env, cf)
}
