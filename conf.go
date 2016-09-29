// Copyright 2016 Giulio Iotti. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package umarell

// TODO: Separate conf from interface to it

import (
	"encoding/json"
	"io/ioutil"
	"time"
)

type duration time.Duration

func (d *duration) UnmarshalJSON(b []byte) error {
	var (
		v   int64
		err error
	)
	if b[0] == '"' {
		sd := string(b[1 : len(b)-1])
		dd, err := time.ParseDuration(sd)
		*d = duration(dd)
		return err
	}
	v, err = json.Number(string(b)).Int64()
	*d = duration(time.Duration(v))
	return err
}

type config struct {
	BranchRegexp    string   `json:"branch_regexp"`
	WorkspacesDir   string   `json:"workspaces_dir"`
	Database        string   `json:"database"`
	Table           string   `json:"table"`
	LimitBuilds     int      `json:"limit_builds"`
	ResultsDuration duration `json:"results_duration"`
	ResultsCleanup  duration `json:"results_cleanup"`
	CommandTimeout  duration `json:"command_timeout"`
	Commands        struct {
		CmdChange  []string `json:"change"`
		CmdCreate  []string `json:"create"`
		CmdUpdate  []string `json:"update"`
		CmdDestroy []string `json:"destroy"`
	} `json:"commands"`
	Envs map[string]struct {
		Branches map[string][]string `json:"branches"`
		Statics  []string            `json:"staticBranches"`
		Merges   map[string]string   `json:"merges"` // branch : dir
		Commands *struct {
			CmdChange  []string `json:"change"`
			CmdCreate  []string `json:"create"`
			CmdUpdate  []string `json:"update"`
			CmdDestroy []string `json:"destroy"`
		} `json:"commands"`
	} `json:"environments"`
}

func NewConfigJSONFile(fname string) (*config, error) {
	file, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	var c config
	if err = json.Unmarshal(file, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
