package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const (
	PORT                = ":28919"
	HEALTHCHECK_TIMEOUT = 1
)

var mux *http.ServeMux = http.NewServeMux()

// Just the URL of the testing machines for now
type TestRunner struct {
	URL   string `json: "url"`
	Alive bool
}

type Commit struct {
	Id         string `json: "id"`
	Repository string `json: "repository"`
	CTime      time.Time
}

// Temporary database, should be replaced with a real DB, like sqlite
// Just carry a mapping of repository -> latest commit processed and an array
// of TestRunner servers
type Store struct {
	Runners      []TestRunner
	Repositories map[string]*Commit
}

var repoStore = Store{[]TestRunner{}, map[string]*Commit{}}

func (s *Store) SendToTestRunner(c *Commit) {
	log.Println("Sending commit ", c)
}

// Simple middleware to log method and path
func reqLog(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.URL.Path, 200)
		next(w, r)
	}
}

func handleCommit(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		// Only POST is allowed, decode the json payload and check if the
		// received commit is elegible for a test-run of it's already been
		// processed before
		decoder := json.NewDecoder(r.Body)
		var c Commit
		err := decoder.Decode(&c)
		// TODO handle
		if err != nil {
			panic(err)
		}
		c.CTime = time.Now()
		if _, ok := repoStore.Repositories[c.Repository]; ok {
			if repoStore.Repositories[c.Repository].Id != c.Id {
				log.Println("New commit received, enqueuing work to test runner")
				repoStore.Repositories[c.Repository] = &c
				repoStore.SendToTestRunner(&c)
			}
		} else {
			repoStore.Repositories[c.Repository] = &c
			repoStore.SendToTestRunner(&c)
		}
	default:
		// 400 for unwanted HTTP methods
		w.WriteHeader(http.StatusBadRequest)
	}
}

func handleTestRunner(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Return a list of already registered testrunners
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repoStore.Runners)
	case http.MethodPost:
		// Register a new testrunner
		decoder := json.NewDecoder(r.Body)
		var t TestRunner
		err := decoder.Decode(&t)
		// TODO handle
		if err != nil {
			panic(err)
		}
		t.Alive = true
		repoStore.Runners = append(repoStore.Runners, t)
	default:
		// 400 for unwanted HTTP methods
		w.WriteHeader(http.StatusBadRequest)
	}
}

func main() {
	hc := make(chan bool)
	ticker := time.NewTicker(HEALTHCHECK_TIMEOUT * time.Second)
	// Start a goroutine to perform basic healthchecks on testrunners
	go func() {
		for {
			select {
			case <-hc:
				ticker.Stop()
				return
			case <-ticker.C:
				for _, t := range repoStore.Runners {
					res, err := http.Get(t.URL + "/status")
					if err != nil || res.StatusCode != 200 {
						t.Alive = false
					}
				}
			}
		}
	}()
	http.HandleFunc("/testrunner", reqLog(handleTestRunner))
	http.HandleFunc("/commit", reqLog(handleCommit))
	log.Println("Listening on", PORT)
	log.Fatal(http.ListenAndServe(PORT, nil))
	// Stop the healthcheck goroutine
	hc <- true
}
