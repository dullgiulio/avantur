package main

import (
	"log"
)

// TODO: Will come from conf.
const regexBranch = `^(?:[a-zA-Z0-9]+/)?(\d+)\-`

// TODO: Implement Jenkins compatible REST interface.

func main() {
	branches := []string{
		"feature/1234-some-branch",
		"bugfix/2-another-branch",
		"123-something-something",
		"bugfix/2-another-branch",
	}
	envName := "microsites"
	cf := newConfig(regexBranch, 2)
	builds := makeBuilds()

	for i := range branches {
		b, err := newBuild(envName, branches[i], cf)
		if err != nil {
			log.Printf("%s: %s", branches[i], err)
			continue
		}
		builds.push(b.ticketNo, b)
	}

	select {}
}
