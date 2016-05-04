package main

import (
	"flag"
	"log"
)

func main() {
	listen := flag.String("listen", ":8111", "Listen to `[ADDR]:PORT`")
	flag.Parse()

	/*
	
	This will check if SHA1 is found in whatever is checked out in DIR.
	If so, it calls back URL, signalling that something must be done.

	*/
}
