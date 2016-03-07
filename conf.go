package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"regexp"
	"strconv"
	"time"
)

type config struct {
	BranchRegexp   string        `json:"branch_regexp"`
	WorkspacesDir  string        `json:"workspaces_dir"`
	LimitBuildsN   int           `json:"limit_builds"`
	CommandTimeout time.Duration `json:"command_timeout"`
	regexBranch    *regexp.Regexp
	// Limit the number of concurrent builds that can be performed
	limitBuilds chan struct{}
}

func newConfig(fname string) (*config, error) {
	file, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	var c config
	if err = json.Unmarshal(file, &c); err != nil {
		return nil, err
	}
	c.regexBranch = regexp.MustCompile(c.BranchRegexp)
	if c.LimitBuildsN > 0 {
		c.limitBuilds = make(chan struct{}, c.LimitBuildsN)
		for i := 0; i < c.LimitBuildsN; i++ {
			c.limitBuilds <- struct{}{}
		}
	}
	return &c, nil
}

func (c *config) parseTicketNo(branch string) (int64, error) {
	groups := c.regexBranch.FindAllStringSubmatch(branch, -1)
	if len(groups) > 0 && len(groups[0]) > 1 {
		return strconv.ParseInt(groups[0][1], 10, 64)
	}
	return 0, errors.New("could not match against regexp")
}
