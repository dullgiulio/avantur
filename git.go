package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"time"
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
	return &gitcommits{commits: make([]gitcommit, 0)}
}

// isMerged returns true if sha1 has been merged in the g git history
func (g *gitcommits) isMerged(sha1 githash) bool {
	for i := range g.commits {
		// Skip non-merge commits
		if len(g.commits[i].parents) == 1 {
			continue
		}
		for j := range g.commits[i].parents {
			if g.commits[i].parents[j].equal(sha1) {
				return true
			}
		}
	}
	return false
}

func (g *gitcommits) since(sha1, dir string) error {
	cmd := exec.Command("git", "log", "--format=%H %P", fmt.Sprintf("%s..", sha1))
	cmd.Dir = dir
	return g.exec(cmd)
}

func (g *gitcommits) last(n int, dir string) error {
	cmd := exec.Command("git", "log", "--format=%H %P", fmt.Sprintf("-%d", n))
	cmd.Dir = dir
	return g.exec(cmd)
}

func (g *gitcommits) exec(cmd *exec.Cmd) error {
	br, err := execResult(cmd, 2*time.Second)
	if err != nil {
		return fmt.Errorf("error executing git command: %s", err)
	}
	commits := make([]gitcommit, 0)
	sc := bufio.NewScanner(bytes.NewReader(br.Stdout))
	for sc.Scan() {
		hashes := bytes.Split(sc.Bytes(), []byte(" "))
		ghashes := make([]githash, 0)
		for i := range hashes {
			if len(hashes[i]) > 0 {
				ghashes = append(ghashes, githash(hashes[i]))
			}
		}
		commits = append(commits, makeGitcommit(ghashes...))
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}
