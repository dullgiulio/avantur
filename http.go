package umarell

import (
	"fmt"
	"log"
	"net/http"
	"sort"

	"github.com/gorilla/mux"
)

const reverseJenkinsURL = "{project}/jenkins/git/notifyCommit?{params}"

func (s *server) jenkinsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	project := vars["project"]
	url := r.URL.Query()

	branches, ok := url["branches"]
	if !ok {
		branches = make([]string, 1)
		branches[0] = "master"
	}
	sha1 := url["sha1"]

	log.Printf("[jenkins] project %s: branch %s: notified commit %s", project, branches[0], sha1[0])
	s.notifs <- newNotif(project, sha1[0], branches[0])
	fmt.Fprintf(w, "Scheduled this %s job for ya!", project)
}

type urlsWriter func(host string, urls []string, w http.ResponseWriter) error

func (s *server) listHandler(wf urlsWriter) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[http] %s: serving request to read jenkins URLs", r.RemoteAddr)
		urls := s.conf.urls.get()
		sort.Strings(urls)
		host := r.Host
		if host == "" {
			host = "localhost"
		}
		if err := wf(host, urls, w); err != nil {
			log.Printf("[http] cannot write URLs: %s", err)
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func textWriter(host string, urls []string, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, url := range urls {
		if _, err := fmt.Fprintf(w, "http://%s/%s\n", host, url); err != nil {
			return err
		}
	}
	return nil
}

func htmlWriter(host string, urls []string, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	for _, url := range urls {
		link := fmt.Sprintf("http://%s/%s\n", host, url)
		if _, err := fmt.Fprintf(w, "<a href=\"%s\">%s</a><br />\n", link, link); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) serveHTTP(listen string) {
	r := mux.NewRouter()
	r.HandleFunc("/_/text", s.listHandler(textWriter))
	r.HandleFunc("/_/html", s.listHandler(htmlWriter))
	r.HandleFunc("/{project}/jenkins/git/notifyCommit", s.jenkinsHandler)
	log.Fatal(http.ListenAndServe(listen, r))
}
