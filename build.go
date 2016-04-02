package main

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/dullgiulio/avantur/store"
)

type vars map[string]string

func makeVars() vars {
	return vars(make(map[string]string))
}

func (v vars) add(key, val string) {
	v[fmt.Sprintf("{%s}", key)] = val
}

func (v vars) applySingle(s string) string {
	for key, val := range v {
		s = strings.Replace(s, key, val, -1)
	}
	return s
}

func (v vars) apply(a []string) []string {
	res := make([]string, len(a))
	for i := range a {
		res[i] = v.applySingle(a[i])
	}
	return res
}

func makeCommand(act store.BuildAct, b *build) *exec.Cmd {
	var cmd []string
	switch act {
	case store.BuildActCreate:
		cmd = b.conf.Commands.CmdCreate
	case store.BuildActChange:
		cmd = b.conf.Commands.CmdChange
	case store.BuildActUpdate:
		cmd = b.conf.Commands.CmdUpdate
	case store.BuildActDestroy:
		cmd = b.conf.Commands.CmdDestroy
	}
	if cmd == nil {
		return nil
	}
	cmd = b.stageVars.apply(cmd)
	return exec.Command(cmd[0], cmd[1:]...)
}

type branchStages map[string][]string

// match returns the stage template for a branch
func (br branchStages) match(branch string, def []string) []string {
	tmpls, ok := br[branch]
	if ok {
		return tmpls
	}
	var (
		found bool
		err   error
	)
	// Try if the branch matches a regex pattern
	for pattern, stage := range br {
		if pattern[0] != '^' {
			continue
		}
		found, err = regexp.MatchString(pattern, branch)
		if err != nil {
			log.Printf("[build] cannot match %s against %s: %s", branch, pattern, err)
			continue
		}
		if found {
			return stage
		}
	}
	return def
}

type buildReq struct {
	act   store.BuildAct
	notif *notif
}

func newBuildReq(act store.BuildAct, n *notif) *buildReq {
	return &buildReq{act: act, notif: n}
}

type build struct {
	project   string
	stage     string
	branch    string
	sha1      string
	ticketNo  int64
	conf      *config
	reqs      chan *buildReq
	stageVars vars
}

func newBuilds(n *notif, conf *config) ([]*build, error) {
	var hasTicket bool
	procf, ok := conf.Envs[n.project]
	if !ok {
		return nil, fmt.Errorf("[build] project %s not configured", n.project)
	}
	defTmpl := procf.Branches["__default__"]
	tmpls := branchStages(procf.Branches).match(n.branch, defTmpl)
	// If it is not a special stage, we can get the ticket number
	if tmpls[0] == defTmpl[0] {
		hasTicket = true
	}
	bs := make([]*build, 0)
	for _, tmpl := range tmpls {
		b := &build{
			project: n.project,
			branch:  n.branch,
			sha1:    n.sha1,
			conf:    conf,
			reqs:    make(chan *buildReq), // TODO: can be buffered
		}
		if hasTicket {
			if err := b.ticket(); err != nil {
				return nil, err
			}
		}
		sv := makeVars()
		sv.add("ENV", n.project)
		sv.add("TICKET", fmt.Sprintf("%d", b.ticketNo))
		sv.add("BRANCH", b.branch)
		b.stageVars = sv
		b.stage = b.stageVars.applySingle(tmpl)
		b.stageVars.add("STAGE", b.stage)
		bs = append(bs, b)
	}
	return bs, nil
}

func (b *build) ticket() error {
	var err error
	b.ticketNo, err = b.conf.parseTicketNo(b.branch)
	return err
}

func (b *build) execResult(cmd *exec.Cmd) (*store.BuildResult, error) {
	return execResult(cmd, time.Duration(b.conf.CommandTimeout))
}

func (b *build) execute(req *buildReq) {
	// On a change request, we might have a different branch
	if req.act == store.BuildActChange {
		if b.branch == req.notif.branch {
			req.act = store.BuildActUpdate
		} else {
			b.branch = req.notif.branch
			b.stageVars.add("BRANCH", b.branch)
		}
	}
	cmd := makeCommand(req.act, b)
	command := strings.Join(cmd.Args, " ")
	log.Printf("[build] stage %s: branch %s: execute '%s'", b.stage, b.branch, command)
	br, err := b.execResult(cmd)
	if err != nil {
		log.Printf("[build] command execution failed: %s", err)
		return
	}
	br.Cmd = command
	br.Act = req.act
	br.Branch = b.branch
	br.SHA1 = req.notif.sha1
	br.Stage = b.stage
	if err = b.conf.storage.Add(br); err != nil {
		log.Printf("[build] cannot persist build result: %s", err)
	}
}

func (b *build) run() {
	for req := range b.reqs {
		// Global builds concurrency semaphore
		if b.conf.limitBuilds != nil {
			<-b.conf.limitBuilds
		}
		b.execute(req)
		if b.conf.limitBuilds != nil {
			b.conf.limitBuilds <- struct{}{}
		}
	}
}

func (b *build) request(act store.BuildAct, n *notif) {
	b.reqs <- newBuildReq(act, n)
}

func (b *build) destroy() {
	close(b.reqs)
}

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

func newProjectsReq(b *build, n *notif, bot *mergebot) *projectsReq {
	return &projectsReq{
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
	p.reqs <- newProjectsReq(b, n, bot)
}

func (p *projects) merge(b *build, n *notif) {
	p.reqs <- newProjectsReq(b, n, nil)
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
	build.request(store.BuildActDestroy, nil)
	build.destroy()
	delete(p.stages, stage)
	return nil
}
