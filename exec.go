package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/dullgiulio/avantur/store"
)

func execResult(cmd *exec.Cmd, timeout time.Duration) (*store.BuildResult, error) {
	var err error
	var out, errOut bytes.Buffer

	wait := make(chan error)
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	br := &store.BuildResult{
		Start: time.Now(),
	}
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
	retval := 0
	if e, ok := err.(*exec.ExitError); ok {
		if status, ok := e.Sys().(syscall.WaitStatus); ok {
			retval = status.ExitStatus()
		}
	}
	br.End = time.Now()
	br.Retval = retval
	br.Stdout = out.Bytes()
	br.Stderr = errOut.Bytes()
	return br, err
}
