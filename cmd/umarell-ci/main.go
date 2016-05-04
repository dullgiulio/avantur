package main

import (
	"flag"
	"log"
	
	"github.com/dullgiulio/umarell"
)

func main() {
	listen := flag.String("listen", ":8111", "Listen to `[ADDR]:PORT`")
	flag.Parse()
	conffile := flag.Arg(0)

	cfg, err := newConfig(conffile)
	if err != nil {
		log.Fatal("configuration file failed to load: ", err)
	}
	srv := newServer(cfg)
	go srv.serveReqs(cfg)
	log.Printf("Listening to port %s", *listen)
	srv.serveHTTP(*listen)
}
