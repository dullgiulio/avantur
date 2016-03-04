package main

// TODO: Will come from conf.
const regexBranch = `^(?:[a-zA-Z0-9]+/)?(\d+)\-`

func main() {
	srv := newServer()
	go srv.serveBuilds()
	srv.serveHTTP(":8111")
}
