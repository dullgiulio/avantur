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
	act    store.BuildAct
	notif  *notif
	doneCh chan struct{}
}

func newBuildReq(act store.BuildAct, n *notif) *buildReq {
	return &buildReq{
		act:    act,
		notif:  n,
		doneCh: make(chan struct{}),
	}
}

func (r *buildReq) done() {
	close(r.doneCh)
}

func (r *buildReq) wait() {
	<-r.doneCh
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
	procf, ok := conf.Envs[n.project]
	if !ok {
		return nil, fmt.Errorf("[build] project %s not configured", n.project)
	}
	defTmpl := procf.Branches["__default__"]
	tmpls := branchStages(procf.Branches).match(n.branch, defTmpl)
	// If it is not a special stage, we can get the ticket number
	var (
		ticketNo int64
		err      error
	)
	if tmpls[0] == defTmpl[0] {
		ticketNo, err = conf.parseTicketNo(n.branch)
		if err != nil {
			return nil, err
		}
	}
	bs := make([]*build, 0)
	for _, tmpl := range tmpls {
		b := &build{
			project:  n.project,
			branch:   n.branch,
			sha1:     n.sha1,
			conf:     conf,
			ticketNo: ticketNo,
			reqs:     make(chan *buildReq), // TODO: can be buffered
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
	log.Printf("[build] stage %s: branch %s: execute '%s': started", b.stage, b.branch, command)
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
	log.Printf("[build] stage %s: branch %s: execute '%s': done", b.stage, b.branch, command)
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
		req.done()
	}
	log.Printf("[build] %s: terminated", b.stage)
}

func (b *build) request(act store.BuildAct, n *notif) {
	br := newBuildReq(act, n)
	b.reqs <- br
	br.wait()
}

func (b *build) destroy() {
	close(b.reqs)
}
