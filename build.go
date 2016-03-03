package main

import (
	"fmt"
	"log"
	"os/exec"
)

type buildResult struct {
	// dates start + date end
	stdout []byte
	stderr []byte
	retval int
}

type buildAct int

const (
	buildActCreate buildAct = iota
	buildActUpdate
	buildActDestroy
)

// TODO: Stuff from here comes from configuration
func (a buildAct) command(b *build) *exec.Cmd {
	switch a {
	case buildActCreate:
		return exec.Command("echo", "t3p", "env:init", fmt.Sprintf("%s.typo%d", b.env, b.ticketNo))
	case buildActUpdate:
		return exec.Command("echo", "t3p", "deploy", fmt.Sprintf("%s.typo%d", b.env, b.ticketNo))
	case buildActDestroy:
		return exec.Command("echo", "t3p", "env:del", fmt.Sprintf("%s.typo%d", b.env, b.ticketNo))
	}
	return nil
}

type build struct {
	branch   string
	env      string
	ticketNo int64
	conf     *config
	acts     chan buildAct
}

func newBuild(env, branch string, conf *config) (*build, error) {
	var err error
	b := &build{
		branch: branch,
		env:    env,
		conf:   conf,
		acts:   make(chan buildAct), // TODO: can be buffered
	}
	if b.ticketNo, err = conf.parseTicketNo(branch); err != nil {
		return nil, err
	}
	go b.run()
	return b, nil
}

func (b *build) execResult(cmd *exec.Cmd) (*buildResult, error) {
	return execResult(cmd, b.conf.commandTimeout)
}

func (b *build) execute(act buildAct) {
	cmd := act.command(b)
	br, err := b.execResult(cmd)
	if err != nil {
		log.Printf("command execution failed: %s", err)
		return
	}
	// TODO: add the build result to global repo
	fmt.Printf("%q\n", br)
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

type builds map[int64]*build // ticketNo : build

func makeBuilds() builds {
	return make(map[int64]*build)
}

// A branch has been pushed: create env or deploy to existing
func (b builds) push(ticket int64, build *build) error {
	var act buildAct
	if existingBuild, ok := b[ticket]; !ok {
		b[ticket] = build
		act = buildActCreate
	} else {
		act = buildActUpdate
		build = existingBuild
	}
	build.request(act)
	return nil
}

// A branch has been merged to master, destory the env
func (b builds) merge(ticket int64) error {
	build, ok := b[ticket]
	if !ok {
		return fmt.Errorf("unknown ticket #%d merged", ticket)
	}
	build.request(buildActDestroy)
	return nil
}
