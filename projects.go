package main

import (
	"fmt"
	"log"

	"github.com/dullgiulio/umarell-ci/store"
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
	notifs map[string]*notif // stage : notif
	reqs   chan *projectsReq
	conf   *config
}

type dirnotif struct {
	notif *notif
	dir   string
}

type branchDirnotif struct {
	entries map[string]*dirnotif
	name    string
}

func newBranchDirnotif(name string) *branchDirnotif {
	return &branchDirnotif{
		entries: make(map[string]*dirnotif),
		name:    name,
	}
}

func (b *branchDirnotif) add(branch, dir string) error {
	git := newGitcommits()
	log.Printf("[project] getting last commit for branch %s in %s", branch, dir)
	if err := git.last(1, dir); err != nil {
		return fmt.Errorf("cannot detect last commit for branch %s dir %s: %s", branch, dir, err)
	}
	b.entries[branch] = &dirnotif{
		notif: newNotif(b.name, string(git.commits[0].hash), branch),
		dir:   dir,
	}
	return nil
}

func (b branchDirnotif) get(branch string) (*dirnotif, bool) {
	bn, ok := b.entries[branch]
	if ok {
		return bn, true
	}
	return &dirnotif{
		notif: newNotif(b.name, "", branch),
	}, false
}

func newProjects(cf *config, bots mergebots) *projects {
	pjs := &projects{
		stages: make(map[string]*build),
		notifs: make(map[string]*notif),
		reqs:   make(chan *projectsReq),
		conf:   cf,
	}
	for name := range cf.Envs {
		pjs.initProject(name, bots, cf)
	}
	go pjs.run()
	return pjs
}

func (p *projects) initProject(name string, bots mergebots, cf *config) {
	envcf := cf.Envs[name]
	// Start a mergebot for this project
	bot := bots.create(name, cf)
	go bot.run(p)
	// Detect the last commit for each checked-out project
	branchNotif := newBranchDirnotif(name)
	for branch, dir := range envcf.Merges {
		if err := branchNotif.add(branch, dir); err != nil {
			log.Printf("[project] %s: error initializing checked-out project: %s", name, err)
		}
	}
	// Create builds for already existing static environments
	for _, branch := range envcf.Statics {
		if err := p.initStatic(branch, cf, bot, branchNotif); err != nil {
			log.Printf("[project] %s: cannot init static checkout: %s", name, err)
		}
	}
}

func (p *projects) initStatic(branch string, cf *config, bot *mergebot, bns *branchDirnotif) error {
	bn, notifyMerge := bns.get(branch)
	builds, err := newBuilds(bn.notif, cf)
	if err != nil {
		return fmt.Errorf("cannot create build for branch %s: %s", branch, err)
	}
	if len(builds) == 0 {
		return fmt.Errorf("no static builds to manage for branch %s", branch)
	}
	for _, b := range builds {
		p.stages[b.stage] = b
		go b.run()
		log.Printf("[project] added stage %s tracking %s", b.stage, branch)
	}
	// Notify the merge detector that this is the current build and notif for this directory
	if notifyMerge {
		bot.addCheckout(bn.dir, bn.notif, builds[0])
	}
	return nil
}

func (p *projects) run() {
	for req := range p.reqs {
		var err error
		switch req.act {
		case projectsActPush:
			p.notifs[req.build.stage] = req.notif
			err = p.doPush(req)
		case projectsActMerge:
			notif, ok := p.notifs[req.build.stage]
			if !ok {
				log.Printf("[project] skipping ghost merge request for %s", req.build.stage)
				continue
			}
			// We can accept a merge notification if there have been no new
			// pushes after the merge check was first triggered.
			if notif.equal(req.notif) {
				err = p.doMerge(req)
				delete(p.notifs, req.build.stage)
			} else {
				log.Printf("[project] ignoring merge request for %s as it is not up-to-date", req.build.stage)
			}
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
		p.conf.urls.set(req.build.stage, req.build.url(reverseJenkinsURL)) // TODO: Using global here sucks.
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
