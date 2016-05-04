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

	cfg, err := umarell.NewConfig(conffile)
	if err != nil {
		log.Fatal("configuration file failed to load: ", err)
	}
	srv := umarell.NewServer(cfg)
	go srv.ServeReqs(cfg)
	log.Printf("Listening to port %s", *listen)
	srv.ServeHTTP(*listen)
}
