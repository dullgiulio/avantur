package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

func execResult(cmd *exec.Cmd, timeout time.Duration) (*buildResult, error) {
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
	br := &buildResult{
		stdout: out.Bytes(),
		stderr: errOut.Bytes(),
		retval: 0, // TODO: Use exec.ExitError...
	}
	return br, err
}
