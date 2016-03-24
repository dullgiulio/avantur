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
	env     string
	sha1    string
	trigger bool
	build   *build
}

func newMergereq(env, sha1 string, trigger bool, build *build) *mergereq {
	return &mergereq{
		env:     env,
		sha1:    sha1,
		trigger: trigger,
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
		bv := b.vers[req.env]
		if bv == nil {
			bv = &buildver{
				build: req.build,
			}
		}
		bv.sha1 = req.sha1
		b.vers[req.env] = bv
		// If this request is not a trigger, we are done
		if !req.trigger {
			continue
		}
		dir, ok := b.conf.Dirs[bv.build.stage]
		if !ok {
			log.Printf("[mergebot] %s: cannot find a directory to run git in", b.name)
			continue
		}
		if bv.commits == nil {
			bv.commits = newGitcommits()
		}
		if bv.sha1 != "" {
			if err := bv.commits.since(bv.sha1, dir); err != nil {
				log.Printf("[mergebot] %s: %s: error running git: %s", b.name, dir, err)
				continue
			}
		} else {
			if err := bv.commits.last(20, dir); err != nil {
				log.Printf("[mergebot] %s: %s: error running git: %s", b.name, dir, err)
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
