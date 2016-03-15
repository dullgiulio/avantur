package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/dullgiulio/avantur/store"
)

func execResult(cmd *exec.Cmd, timeout time.Duration) (*store.BuildResult, error) {
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
	time.AfterFunc(timeout, func() {
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
	br := &store.BuildResult{
		Stdout: out.Bytes(),
		Stderr: errOut.Bytes(),
		Retval: 0, // TODO: Use exec.ExitError...
	}
	return br, err
}
