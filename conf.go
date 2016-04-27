package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"time"

	"github.com/dullgiulio/umarell-ci/store"
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
	LimitBuildsN    int      `json:"limit_builds"`
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
	} `json:"environments"`
	regexBranch *regexp.Regexp
	// Limit the number of concurrent builds that can be performed
	limitBuilds chan struct{}
	storage     store.Store
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
	if c.Database != "" && c.Table != "" {
		if c.storage, err = store.NewMysql(c.Database, c.Table); err != nil {
			log.Printf("[error] cannot start database storage: %s", err)
		}
	}
	if c.storage == nil {
		log.Printf("[info] no database configured, using memory storage")
		c.storage = store.NewMemory()
	}
	if c.ResultsDuration > 0 && c.ResultsCleanup > 0 {
		go c.cleaner()
	}
	return &c, nil
}

func (c *config) cleaner() {
	d := time.Duration(c.ResultsDuration)
	s := time.Duration(c.ResultsCleanup)
	for {
		before := time.Now().Add(-d)
		log.Printf("[server] results cleaner: cleaning jobs before %s", before.Format("2006-02-01 15:04:05"))
		if err := c.storage.Clean(before); err != nil {
			log.Printf("[error] results cleaner: %s", err)
		}
		time.Sleep(s)
	}
}

func (c *config) parseTicketNo(branch string) (int64, error) {
	groups := c.regexBranch.FindAllStringSubmatch(branch, -1)
	if len(groups) > 0 && len(groups[0]) > 1 {
		return strconv.ParseInt(groups[0][1], 10, 64)
	}
	return 0, errors.New("could not match against regexp")
}
