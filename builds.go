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

type command struct {
	cmd  *exec.Cmd
	name string
}

func newCommand(act store.BuildAct, b *build) *command {
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
	if len(cmd) == 0 {
		return nil
	}
	cmd = b.stageVars.apply(cmd)
	return &command{
		cmd:  exec.Command(cmd[0], cmd[1:]...),
		name: strings.Join(cmd, " "),
	}
}

func (c *command) String() string {
	return c.name
}

type branchStages map[string][]string

// match returns the stage template for a branch
func (br branchStages) match(branch string) ([]string, bool) {
	tmpls, ok := br[branch]
	if ok {
		return tmpls, true
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
			return stage, true
		}
	}
	return nil, false
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

func (r *buildReq) String() string {
	return fmt.Sprintf("build-request %s", r.notif)
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
		return nil, fmt.Errorf("project %s not configured", n.project)
	}
	var (
		ticketNo int64
		err      error
	)
	tmpls, matched := branchStages(procf.Branches).match(n.branch)
	// If it is not a special stage, we can get the ticket number
	if !matched {
		tmpls = procf.Branches["__default__"]
		if len(tmpls) == 0 {
			return nil, fmt.Errorf("project %s does not specify a __default__ template", n.project)
		}
		ticketNo, err = conf.parseTicketNo(n.branch)
		if err != nil {
			return nil, fmt.Errorf("extrating ticket number: %s", err)
		}
	}
	if len(tmpls) == 0 {
		return nil, fmt.Errorf("project %s has no stages to build for branch %s", n.project, n.branch)
	}
	bs := make([]*build, 0, len(tmpls))
	for _, tmpl := range tmpls {
		b := &build{
			project:  n.project,
			branch:   n.branch,
			sha1:     n.sha1,
			conf:     conf,
			ticketNo: ticketNo,
			reqs:     make(chan *buildReq), // TODO: can be buffered
		}
		b.initVars(n.project, tmpl)
		bs = append(bs, b)
	}
	return bs, nil
}

func (b *build) String() string {
	return fmt.Sprintf("%s: %s", b.stage, b.branch)
}

func (b *build) initVars(project, tmpl string) {
	sv := makeVars()
	sv.add("ENV", project)
	sv.add("TICKET", fmt.Sprintf("%d", b.ticketNo))
	sv.add("BRANCH", b.branch)
	// Stage can include the previous vars
	b.stage = sv.applySingle(tmpl)
	sv.add("STAGE", b.stage)
	b.stageVars = sv
}

func (b *build) execResult(c *command) (*store.BuildResult, error) {
	return execResult(c.cmd, time.Duration(b.conf.CommandTimeout))
}

func (b *build) prepare(req *buildReq) {
	// On a change request, we might have a different branch
	if req.act == store.BuildActChange {
		if b.branch == req.notif.branch {
			req.act = store.BuildActUpdate
		} else {
			b.branch = req.notif.branch
			b.stageVars.add("BRANCH", b.branch)
		}
	}
}

func (b *build) execute(cmd *command, req *buildReq) (*store.BuildResult, error) {
	// Run the actual build command
	log.Printf("[build] %s: start '%s'", b, cmd)
	br, err := b.execResult(cmd)
	log.Printf("[build] %s: done '%s'", b, cmd)
	if err != nil {
		return nil, fmt.Errorf("command execution failed: %s", err)
	}
	return br, nil
}

func (b *build) persist(cmd *command, req *buildReq, br *store.BuildResult) error {
	// Fill and persist the build result
	br.Cmd = cmd.String()
	br.Act = req.act
	br.Branch = b.branch
	br.SHA1 = req.notif.sha1
	br.Stage = b.stage
	br.Ticket = b.ticketNo
	if err := b.conf.storage.Add(br); err != nil {
		return fmt.Errorf("cannot persist build result: %s", err)
	}
	return nil
}

func (b *build) doReq(req *buildReq) {
	b.prepare(req)
	cmd := newCommand(req.act, b)
	if cmd == nil {
		log.Printf("[build] %s: nothing to do", req)
		return
	}
	br, err := b.execute(cmd, req)
	if err != nil {
		log.Printf("[build] %s: build failed: %s", req, err)
		return
	}
	if err := b.persist(cmd, req, br); err != nil {
		log.Printf("[build] %s: build persistance failed: %s", req, err)
	}
}

func (b *build) run() {
	for req := range b.reqs {
		// Global builds concurrency semaphore
		if b.conf.limitBuilds != nil {
			<-b.conf.limitBuilds
		}
		b.doReq(req)
		if b.conf.limitBuilds != nil {
			b.conf.limitBuilds <- struct{}{}
		}
		req.done()
	}
	log.Printf("[build] %s: terminated", b)
}

func (b *build) request(act store.BuildAct, n *notif) {
	br := newBuildReq(act, n)
	b.reqs <- br
	br.wait()
}

func (b *build) destroy() {
	close(b.reqs)
}
