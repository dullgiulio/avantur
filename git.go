package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"time"
)

type githash []byte

type gitcommit struct {
	hash    githash
	parents []githash
}

func newGitcommit(commit ...githash) *gitcommit {
	return &gitcommit{
		hash:    commit[0],
		parents: commit[1:],
	}
}

func gitLastCommit(dir string) (*gitcommit, error) {
	cmd := exec.Command("git", "log", "--format=%H %P", "-1")
	cmd.Dir = dir
	br, err := execResult(cmd, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("error executing git command: %s", err)
	}
	if len(br.stdout) > 0 && br.stdout[len(br.stdout)-1] == '\n' {
		br.stdout = br.stdout[:len(br.stdout)-1]
	}
	hashes := bytes.Split(br.stdout, []byte(" "))
	ghashes := make([]githash, 0)
	for i := range hashes {
		if len(hashes[i]) > 0 {
			ghashes = append(ghashes, githash(hashes[i]))
		}
	}
	return newGitcommit(ghashes...), nil
}
