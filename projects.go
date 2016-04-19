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
	for proname, procf := range cf.Envs {
		// Start a mergebot for this project
		bot := bots.create(proname, cf)
		go bot.run(pjs)

		branchNotif := make(map[string]struct {
			notif *notif
			dir   string
		})
		for branch, dir := range procf.Merges {
			git := newGitcommits()
			log.Printf("[project] getting last commit for branch %s in %s", branch, dir)
			if err := git.last(1, dir); err != nil {
				log.Printf("[project] cannot determine last commit for branch %s dir %s: %s", branch, dir, err)
				continue
			}
			branchNotif[branch] = struct {
				notif *notif
				dir   string
			}{
				notif: newNotif(proname, string(git.commits[0].hash), branch),
				dir:   dir,
			}
		}
		for _, branch := range procf.Statics {
			bn, notifyMerge := branchNotif[branch]
			if !notifyMerge {
				bn.notif = newNotif(proname, "", branch)
			}
			builds, err := newBuilds(bn.notif, cf)
			if err != nil {
				log.Printf("[project] cannot add existing build %s, branch %s: %s", proname, branch, err)
				continue
			}
			for _, b := range builds {
				pjs.stages[b.stage] = b
				go b.run()
				log.Printf("[project] added stage %s tracking %s", b.stage, branch)
				if notifyMerge {
					// TODO: This is ugly because it needs a build object. Check.
					bot.addCheckout(bn.dir, bn.notif, b)
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
			log.Printf("[project] error processing build action: %s", err)
		}
	}
}

func (p *projects) push(b *build, n *notif, bot *mergebot) {
	p.reqs <- newProjectsReq(projectsActPush, b, n, bot)
}

func (p *projects) merge(b *build, n *notif) {
	p.reqs <- newProjectsReq(projectsActMerge, b, n, nil)
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
	log.Printf("[project] remove build stage %s", stage)
	build, ok := p.stages[stage]
	if !ok {
		return fmt.Errorf("unknown stage %s merged", stage)
	}
	build.request(store.BuildActDestroy, req.notif)
	build.destroy()
	delete(p.stages, stage)
	return nil
}
