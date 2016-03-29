package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func (s *server) jenkinsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	env := vars["env"]
	url := r.URL.Query()

	branches, ok := url["branches"]
	if !ok {
		branches = make([]string, 1)
		branches[0] = "master"
	}
	sha1 := url["sha1"]

	log.Printf("[jenkins] env %s: branch %s: notified commit %s", env, branches[0], sha1[0])
	s.notifs <- newNotif(env, sha1[0], branches[0])
	fmt.Fprintf(w, "Scheduled this %s job for ya!", env)
}

func (s *server) serveHTTP(listen string) {
	r := mux.NewRouter()
	r.HandleFunc("/{env}/jenkins/git/notifyCommit", s.jenkinsHandler)
	log.Fatal(http.ListenAndServe(listen, r))
}
