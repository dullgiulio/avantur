package main

import (
	"fmt"
	"log"
	"os/exec"
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

func (v vars) apply(a []string) []string {
	res := make([]string, len(a))
	for i := range a {
		for key, val := range v {
			res[i] = strings.Replace(a[i], key, val, -1)
		}
	}
	return res
}

type buildAct int

const (
	buildActCreate buildAct = iota
	buildActUpdate
	buildActDestroy
)

func (a buildAct) String() string {
	switch a {
	case buildActCreate:
		return "create"
	case buildActUpdate:
		return "update"
	case buildActDestroy:
		return "destroy"
	}
	return "unknown"
}

func (a buildAct) command(b *build) *exec.Cmd {
	var cmd []string
	vars := makeVars()
	vars.add("STAGE", b.stage())
	switch a {
	case buildActCreate:
		cmd = b.conf.Commands.CmdCreate
	case buildActUpdate:
		cmd = b.conf.Commands.CmdUpdate
	case buildActDestroy:
		cmd = b.conf.Commands.CmdDestroy
	}
	if cmd == nil {
		return nil
	}
	cmd = vars.apply(cmd)
	return exec.Command(cmd[0], cmd[1:]...)
}

type build struct {
	branch   string
	env      string
	ticketNo int64
	conf     *config
	acts     chan buildAct
}

func newBuild(env, branch string, conf *config) (*build, error) {
	b := &build{
		branch: branch,
		env:    env,
		conf:   conf,
		acts:   make(chan buildAct), // TODO: can be buffered
	}
	if err := b.ticket(); err != nil {
		return nil, err
	}
	go b.run()
	return b, nil
}

// TODO: This logic should be configurable (i.e. a map)
func (b *build) stage() string {
	if b.branch == "master" {
		return fmt.Sprintf("%s.dev", b.env)
	}
	if b.branch == "production" {
		return fmt.Sprintf("%s.hotfix", b.env)
	}
	return fmt.Sprintf("%s.typo%d", b.env, b.ticketNo)
}

func (b *build) ticket() error {
	// TODO: Handle all special cases, make them configurable
	if b.branch == "master" || b.branch == "production" {
		return nil
	}
	var err error
	b.ticketNo, err = b.conf.parseTicketNo(b.branch)
	return err
}

func (b *build) execResult(cmd *exec.Cmd) (*store.BuildResult, error) {
	return execResult(cmd, time.Duration(b.conf.CommandTimeout))
}

func (b *build) execute(act buildAct) {
	cmd := act.command(b)
	log.Printf("[build] env %s: branch %s: execute '%s'", b.env, b.branch, strings.Join(cmd.Args, " "))
	br, err := b.execResult(cmd)
	if err != nil {
		log.Printf("[build] command execution failed: %s", err)
		return
	}
	if err = b.conf.storage.Add(b.env, b.ticketNo, br); err != nil {
		log.Printf("[build] cannot persist build result: %s", err)
	}
	/*
		// TODO: Only if everythig is okay, we remove all results
		if act == buildActDestroy {
			if err = b.conf.storage.DeleteEnv(b.env); err != nil {
				log.Printf("cannot remove build results for %s: %s", b.env, err)
			}
		}
	*/
}

func (b *build) run() {
	for act := range b.acts {
		// Global builds concurrency semaphore
		if b.conf.limitBuilds != nil {
			<-b.conf.limitBuilds
		}
		b.execute(act)
		if b.conf.limitBuilds != nil {
			b.conf.limitBuilds <- struct{}{}
		}
	}
}

func (b *build) request(act buildAct) {
	b.acts <- act
}

func (b *build) destroy() {
	close(b.acts)
}

type builds map[string]*build // stage : build

func makeBuilds(cf *config) builds {
	bs := make(map[string]*build)
	// TODO: Detect and prefill envs automatically from existing dirs
	for _, env := range cf.Envs {
		b, err := newBuild(env, "master", cf)
		if err != nil {
			log.Printf("[build] cannot add existing build %s: %s", b.stage(), err)
			continue
		}
		bs[b.stage()] = b
		log.Printf("[build] added existing build %s", b.stage())
	}
	return bs
}

// A branch has been pushed: create env or deploy to existing
func (b builds) push(build *build) error {
	var act buildAct
	stage := build.stage()
	if existingBuild, ok := b[stage]; !ok {
		b[stage] = build
		act = buildActCreate
	} else {
		act = buildActUpdate
		build = existingBuild
	}
	build.request(act)
	return nil
}

// A branch has been merged to master, destory the env
func (b builds) merge(stage string) error {
	build, ok := b[stage]
	if !ok {
		return fmt.Errorf("unknown stage %s merged", stage)
	}
	build.request(buildActDestroy)
	build.destroy()
	return nil
}
