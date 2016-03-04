package main

// TODO: Will come from conf.
const regexBranch = `^(?:[a-zA-Z0-9]+/)?(\d+)\-`

func main() {
	// TODO: Detect all existing ticket-based envs and the master (dev) ones.
	// TODO: Each env has a last commit associated. It is also detected on startup.

	srv := newServer()
	go srv.serveBuilds()
	srv.serveHTTP(":8111")
}
