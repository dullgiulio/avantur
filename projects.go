package main

import (
	"fmt"
	"log"

	"github.com/dullgiulio/avantur/store"
)

type projectsAct int

const (
	projectsActPush projectsAct = iota
	projectsActMerge
)

type projectsReq struct {
	act   projectsAct
	build *build
	notif *notif
	bot   *mergebot
}

func newProjectsReq(act projectsAct, b *build, n *notif, bot *mergebot) *projectsReq {
	return &projectsReq{
		act:   act,
		build: b,
		notif: n,
		bot:   bot,
	}
}

type projects struct {
	stages map[string]*build // stage : build
	reqs   chan *projectsReq
}

func newProjects(cf *config, bots mergebots) *projects {
	pjs := &projects{
		stages: make(map[string]*build),
		reqs:   make(chan *projectsReq),
	}
	// TODO: Detect and prefill envs automatically from existing dirs
	for proname, procf := range cf.Envs {
		var sha1 string
		git := newGitcommits()
		if err := git.last(1, procf.Dir); err == nil {
			sha1 = string(git.commits[0].hash)
		} else {
			log.Printf("[build] cannot determine last commit: %s", err)
		}
		for _, branch := range procf.Statics {
			notifyMerge := branch == "master"
			notif := newNotif(proname, sha1, branch)
			builds, err := newBuilds(notif, cf)
			if err != nil {
				log.Printf("[build] cannot add existing build %s, branch %s: %s", proname, branch, err)
				continue
			}
			for _, b := range builds {
				pjs.stages[b.stage] = b
				go b.run()
				log.Printf("[build] added stage %s tracking %s", b.stage, branch)
				if notifyMerge {
					// Set the master information for merges handling
					bots[proname].initMaster(notif, b)
					notifyMerge = false
				}
			}
		}
	}
	go pjs.run()
	return pjs
}

func (p *projects) run() {
	for req := range p.reqs {
		var err error
		switch req.act {
		case projectsActPush:
			err = p.doPush(req)
		case projectsActMerge:
			err = p.doMerge(req)
		}
		if err != nil {
			log.Printf("[build] error processing build action: %s", err)
		}
	}
}

func (p *projects) push(b *build, n *notif, bot *mergebot) {
	p.reqs <- newProjectsReq(projectsActPush, b, n, bot)
}

func (p *projects) merge(b *build, n *notif, bot *mergebot) {
	p.reqs <- newProjectsReq(projectsActMerge, b, n, bot)
}

// A branch has been pushed: create env or deploy to existing
func (p *projects) doPush(req *projectsReq) error {
	var act store.BuildAct
	if existingBuild, ok := p.stages[req.build.stage]; !ok {
		p.stages[req.build.stage] = req.build
		act = store.BuildActCreate
		go req.build.run()
	} else {
		// If the branch is the same as last seen, act will be treated as Update
		act = store.BuildActChange
		req.build = existingBuild
	}
	req.build.request(act, req.notif)
	req.bot.send(newMergereq(req.notif, req.build))
	return nil
}

func (p *projects) doMerge(req *projectsReq) error {
	stage := req.build.stage
	log.Printf("[build] remove build stage %s", stage)
	build, ok := p.stages[stage]
	if !ok {
		return fmt.Errorf("unknown stage %s merged", stage)
	}
	build.request(store.BuildActDestroy, req.notif)
	build.destroy()
	delete(p.stages, stage)
	return nil
}
