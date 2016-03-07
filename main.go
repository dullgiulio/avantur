package main

import (
	"flag"
	"log"
)

func main() {
	// TODO: Detect all existing ticket-based envs and the master (dev) ones.
	// TODO: Each env has a last commit associated. It is also detected on startup.

	flag.Parse()
	conffile := flag.Arg(0)

	cfg, err := newConfig(conffile)
	if err != nil {
		log.Fatal("configuration file failed to load: ", err)
	}
	srv := newServer()
	go srv.serveBuilds(cfg)
	log.Print("Listening to port 8111")
	srv.serveHTTP(":8111")
}
