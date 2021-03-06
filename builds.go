// Copyright 2016 Giulio Iotti. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package umarell

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dullgiulio/umarell/store"
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
		cmd = b.getCmd("create")
	case store.BuildActChange:
		cmd = b.getCmd("change")
	case store.BuildActUpdate:
		cmd = b.getCmd("update")
	case store.BuildActDestroy:
		cmd = b.getCmd("destroy")
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
func (br branchStages) match(branch string, log logger) ([]string, bool) {
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

func parseTicketNo(srv *server, branch string) (int64, error) {
	groups := srv.regexBranch.FindAllStringSubmatch(branch, -1)
	if len(groups) > 0 && len(groups[0]) > 1 {
		return strconv.ParseInt(groups[0][1], 10, 64)
	}
	return 0, errors.New("could not match against regexp")
}

type build struct {
	project   string
	stage     string
	branch    string
	sha1      string
	ticketNo  int64
	reqs      chan *buildReq
	stageVars vars
	srv       *server
}

func newBuilds(n *notif, srv *server) ([]*build, error) {
	procf, ok := srv.conf.Envs[n.project]
	if !ok {
		return nil, fmt.Errorf("project %s not configured", n.project)
	}
	var (
		ticketNo int64
		err      error
	)
	tmpls, matched := branchStages(procf.Branches).match(n.branch, srv.log)
	// If it is not a special stage, we can get the ticket number
	if !matched {
		tmpls = procf.Branches["__default__"]
		if len(tmpls) == 0 {
			return nil, fmt.Errorf("project %s does not specify a __default__ template", n.project)
		}
		ticketNo, err = parseTicketNo(srv, n.branch)
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
			srv:      srv,
			ticketNo: ticketNo,
			reqs:     make(chan *buildReq), // XXX: can be buffered
		}
		b.initVars(n.project, tmpl)
		bs = append(bs, b)
	}
	return bs, nil
}

func (b *build) String() string {
	return fmt.Sprintf("%s: %s", b.stage, b.branch)
}

func (b *build) getCmd(c string) []string {
	var cmd []string
	bcmd := b.srv.conf.Envs[b.project].Commands
	switch c {
	case "create":
		cmd = b.srv.conf.Commands.CmdCreate
		if bcmd != nil && bcmd.CmdCreate != nil {
			cmd = bcmd.CmdCreate
		}
	case "change":
		cmd = b.srv.conf.Commands.CmdChange
		if bcmd != nil && bcmd.CmdChange != nil {
			cmd = bcmd.CmdChange
		}
	case "update":
		cmd = b.srv.conf.Commands.CmdUpdate
		if bcmd != nil && bcmd.CmdUpdate != nil {
			cmd = bcmd.CmdUpdate
		}
	case "destroy":
		cmd = b.srv.conf.Commands.CmdDestroy
		if bcmd != nil && bcmd.CmdDestroy != nil {
			cmd = bcmd.CmdDestroy
		}
	default:
		panic("Only use create, change, update, destroy")
	}
	return cmd
}

func (b *build) url(tmpl string) string {
	// Variables to make the GET url, Jenkins-style
	v := url.Values{}
	v.Set("branches", b.branch)
	v.Set("sha1", b.sha1)
	// Variables to display in the template
	vs := makeVars()
	vs.add("branch", b.branch)
	vs.add("sha1", b.sha1)
	vs.add("ticket", fmt.Sprintf("%d", b.ticketNo))
	vs.add("project", b.project)
	vs.add("params", v.Encode())
	return vs.applySingle(tmpl)
}

func (b *build) initVars(project, tmpl string) {
	vs := makeVars()
	vs.add("ENV", project)
	vs.add("TICKET", fmt.Sprintf("%d", b.ticketNo))
	vs.add("BRANCH", b.branch)
	// Stage can include the previous vars
	b.stage = vs.applySingle(tmpl)
	vs.add("STAGE", b.stage)
	b.stageVars = vs
}

func (b *build) execResult(c *command) (*store.BuildResult, error) {
	return execResult(c.cmd, time.Duration(b.srv.conf.CommandTimeout))
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
	b.srv.log.Printf("[build] %s: start '%s'", b, cmd)
	br, err := b.execResult(cmd)
	b.srv.log.Printf("[build] %s: done '%s'", b, cmd)
	if err != nil {
		return br, fmt.Errorf("command execution failed: %s", err)
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
	if err := b.srv.storage.Add(br); err != nil {
		return fmt.Errorf("cannot persist build result: %s", err)
	}
	return nil
}

func (b *build) doReq(req *buildReq) {
	b.prepare(req)
	cmd := newCommand(req.act, b)
	if cmd == nil {
		b.srv.log.Printf("[build] %s: nothing to do", req)
		return
	}
	br, err := b.execute(cmd, req)
	if err != nil {
		b.srv.log.Printf("[build] %s: build failed: %s", req, err)
	}
	// If the build failed but there is a result to save.
	if br != nil {
		if err := b.persist(cmd, req, br); err != nil {
			b.srv.log.Printf("[build] %s: build persistance failed: %s", req, err)
		}
	}
}

func (b *build) run() {
	for req := range b.reqs {
		b.srv.startBuild()
		b.doReq(req)
		b.srv.stopBuild()
		req.done()
	}
	b.srv.log.Printf("[build] %s: terminated", b)
}

func (b *build) request(act store.BuildAct, n *notif) {
	br := newBuildReq(act, n)
	b.reqs <- br
	br.wait()
}

func (b *build) destroy() {
	close(b.reqs)
}
