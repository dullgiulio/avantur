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

type branchStages map[string]string

// match returns the stage template for a branch
func (br branchStages) match(branch, def string) string {
	tmpl, ok := br[branch]
	if ok {
		return tmpl
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
	branch string
}

func newBuildReq(act store.BuildAct, branch string) *buildReq {
	return &buildReq{act: act, branch: branch}
}

type build struct {
	stage     string
	branch    string
	sha1      string
	ticketNo  int64
	conf      *config
	acts      chan *buildReq
	stageVars vars
}

func newBuild(n *notif, conf *config) (*build, error) {
	b := &build{
		branch: n.branch,
		sha1:   n.sha1,
		conf:   conf,
		acts:   make(chan *buildReq), // TODO: can be buffered
	}
	defTmpl := b.conf.Branches["__default__"]
	tmpl := branchStages(b.conf.Branches).match(n.branch, defTmpl)
	// If it is not a special stage, we can get the ticket number
	if tmpl == defTmpl {
		if err := b.ticket(); err != nil {
			return nil, err
		}
	}
	sv := makeVars()
	sv.add("ENV", n.env)
	sv.add("TICKET", fmt.Sprintf("%d", b.ticketNo))
	sv.add("BRANCH", b.branch)
	b.stageVars = sv
	b.stage = b.stageVars.applySingle(tmpl)
	b.stageVars.add("STAGE", b.stage)
	go b.run()
	return b, nil
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
		if b.branch == req.branch {
			req.act = store.BuildActUpdate
		} else {
			b.branch = req.branch
			b.stageVars.add("BRANCH", b.branch)
		}
	}
	cmd := makeCommand(req.act, b)
	log.Printf("[build] stage %s: branch %s: execute '%s'", b.stage, b.branch, strings.Join(cmd.Args, " "))
	br, err := b.execResult(cmd)
	if err != nil {
		log.Printf("[build] command execution failed: %s", err)
		return
	}
	br.Act = req.act
	br.Branch = b.branch
	br.SHA1 = b.sha1
	br.Stage = b.stage
	if err = b.conf.storage.Add(br); err != nil {
		log.Printf("[build] cannot persist build result: %s", err)
	}
}

func (b *build) run() {
	for req := range b.acts {
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

func (b *build) request(act store.BuildAct, branch string) {
	b.acts <- newBuildReq(act, branch)
}

func (b *build) destroy() {
	close(b.acts)
}

type builds map[string]*build // stage : build

func makeBuilds(cf *config) builds {
	bs := make(map[string]*build)
	// TODO: Detect and prefill envs automatically from existing dirs
	for env, branches := range cf.Envs {
		for _, branch := range branches {
			b, err := newBuild(newNotif(env, "", branch), cf)
			if err != nil {
				log.Printf("[build] cannot add existing build %s: %s", env, err)
				continue
			}
			bs[b.stage] = b
			log.Printf("[build] added stage %s tracking %s", b.stage, branch)
		}
	}
	return bs
}

// A branch has been pushed: create env or deploy to existing
func (b builds) push(build *build) error {
	var (
		act    store.BuildAct
		branch string
	)
	if existingBuild, ok := b[build.stage]; !ok {
		b[build.stage] = build
		act = store.BuildActCreate
	} else {
		// If the branch is the same as last seen, act will be treated as Update
		act = store.BuildActChange
		branch = build.branch
		build.destroy() // Prevent leaking a goroutine
		build = existingBuild
	}
	build.request(act, branch)
	return nil
}

// A branch has been merged to master, destory the env
func (b builds) merge(stage string) error {
	build, ok := b[stage]
	if !ok {
		return fmt.Errorf("unknown stage %s merged", stage)
	}
	build.request(store.BuildActDestroy, "")
	build.destroy()
	return nil
}
