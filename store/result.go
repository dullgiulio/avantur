package store

import "time"

type BuildResult struct {
	Start  time.Time
	End    time.Time
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
