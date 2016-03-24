package main

import (
	"flag"
	"log"
)

func main() {
	flag.Parse()
	conffile := flag.Arg(0)

	cfg, err := newConfig(conffile)
	if err != nil {
		log.Fatal("configuration file failed to load: ", err)
	}
	srv := newServer(cfg)
	go srv.serveBuilds(cfg)
	log.Print("Listening to port 8111")
	srv.serveHTTP(":8111")
}
