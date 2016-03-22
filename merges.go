package main

type lastver struct {
	vers map[string]string // stage : sha1
	// chan
}

func newLastver() *lastver {
	l := &lastver{
		vers: make(map[string]string),
	}
	go l.run()
	return l
}

func (l *lastver) run() {
	// select chan:
	// case addVersion(stage, sha1):
	//    stage.setLastSHA1(sha1)
	// case rescanStage(stage):
	//    commits = scanStage(stage)
	//    merges = filterMerges(commits)
	//    for merge in merges:
	//         stage = stage where last version is merge
	//         stage.build.merge()
}
