package store

import "time"

type BuildResult struct {
	Start  time.Time
	End    time.Time
	Act    BuildAct
	Stdout []byte
	Stderr []byte
	Retval int
	Ticket int64
	Stage  string
	Branch string
	SHA1   string
}

type Store interface {
	Add(br *BuildResult) error
	Get(stage string) ([]*BuildResult, error)
	Delete(stage string) error
}

type BuildAct int

const (
	BuildActCreate BuildAct = iota
	BuildActUpdate
	BuildActChange
	BuildActDestroy
)

func (a BuildAct) String() string {
	switch a {
	case BuildActCreate:
		return "create"
	case BuildActUpdate:
		return "update"
	case BuildActChange:
		return "change"
	case BuildActDestroy:
		return "destroy"
	}
	return "unknown"
}
