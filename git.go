// Copyright 2016 Giulio Iotti. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package umarell

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dullgiulio/umarell/store"
)

type githash []byte

// When comparing hashes, truncate to the shortest one
func (g githash) equal(o githash) bool {
	if len(g) > len(o) {
		g = g[:len(o)]
	}
	if len(o) > len(g) {
		o = o[:len(g)]
	}
	return bytes.Compare(o, g) == 0
}

type gitcommit struct {
	hash    githash
	parents []githash
}

func makeGitcommit(commit ...githash) gitcommit {
	return gitcommit{
		hash:    commit[0],
		parents: commit[1:],
	}
}

type gitcommits struct {
	commits []gitcommit
}

func newGitcommits() *gitcommits {
	return &gitcommits{}
}

func (g *gitcommits) String() string {
	return fmt.Sprintf("%s", g.commits)
}

// contains returns true if sha1 has been found in the history
func (g *gitcommits) contains(sha1 githash) bool {
	for i := range g.commits {
		if g.commits[i].hash.equal(sha1) {
			return true
		}
	}
	return false
}

func (g *gitcommits) since(sha1, dir string) error {
	cmd := exec.Command("git", "log", "--format=%H %P", fmt.Sprintf("%s..", sha1))
	cmd.Dir = dir
	return g.execCommits(cmd)
}

func (g *gitcommits) last(n int, dir string) error {
	cmd := exec.Command("git", "log", "--format=%H %P", fmt.Sprintf("-%d", n))
	cmd.Dir = dir
	return g.execCommits(cmd)
}

func (g *gitcommits) branch(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	return g.execBranch(cmd)
}

func (g *gitcommits) exec(cmd *exec.Cmd) (*store.BuildResult, error) {
	br, err := execResult(cmd, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("exec error: %s: %s: %s", cmd.Dir, strings.Join(cmd.Args, " "), err)
	}
	return br, nil
}

func (g *gitcommits) execCommits(cmd *exec.Cmd) error {
	br, err := g.exec(cmd)
	if err != nil {
		return fmt.Errorf("git error: %s", err)
	}
	return g.scanCommits(br.Stdout)
}

func (g *gitcommits) execBranch(cmd *exec.Cmd) (string, error) {
	br, err := g.exec(cmd)
	if err != nil {
		return "", fmt.Errorf("git error: %s", err)
	}
	return strings.TrimSpace(string(br.Stdout)), nil
}

func (g *gitcommits) scanCommits(output []byte) error {
	g.commits = make([]gitcommit, 0)
	sc := bufio.NewScanner(bytes.NewReader(output))
	for sc.Scan() {
		hashes := bytes.Split(sc.Bytes(), []byte(" "))
		ghashes := make([]githash, 0)
		for i := range hashes {
			if len(hashes[i]) > 0 {
				ghashes = append(ghashes, githash(hashes[i]))
			}
		}
		g.commits = append(g.commits, makeGitcommit(ghashes...))
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}
