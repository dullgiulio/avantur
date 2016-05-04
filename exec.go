package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/dullgiulio/umarell/store"
)

type execError struct {
	err  error
	out  []byte
	eout []byte
}

func newExecError(err error, out, eout []byte) *execError {
	return &execError{
		err:  err,
		out:  out,
		eout: eout,
	}
}

func (e *execError) Error() string {
	return fmt.Sprintf("%s\n--OUTPUT--\n%s--OUTPUT--\n--ERROR--\n%s\n--ERROR--", e.err, e.out, e.eout)
}

func execResult(cmd *exec.Cmd, timeout time.Duration) (*store.BuildResult, error) {
	var err error
	var out, errOut bytes.Buffer

	wait := make(chan error)
	over := make(chan struct{})
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	br := &store.BuildResult{
		Start: time.Now(),
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		select {
		case err = <-wait:
		case <-time.After(timeout):
			if err = cmd.Process.Kill(); err != nil {
				err = errors.New("timeout while executing command")
			} else {
				err = fmt.Errorf("timeout while executing command, kill process failed: %s", err)
			}
			<-wait
		}
		over <- struct{}{}
	}()
	wait <- cmd.Wait()
	<-over
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
	if err != nil {
		err = newExecError(err, br.Stdout, br.Stderr)
	}
	return br, err
}
