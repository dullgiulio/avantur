package store

type BuildResult struct {
	// dates start + date end
	Stdout []byte
	Stderr []byte
	Retval int
}

type Store interface {
	Add(env string, ticket int64, br *BuildResult) error
	Get(env string, ticket int64) ([]*BuildResult, error)
	DeleteTicket(env string, ticket int64) error
	DeleteEnv(env string) error
}
