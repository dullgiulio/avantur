package main

import (
	"errors"
	"regexp"
	"strconv"
	"time"
)

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
