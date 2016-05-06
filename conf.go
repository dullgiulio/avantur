package umarell

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
	} `json:"environments"`
}

func (c *config) GetBranchRegexp() string {
	return c.BranchRegexp
}

func (c *config) GetWorkspacesDir() string {
	return c.WorkspacesDir
}

func (c *config) GetCmdChange() []string {
	return c.Commands.CmdChange
}

func (c *config) GetCmdCreate() []string {
	return c.Commands.CmdCreate
}

func (c *config) GetCmdUpdate() []string {
	return c.Commands.CmdUpdate
}

func (c *config) GetCmdDestroy() []string {
	return c.Commands.CmdDestroy
}

func (c *config) GetCommandTimeout() duration {
	return c.CommandTimeout
}

func (c *config) GetEnvNames() []string {
	names := make([]string, len(c.Envs))
	i := 0
	for k := range c.Envs {
		names[i] = k
	}
	return names
}

func (c *config) HasEnv(name string) bool {
	_, ok := c.Envs[name]
	return ok
}

func (c *config) SetEnvProperty(prop, key, val string) error {
	parts := strings.Split(prop, ".", -1)
	if len(parts) < 3 {
		return errors.New("invalid property to set")
	}
	envName := parts[0]
	if _, ok := c.Envs[envName]; !ok {
		return fmt.Errorf("non-existing project %s", envName)
	}
	switch parts[1] {
	case "branches":
		b := c.Env[envName].Branches[key]
		for i := range b {
			if b[i] == val {
				return fmt.Errorf("cannot insert %s into %s.branches: already existing", val, envName)
			}
		}
		b = append(b, val)
		c.Env[envName].Branches[key] = b
	case "merges":
		c.Env[envName].Merges[key] = val
	default:
		return fmt.Errorf("invalid property %s to set", parts[1])
	}
	return nil
}

func (c *config) Get() {
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
