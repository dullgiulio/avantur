package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// TODO: Will come from conf.
const regexBranch = `^(?:[a-zA-Z0-9]+/)?(\d+)\-`

// TODO: Implement Jenkins compatible REST interface.

// Commands output repository

type buildResult struct {
	// dates start + date end
	stdout []byte
	stderr []byte
	retval int
}

type buildRepository struct {
	mux  sync.RWMutex
	data map[string]map[int64][]*buildResult // env, ticket, buildResults
}

func newBuildRepository() *buildRepository {
	return &buildRepository{
		data: make(map[string]map[int64][]*buildResult),
	}
}

func (b *buildRepository) add(env string, ticket int64, br *buildResult) {
	b.mux.Lock()
	defer b.mux.Unlock()

	if _, ok := b.data[env]; !ok {
		b.data[env] = make(map[int64][]*buildResult)
	}
	if b.data[env][ticket] == nil {
		b.data[env][ticket] = make([]*buildResult, 0)
	}
	b.data[env][ticket] = append(b.data[env][ticket], br)
}

func (b *buildRepository) get(env string, ticket int64) []*buildResult {
	b.mux.RLock()
	defer b.mux.RUnlock()

	if _, ok := b.data[env]; !ok {
		return nil
	}
	return b.data[env][ticket]
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

type config struct {
	regexBranch *regexp.Regexp
	// Limit the number of concurrent builds that can be performed
	limitBuilds    chan struct{}
	commandTimeout time.Duration
}

func newConfig(reBranch string, nbuilds int) *config {
	c := &config{
		regexBranch:    regexp.MustCompile(regexBranch),
		commandTimeout: 2 * time.Second, // TODO: Make me some minutes
	}
	if nbuilds > 0 {
		c.limitBuilds = make(chan struct{}, nbuilds)
		for i := 0; i < nbuilds; i++ {
			c.limitBuilds <- struct{}{}
		}
	}
	return c
}

func (c *config) parseTicketNo(branch string) (int64, error) {
	groups := c.regexBranch.FindAllStringSubmatch(branch, -1)
	if len(groups) > 0 && len(groups[0]) > 1 {
		return strconv.ParseInt(groups[0][1], 10, 64)
	}
	return 0, errors.New("could not match against regexp")
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
	var err error
	var out, errOut bytes.Buffer

	wait := make(chan error)
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err = cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		wait <- cmd.Wait()
		close(wait)
	}()
	time.AfterFunc(b.conf.commandTimeout, func() {
		cmd.Process.Kill()
		if werr := <-wait; werr != nil {
			err = errors.New("timeout while executing command")
		} else {
			err = fmt.Errorf("timeout while executing command, kill process failed: %s", err)
		}
	})
	werr := <-wait
	if werr != nil && err == nil {
		err = werr
	}
	br := &buildResult{
		stdout: out.Bytes(),
		stderr: errOut.Bytes(),
		retval: 0, // TODO: Use exec.ExitError...
	}
	return br, err
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

func main() {
	branches := []string{
		"feature/1234-some-branch",
		"bugfix/2-another-branch",
		"123-something-something",
		"bugfix/2-another-branch",
	}
	envName := "microsites"
	cf := newConfig(regexBranch, 2)
	builds := makeBuilds()

	for i := range branches {
		b, err := newBuild(envName, branches[i], cf)
		if err != nil {
			log.Printf("%s: %s", branches[i], err)
			continue
		}
		builds.push(b.ticketNo, b)
	}

	select{}
}
